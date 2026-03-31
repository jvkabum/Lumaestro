package agents

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Lumaestro/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ACPExecutor gerencia a execução do Gemini CLI em modo --acp (JSON-RPC)
type ACPExecutor struct {
	Mu             sync.Mutex
	msgIDCounter   uint64
	ActiveSessions map[string]*ACPSession
	LogChan        chan ExecutionLog
	TerminalOutput chan TerminalData
	Proxy          *FSProxy // As "mãos" do backend

	// Modo Autônomo (--approval-mode=yolo) ou as flags de segurança finas
	AutonomousMode bool
	Ctx            context.Context

	pendingReviews map[string]chan bool
	reviewMu       sync.Mutex

	// pendingRequests mapeia o ID da mensagem para um canal que receberá o resultado.
	pendingRequests   map[int]chan JSONRPCMessage
	requestsMu        sync.Mutex
	
	Tools             *ToolRegistry // 🛠️ Biblioteca de ferramentas do Obsidian
}

// SessionInfo representa metadados de uma sessão ACP (Checkpoint)
type SessionInfo struct {
	SessionID        string `json:"sessionId"`
	Title            string `json:"title"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
	File             string `json:"file"`
	IsCurrentSession bool   `json:"isCurrentSession"`
}

// ListSessionsResponse é a resposta estruturada para o método listSessions
type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

// ACPSession representa a conexão JSON-RPC ativa com um Agent Server.
type ACPSession struct {
	ID        string // Lumaestro Session ID
	ACPSessID string // ACP Internal Session ID
	AgentName string
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser
	Cancel    context.CancelFunc
	// initDone sinaliza eventos de inicialização.
	// Usamos buffer de 1 para evitar bloqueios ao sinalizar.
	initDone chan struct{}
}

// NewACPExecutor inicializa o novo executor JSON-RPC.
func NewACPExecutor() *ACPExecutor {
	return &ACPExecutor{
		ActiveSessions: make(map[string]*ACPSession),
		LogChan:        make(chan ExecutionLog, 100),
		TerminalOutput: make(chan TerminalData, 256),
		Proxy:          NewFSProxy(),
		Tools:          NewToolRegistry(), // 🛠️ Inicializa as ferramentas Obsidian
		pendingReviews: make(map[string]chan bool),
		pendingRequests: make(map[int]chan JSONRPCMessage),
	}
}

// waitForResponse aguarda a resposta de um ID específico por um tempo determinado.
func (e *ACPExecutor) waitForResponse(id int, timeout time.Duration) (JSONRPCMessage, error) {
	ch := make(chan JSONRPCMessage, 1)

	e.requestsMu.Lock()
	e.pendingRequests[id] = ch
	e.requestsMu.Unlock()

	defer func() {
		e.requestsMu.Lock()
		delete(e.pendingRequests, id)
		e.requestsMu.Unlock()
	}()

	select {
	case msg := <-ch:
		if msg.Error != nil {
			return msg, fmt.Errorf("erro RPC [%d]: %s", msg.Error.Code, msg.Error.Message)
		}
		return msg, nil
	case <-time.After(timeout):
		return JSONRPCMessage{}, fmt.Errorf("timeout aguardando resposta para ID %d", id)
	}
}

func (e *ACPExecutor) getNextID() int {
	return int(atomic.AddUint64(&e.msgIDCounter, 1))
}

// RequestReview emite um evento para o Wails e aguarda a resposta do usuário.
func (e *ACPExecutor) RequestReview(reviewID string, action string, details string) bool {
	ch := make(chan bool)

	e.reviewMu.Lock()
	e.pendingReviews[reviewID] = ch
	e.reviewMu.Unlock()

	// Emite evento para o Frontend (Wails)
	runtime.EventsEmit(e.Ctx, "agent:review_request", map[string]string{
		"id":      reviewID,
		"action":  action,
		"details": details,
	})

	// Aguarda a resposta (bloqueia a goroutine do RPC Handler)
	approved := <-ch

	e.reviewMu.Lock()
	delete(e.pendingReviews, reviewID)
	e.reviewMu.Unlock()

	return approved
}

// SubmitReview é chamado pelo Frontend via Wails para aprovar/rejeitar.
func (e *ACPExecutor) SubmitReview(reviewID string, approved bool) {
	e.reviewMu.Lock()
	ch, ok := e.pendingReviews[reviewID]
	e.reviewMu.Unlock()

	if ok {
		ch <- approved
	}
}

// SendRPC envia uma mensagem JSON-RPC para o processo ACP via stdin usando o formato ndJSON.
func (e *ACPExecutor) SendRPC(s *ACPSession, msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// ACP oficial usa Newline-Delimited JSON (ndJSON).
	// Enviamos o JSON puro seguido de uma quebra de linha.
	_, err = s.Stdin.Write(append(data, '\n'))
	return err
}

// StartSession inicia o Gemini CLI com a flag --acp. Se loadSessionID for fornecido, tenta restaurar essa sessão em vez de criar uma nova.
func (e *ACPExecutor) StartSession(ctx context.Context, agent string, sessionID string, loadSessionID string) error {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	e.Ctx = ctx

	if s, ok := e.ActiveSessions[sessionID]; ok {
		if s.Cancel != nil {
			s.Cancel()
		}
		delete(e.ActiveSessions, sessionID)
	}

	cmdCtx, cancel := context.WithCancel(ctx)

	// Resolver binário de forma robusta
	binaryPath := agent
	args := []string{"--acp"}

	// 1. Tenta binário global (LookPath)
	if globalPath, errGL := exec.LookPath(binaryPath); errGL == nil {
		binaryPath = globalPath
	} else {
		// 2. Fallback para node_modules local (estilo dev)
		cwd, _ := os.Getwd()
		localBin := filepath.Join(cwd, "node_modules", ".bin", binaryPath+".cmd")

		// [TRUQUE DE SINFONIA] Se estivermos no Windows e for o Gemini, o .cmd é instável com espaços.
		// Tentamos o Bypass: rodar o script JS diretamente via 'node'.
		if agent == "gemini" {
			jsPath := filepath.Join(cwd, "node_modules", "@google", "gemini-cli", "dist", "index.js")
			if _, err := os.Stat(jsPath); err == nil {
				binaryPath = "node"
				args = []string{"--no-warnings=DEP0040", jsPath, "--acp"}
				fmt.Printf("[ACP] Bypass CMD ativado: Rodando via Node.js diretamento no script JS.\n")
			} else {
				binaryPath = localBin
			}
		} else {
			binaryPath = localBin
		}

		// Se o arquivo local existe, pegamos o caminho absoluto (crucial para Windows com espaços)
		if absPath, errAbs := filepath.Abs(binaryPath); errAbs == nil && binaryPath != "node" {
			binaryPath = absPath
		}
	}

	fmt.Printf("[ACP] Executando: %s %v\n", binaryPath, args)

	// Garantir aspas no Windows para caminhos com espaços
	cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
	cmd.Dir, _ = os.Getwd()
	cmd.Env = os.Environ()

	// 🚨 CRUCIAL: Injetar a chave da API do config.json no processo Node
	// E forçar o uso da pasta .gemini local do projeto (portabilidade)
	cwd, _ := os.Getwd()
	cmd.Env = append(cmd.Env, "GEMINI_CLI_HOME="+cwd)

	// Habilitar Telemetria para Diagnóstico (Igual ao script de sucesso)
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_ENABLED=true")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_TARGET=local")
	diagLog := filepath.Join(cwd, "gemini-telemetry.json")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_OUTFILE="+diagLog)

	if cfg, errCfg := config.Load(); errCfg == nil {
		if agent == "gemini" && cfg.GeminiAPIKey != "" {
			cmd.Env = append(cmd.Env, "GEMINI_API_KEY="+cfg.GeminiAPIKey)
		} else if agent == "claude" && cfg.ClaudeAPIKey != "" {
			cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+cfg.ClaudeAPIKey)
		}
	}

	// No Windows com espaços no caminho, o CommandContext às vezes precisa de ajuda.
	// Garantimos que o caminho seja tratado como uma string única.

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	// stderr piped para log e diagnóstico
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("falha ao iniciar %s no modo ACP: %v", agent, err)
	}

	session := &ACPSession{
		ID:        sessionID,
		AgentName: agent,
		Cmd:       cmd,
		Stdin:     stdin,
		Cancel:    cancel,
		initDone:  make(chan struct{}, 1),
	}

	e.ActiveSessions[sessionID] = session
	runtime.EventsEmit(e.Ctx, "agent:starting", agent)

	// Inicia os Listeners (Stdout para RPC, Stderr para Diagnóstico)
	go e.runRPCListener(session, stdout)
	go e.runStderrMonitor(session, stderr)

	// O Gemini CLI pode demorar até 2~3 segundos validando plugins e o ambiente
	// antes de registrar o canal ndJSON. Enviamos com delay proposital para garantir que ele já ouca o PIPE do STDIN.
	// 🚀 A Sincronização com o Script de Sucesso exige 3s+ no Windows.
	time.Sleep(3500 * time.Millisecond)

	// 1. Handshake Inicial: "initialize" (Estágio 1/3)
	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":1,"clientInfo":{"name":"Lumaestro","version":"2.0.0"},"clientCapabilities":{"fs":{"readTextFile":true,"writeTextFile":true}}}`),
	})

	// Aguarda initialize responder
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 1 (initialize) concluído.")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout no 'initialize' do Gemini")
	}

	// 2. Autenticação: "authenticate" (Estágio 2/3)
	// Como o usuário pode escolher entre a API (paga/rápida) ou o OAuth (gratuito) pelas configurações:
	methodId := "oauth-personal"
	if cfg, errCfg := config.Load(); errCfg == nil && cfg.UseGeminiAPIKey {
		methodId = "gemini-api-key"
	}

	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "authenticate",
		Params:  json.RawMessage(`{"methodId":"` + methodId + `"}`),
	})

	// Aguarda autenticação responder
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 2 (authenticate) concluído.")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout no 'authenticate' do Gemini")
	}

	// 3. Criar ou Carregar Sessão: "session/new" ou "session/load" (Estágio 3/3)
	if loadSessionID != "" {
		targetID := loadSessionID
		
		// 🚀 Lógica de Auto-Load da Última Sinfonia (Cursor Style)
		if loadSessionID == "LATEST" {
			fmt.Println("[ACP] Buscando a última sinfonia para restauração automática...")
			history, err := e.ListSessions(session)
			if err == nil && len(history) > 0 {
				targetID = history[0].SessionID 
				fmt.Printf("[ACP] Última sinfonia encontrada: %s\n", targetID)
			} else {
				fmt.Println("[ACP] Nenhuma sinfonia encontrada. Preparando palco novo.")
				targetID = "" 
			}
		}

		if targetID != "" {
			errLoad := e.LoadSession(session, targetID)
			if errLoad == nil {
				select {
				case session.initDone <- struct{}{}:
				default:
				}
				fmt.Printf("[ACP] Sessão anterior (%s) restaurada com sucesso!\n", targetID)
			} else {
				fmt.Printf("[ACP] Erro ao carregar sessão anterior (tentando nova): %v\n", errLoad)
				targetID = "" // Fallback para nova sessão
			}
		}

		if targetID == "" {
			e.SendRPC(session, JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      e.getNextID(),
				Method:  "session/new",
				Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `","mcpServers":[]}`),
			})
		}
	} else {
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      e.getNextID(),
			Method:  "session/new",
			Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `","mcpServers":[]}`),
		})
	}

	// Aguarda session/new ou session/load responder
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 3 concluído. Sessão pronta!")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout no estágio 3 do Gemini")
	}

	// 🔑 Estágio 4: Auto-Approve - Libera as mãos do Maestro
	// Envia setSessionMode para que o Gemini CLI execute ferramentas sem pedir permissão
	fmt.Println("[ACP] Enviando setSessionMode (auto-approve)...")
	modeParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"mode": map[string]interface{}{
			"toolConfirmation": "none",
		},
	})
	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "setSessionMode",
		Params:  modeParams,
	})
	fmt.Println("[ACP] Auto-approve configurado. Mãos livres!")

	// Handshake Concluído! Agora sim avisamos a UI que o agente está pronto.
	runtime.EventsEmit(e.Ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "ACP (JSON-RPC)",
		"isRealPTY": false,
	})

	// Monitora o fim do processo em background
	go func() {
		err := cmd.Wait()
		
		e.Mu.Lock()
		currentSession, isCurrentlyActive := e.ActiveSessions[sessionID]
		// Só deve reportar se o processo que morreu for EXATAMENTE a instância que iniciamos agora,
		// e não uma instância antiga que foi morta pelo nosso próprio 'cancel()' na abertura de uma nova.
		stillActive := isCurrentlyActive && currentSession.Cmd == cmd
		if stillActive {
			delete(e.ActiveSessions, sessionID)
		}
		e.Mu.Unlock()

		if err != nil && stillActive {
			// Não loga se for erro de contexto cancelado intencionalmente
			if cmdCtx.Err() == nil {
				e.LogChan <- ExecutionLog{
					Source:  "ERROR",
					Content: fmt.Sprintf("⚠️ Sinfonia interrompida abruptamente: %v", err),
				}
			}
		}

		if stillActive {
			fmt.Printf("[ACP] Sessão %s encerrada do mapa.\n", agent)
			runtime.EventsEmit(e.Ctx, "terminal:closed", agent)
			
			e.LogChan <- ExecutionLog{
				Source:  "SYSTEM",
				Content: "Sessão ACP " + agent + " encerrada.",
			}
		}
	}()

	return nil
}

func (e *ACPExecutor) runRPCListener(s *ACPSession, stdout io.Reader) {
	handler := &ACPRpcHandler{Executor: e, Session: s}
	StartJSONRPCListener(stdout, handler)
}

func (e *ACPExecutor) runStderrMonitor(s *ACPSession, stderr io.Reader) {
	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cleanLine := strings.TrimSpace(line)
		if cleanLine == "" {
			continue
		}

		// Filtragem inteligente: Reportar apenas o que importa para o usuário no Chat
		// Mensagem com "error", "login", "warning" ou "denied" sobem para a UI
		lv := strings.ToLower(cleanLine)
		isRelevant := strings.Contains(lv, "login") ||
			strings.Contains(lv, "auth") ||
			strings.Contains(lv, "error") ||
			strings.Contains(lv, "warning") ||
			strings.Contains(lv, "denied") ||
			strings.Contains(lv, "not found")

		if isRelevant {
			e.LogChan <- ExecutionLog{
				Source:  "CLI/AVISO",
				Content: cleanLine,
			}
		}

		// Gatilhos específicos
		if strings.Contains(cleanLine, "Login required") {
			runtime.EventsEmit(e.Ctx, "agent:login_required", s.AgentName)
		}

		// Log interno (Terminal do Wails)
		fmt.Printf("[%s/stderr] %s\n", s.AgentName, cleanLine)
	}
}

// Implementação do JSONRPCHandler para plugar o backend do Lumaestro nas chamadas ACP
type ACPRpcHandler struct {
	Executor *ACPExecutor
	Session  *ACPSession
}

func (h *ACPRpcHandler) HandleNotification(method string, params json.RawMessage) {
	// 1. Notificações de Progresso (Trabalho em background)
	if method == "agent/progress" {
		var p struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil {
			h.Executor.LogChan <- ExecutionLog{
				Source:  h.Session.AgentName,
				Content: fmt.Sprintf("⏳ %s...", p.Message),
			}
		}
	}

	// 2. Notificações de Streaming de Sessão (O texto real da resposta)
	if method == "session/update" {
		fmt.Printf("[ACP RAW] session/update Recebido: %s\n", string(params))

		var p struct {
			Update struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"update"`
		}
		if json.Unmarshal(params, &p) == nil {
			update := p.Update
			// Captura blocos de mensagem ou pensamentos
			if update.SessionUpdate == "agent_message_chunk" || update.SessionUpdate == "agent_thought_chunk" {
				if update.Content.Text != "" {
					logType := "message"
					if update.SessionUpdate == "agent_thought_chunk" {
						logType = "thought"
					}
					
					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: update.Content.Text,
						Type:    logType,
					}
				}
			} else if update.SessionUpdate == "agent_message_error" || update.SessionUpdate == "error" {
				// Relatar qualquer erro ou notificação inesperada
				h.Executor.LogChan <- ExecutionLog{
					Source:  "ERROR",
					Content: "⚠️ Aviso do Gemini: O formato da sua mensagem (prompt) pode ter sido rejeitado internamente.",
				}
			}
		}
	}
}

// HandleRequest lida com os pedidos de ferramenta (hands) da IA.
func (h *ACPRpcHandler) HandleRequest(id interface{}, method string, params json.RawMessage) {
	fmt.Printf("[ACP DEBUG] Método Recebido: %s\n", method)
	// Normalização do método para compatibilidade entre dialetos (client/, fs/)
	normMethod := strings.ToLower(method)
	normMethod = strings.TrimPrefix(normMethod, "client/")
	normMethod = strings.TrimPrefix(normMethod, "fs/")

	var result interface{}
	var rpcErr *RPCError
	reviewID := fmt.Sprintf("rev-%v", id)

	switch normMethod {
	case "readfile", "read_file", "read_text_file":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowRead {
				content, err := h.Executor.Proxy.ReadFile(p.Path)
				if err == nil {
					result = map[string]string{"content": content}
				} else {
					rpcErr = &RPCError{Code: -32000, Message: err.Error()}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ LEITURA BLOQUEADA: Ative 'Permitir Leitura' nas configurações."}
			}
		}

	case "writefile", "write_file", "write_text_file", "write_file_content":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			
			// Verifica se o arquivo já existe para decidir se é uma 'Escrita' ou 'Criação'
			fileExists := false
			if _, err := os.Stat(p.Path); err == nil {
				fileExists = true
			}

			canAct := false
			if fileExists {
				canAct = cfg.Security.AllowWrite
			} else {
				canAct = cfg.Security.AllowCreate
			}

			if canAct {
				// Se for um arquivo crítico ou se Acesso Global estiver desligado, pedir review
				needsReview := !cfg.Security.FullMachineAccess || strings.HasSuffix(p.Path, ".go") || strings.HasSuffix(p.Path, ".json")
				
				if needsReview {
					actionLabel := "ESCREVER ARQUIVO"
					if !fileExists { actionLabel = "CRIAR ARQUIVO" }

					if h.Executor.RequestReview(reviewID, actionLabel, p.Path) {
						err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
						if err == nil {
							result = map[string]bool{"success": true}
						} else {
							rpcErr = &RPCError{Code: -32001, Message: err.Error()}
						}
					} else {
						rpcErr = &RPCError{Code: 403, Message: "Ação recusada pelo usuário."}
					}
				} else {
					// Ação direta para arquivos não-críticos (.txt, .md, etc)
					err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32001, Message: err.Error()}
					}
				}
			} else {
				msg := "🛡️ ESCRITA BLOQUEADA: Ative 'Permitir Escrita'"
				if !fileExists { msg = "🛡️ CRIAÇÃO BLOQUEADA: Ative 'Permitir Criação' nas configurações." }
				rpcErr = &RPCError{Code: 403, Message: msg}
			}
		}

	case "deletefile", "delete_file", "remove":

		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowDelete {
				if h.Executor.RequestReview(reviewID, "DELETAR ARQUIVO", p.Path) {
					err := h.Executor.Proxy.DeleteFile(p.Path)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32002, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Ação rejeitada pelo usuário"}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ DELEÇÃO BLOQUEADA: Permissão insuficiente."}
			}
		}

	case "movefile", "move_file":
		var p struct {
			OldPath string `json:"oldPath"`
			NewPath string `json:"newPath"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowMove {
				details := fmt.Sprintf("%s -> %s", p.OldPath, p.NewPath)
				if h.Executor.RequestReview(reviewID, "MOVER/RENOMEAR", details) {
					err := h.Executor.Proxy.MoveFile(p.OldPath, p.NewPath)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32003, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Ação rejeitada pelo usuário"}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ MOVIMENTAÇÃO BLOQUEADA."}
			}
		}

	case "runcommand", "run_command", "run_shell_command", "execute_command":
		var p struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowRunCommands {
				details := fmt.Sprintf("%s %s", p.Command, strings.Join(p.Args, " "))
				// Comandos sempre pedem review por segurança extrema
				if h.Executor.RequestReview(reviewID, "EXECUTAR COMANDO", details) {
					output, err := h.Executor.Proxy.RunCommand(p.Command, p.Args)
					if err == nil {
						// 🛠️ FORMATO ACP OFICIAL: Lista de Conteúdo
						result = map[string]interface{}{
							"content": []map[string]interface{}{
								{"type": "text", "text": output},
							},
						}
					} else {
						rpcErr = &RPCError{Code: -32004, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Execução rejeitada pelo usuário"}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ EXECUÇÃO BLOQUEADA: Ative 'Executar Comandos' nas configurações."}
			}
		}

	default:
		// 🔑 Protocolo ACP: Pedido de Permissão para usar ferramentas
		// O Gemini envia session/request_permission ANTES de executar qualquer ferramenta.
		// Precisamos responder com "permitted: true" para ele prosseguir.
		if method == "session/request_permission" {
			fmt.Printf("[ACP PERMISSION] Permissão solicitada. Aprovando automaticamente.\n")
			result = map[string]interface{}{
				"permitted": true,
			}
		} else if strings.HasPrefix(method, "Lumaestro/") {
			toolName := strings.TrimPrefix(method, "Lumaestro/")
			if tool, exists := h.Executor.Tools.Tools[toolName]; exists {
				var args map[string]interface{}
				// Unmarshal params (se houver)
				if len(params) > 0 && string(params) != "null" {
					json.Unmarshal(params, &args)
				}
				
				// Injeta o caminho do Vault automaticamente para ferramentas do Obsidian
				if args == nil { args = make(map[string]interface{}) }
				if _, ok := args["path"]; !ok {
					cfg, _ := config.Load()
					args["path"] = cfg.ObsidianVaultPath
				}

				output, err := tool.Function(args)
				if err == nil {
					result = map[string]interface{}{"result": output}
				} else {
					rpcErr = &RPCError{Code: -32005, Message: err.Error()}
				}
			} else {
				rpcErr = &RPCError{Code: -32601, Message: fmt.Sprintf("Ferramenta '%s' não registrada", toolName)}
			}
		} else {
			rpcErr = &RPCError{
				Code:    -32601,
				Message: "🛡️ AÇÃO BLOQUEADA: Método não reconhecido ou gesso de segurança ativo.",
			}
		}
	}


	err := h.Executor.SendRPC(h.Session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  h.wrapResult(result),
		Error:   rpcErr,
	})
	if err != nil {
		fmt.Printf("!! Erro ao responder ferramenta (HandleRequest): %v\n", err)
	}

	// Se for uma resposta a um prompt (ferramenta), avisamos o frontend para atualizar o histórico
	runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
}

// Helper para encapsular resultados no formato que o Unmarshal aceita
func (h *ACPRpcHandler) wrapResult(res interface{}) json.RawMessage {
	if res == nil {
		return nil
	}
	b, _ := json.Marshal(res)
	return b
}

func (h *ACPRpcHandler) HandleResponse(id interface{}, result json.RawMessage, rpcErr *RPCError) {
	// JSON numbers são float64 em Go quando vindos de decodificação genérica
	idFloat, ok := id.(float64)
	if !ok {
		return
	}
	idInt := int(idFloat)

	h.Executor.requestsMu.Lock()
	ch, found := h.Executor.pendingRequests[idInt]
	h.Executor.requestsMu.Unlock()

	if found {
		ch <- JSONRPCMessage{
			ID:     id,
			Result: result,
			Error:  rpcErr,
		}
		return
	}

	if rpcErr != nil {
		fmt.Printf("<< Erro RPC Respondido [ID %v]: %s\n", id, rpcErr.Message)
		// Reporta o erro no chat para o usuário não ficar no vácuo
		h.Executor.LogChan <- ExecutionLog{
			Source:  "ERROR",
			Content: fmt.Sprintf("❌ Erro na Sinfonia ACP: %s", rpcErr.Message),
		}
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(result, &response); err == nil {
		// 1. Captura de SessionID (Handshake Estágio 2)
		if sessID, ok := response["sessionId"].(string); ok {
			h.Session.ACPSessID = sessID
			fmt.Printf("<< SessionID Capturado: %s\n", sessID)
			select {
			case h.Session.initDone <- struct{}{}:
			default:
			}
			return
		}

		select {
		case h.Session.initDone <- struct{}{}:
		default:
		}
		// Sincroniza lista após carregar uma sessão ou finalizar um turno
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
}

// SendInput envia texto para uma sessão ativa da IA via RPC 'prompt'.
func (e *ACPExecutor) SendInput(sessionID string, input string, images []map[string]string) error {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok {
		return fmt.Errorf("sessão '%s' não encontrada", sessionID)
	}

	if session.ACPSessID == "" {
		return fmt.Errorf("sessão não initializada completamente (sem ACP sessionId)")
	}

	// 🧠 Construção do Prompt Multimodal (Texto + Imagens)
	var promptData []interface{}
	
	// Adiciona a parte de texto
	promptData = append(promptData, map[string]string{
		"type": "text",
		"text": input,
	})

	// Adiciona as partes de imagem (se houver)
	for _, img := range images {
		promptData = append(promptData, map[string]interface{}{
			"type": "image",
			"source": map[string]string{
				"type":      "base64",
				"mediaType": img["type"],
				"data":      img["data"],
			},
		})
	}

	params, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"prompt":    promptData,
	})

	return e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "session/prompt",
		Params:  params,
	})
}

// StopSession encerra uma sessão ativa.
func (e *ACPExecutor) StopSession(sessionID string) error {
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if ok {
		if session.Cancel != nil {
			session.Cancel()
		}
		e.Mu.Lock()
		delete(e.ActiveSessions, sessionID)
		e.Mu.Unlock()
		return nil
	}
	return fmt.Errorf("sessão '%s' não encontrada", sessionID)
}

// ListSessions pede ao Agente a lista de sessões disponíveis.
func (e *ACPExecutor) ListSessions(s *ACPSession) ([]SessionInfo, error) {
	id := e.getNextID()
	err := e.SendRPC(s, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  "session/list",
		Params:  json.RawMessage("{}"),
	})
	if err != nil {
		return nil, err
	}

	resp, err := e.waitForResponse(id, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var list struct {
		Sessions []struct {
			ID          string `json:"id"`
			StartTime   string `json:"startTime"`
			LastUpdated string `json:"lastUpdated"`
			DisplayName string `json:"displayName"`
		} `json:"sessions"`
	}

	if err := json.Unmarshal(resp.Result, &list); err != nil {
		return nil, fmt.Errorf("falha ao decodificar lista de sessões: %v", err)
	}

	var finalList []SessionInfo
	for _, s := range list.Sessions {
		finalList = append(finalList, SessionInfo{
			SessionID: s.ID,
			Title:     s.DisplayName,
			CreatedAt: s.StartTime,
			UpdatedAt: s.LastUpdated,
		})
	}

	return finalList, nil
}

// LoadSession restaura uma sessão específica (Checkpoint).
func (e *ACPExecutor) LoadSession(s *ACPSession, acpSessionID string) error {
	s.ACPSessID = acpSessionID // 🚀 Salva o ID restaurado na Struct da conexão local!

	id := e.getNextID()
	cwd, _ := os.Getwd()
	params := map[string]interface{}{
		"sessionId":  acpSessionID,
		"cwd":        cwd,
		"mcpServers": []interface{}{},
	}
	paramsJSON, _ := json.Marshal(params)

	err := e.SendRPC(s, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  "session/load",
		Params:  paramsJSON,
	})
	if err != nil {
		return err
	}

	_, err = e.waitForResponse(id, 15*time.Second)
	return err
}
