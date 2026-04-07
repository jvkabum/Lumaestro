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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Lumaestro/internal/config"
	"Lumaestro/internal/db"
	"Lumaestro/internal/lightning"
	"Lumaestro/internal/orchestration"
	"Lumaestro/internal/utils"

	"github.com/google/uuid"
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
	
	// Fila de execução para ferramentas (Semáforo)
	execLock chan struct{}

	// 📡 Agregador de logs de rede
	NetLog *utils.NetworkLogger
	
	// Turnos Ativos para AskSync
	turnChannels map[string]chan string
	turnMu       sync.Mutex

	// ✨ Motores de Elite (Lightning)
	LStore         *lightning.DuckDBStore
	RewardEngine   *lightning.RewardEngine
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
	
	// Governança e Orquestração Swarm
	AgentID        uuid.UUID
	CurrentIssueID *uuid.UUID

	// Trava de escrita para garantir integridade do JSON no stdin
	WriteMu sync.Mutex

	// Estados de log para evitar flooding no terminal
	isLoggingThought bool
	isLoggingMessage bool

	// 🧬 Telemetria Lightning (Rastreamento de Elite)
	RolloutID string
	AttemptID string
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
		execLock:        make(chan struct{}, 1), // Apenas 1 ferramenta por vez
		NetLog:          utils.NewNetworkLogger(5 * time.Second),
		turnChannels:    make(map[string]chan string),
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

	// Garante que apenas um pacote seja escrito por vez no Pipe
	s.WriteMu.Lock()
	defer s.WriteMu.Unlock()

	fmt.Printf(">> [ACP SEND] %s\n", string(data))
	_, err = fmt.Fprintln(s.Stdin, string(data))
	return err
}

// StartSession inicia o Gemini CLI com a flag --acp. Se loadSessionID for fornecido, tenta restaurar essa sessão em vez de criar uma nova.
func (e *ACPExecutor) StartSession(ctx context.Context, agent string, sessionID string, loadSessionID string, agentID uuid.UUID, issueID *uuid.UUID) error {
	e.Mu.Lock()
	defer e.Mu.Unlock()
	e.Ctx = ctx
	e.Tools.Ctx = ctx

	if s, ok := e.ActiveSessions[sessionID]; ok {
		if s.Cancel != nil {
			s.Cancel()
		}
		delete(e.ActiveSessions, sessionID)
	}

	cmdCtx, cancel := context.WithCancel(ctx)

	// Resolver binário de forma robusta
	binaryPath := agent
	args := []string{"--acp", "--approval-mode=yolo"}

	// 1. Tenta binário global (LookPath)
	if globalPath, errGL := exec.LookPath(binaryPath); errGL == nil {
		binaryPath = globalPath
	} else {
		// 2. Fallback para node_modules local (estilo dev)
		cwd, _ := os.Getwd()
		binaryPath = filepath.Join(cwd, "node_modules", ".bin", binaryPath+".cmd")
	}

	// [TRUQUE DE SINFONIA] Se estivermos no Windows e for o Gemini, o .cmd (tanto local quanto global)
	// costuma engolir o Stdin em Pipes IPC, quebrando o JSON-RPC. Precisamos bypassar rodando via 'node'.
	if agent == "gemini" && strings.HasSuffix(binaryPath, ".cmd") {
		// A pasta .cmd do NPM fica em {NPM_ROOT}/node_modules/.bin
		// E o JS real fica em {NPM_ROOT}/node_modules/@google/gemini-cli/bundle/gemini.js
		baseDir := filepath.Dir(binaryPath)

		// Verifica se é local (../node_modules) ou global (node_modules direto)
		// Em local, baseDir = "cwd/node_modules/.bin". Logo o pacote está em "../@google/..."
		// Em global, baseDir = "AppData/.../npm". Logo pacote está em "node_modules/@google/..."
		jsPathGlobalDist := filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "dist", "index.js")
		jsPathLocalDist := filepath.Join(baseDir, "..", "@google", "gemini-cli", "dist", "index.js")
		jsPathGlobalBundle := filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "bundle", "gemini.js")
		jsPathLocalBundle := filepath.Join(baseDir, "..", "@google", "gemini-cli", "bundle", "gemini.js")

		jsTarget := ""
		// v0.36+: O pacote NPM shipa apenas bundle/gemini.js (dist/ foi removido)
		if _, err := os.Stat(jsPathLocalBundle); err == nil {
			jsTarget = jsPathLocalBundle
		} else if _, err := os.Stat(jsPathGlobalBundle); err == nil {
			jsTarget = jsPathGlobalBundle
		} else if _, err := os.Stat(jsPathLocalDist); err == nil {
			jsTarget = jsPathLocalDist
		} else if _, err := os.Stat(jsPathGlobalDist); err == nil {
			jsTarget = jsPathGlobalDist
		}

		if jsTarget != "" {
			binaryPath = "node"
			args = []string{"--no-warnings=DEP0040", jsTarget, "--acp", "--approval-mode=yolo"}
			fmt.Printf("[ACP] Bypass CMD ativado: Rodando diretamente Node em %s\n", jsTarget)
		}
	}

	// Se o arquivo não passou pelo Bypass (não é 'node'), resolvemos caminho absoluto
	if absPath, errAbs := filepath.Abs(binaryPath); errAbs == nil && binaryPath != "node" {
		binaryPath = absPath
	}

	fmt.Printf("[ACP] Executando: %s %v\n", binaryPath, args)

	// Garantir aspas no Windows para caminhos com espaços
	cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
	cmd.Dir, _ = os.Getwd()
	cmd.Env = os.Environ()

	// 🚨 CRUCIAL: Injetar a pasta de sessão específica desta conta (Prioridade Local)
	cwd, _ := os.Getwd()
	sessionHome := cwd // Default: Diretório do Projeto

	// Se houver uma pasta .gemini local, assumimos ela como Home sempre.
	if _, err := os.Stat(filepath.Join(cwd, ".gemini")); err == nil {
		fmt.Println("[ACP] 📂 Pasta .gemini local detectada! Forçando modo Project-Specific.")
		sessionHome = cwd
	} else if cfg, errCfg := config.Load(); errCfg == nil {
		// Fallback para config global apenas se não houver .gemini local
		for _, acc := range cfg.GeminiAccounts {
			if acc.Active && acc.HomeDir != "" {
				sessionHome = acc.HomeDir
				break
			}
		}
	}
	cmd.Env = append(cmd.Env, "GEMINI_CLI_HOME="+sessionHome)

	// Habilitar Telemetria para Diagnóstico
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_ENABLED=true")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_TARGET=local")
	diagLog := filepath.Join(sessionHome, "gemini-telemetry.json")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_OUTFILE="+diagLog)

	if cfg, errCfg := config.Load(); errCfg == nil {
		if agent == "claude" && cfg.ClaudeAPIKey != "" {
			cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+cfg.ClaudeAPIKey)
		}
	}

	// 🔐 SISTEMA DE SEGREDOS (Enterprise Vault)
	// Injeta API Keys específicas deste agente registradas no banco.
	if agentID != uuid.Nil {
		var secrets []db.AgentSecret
		if err := db.InstanceDB.Where("agent_id = ?", agentID).Find(&secrets).Error; err == nil {
			for _, s := range secrets {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", s.Key, s.Value))
			}
		}
	}

	// ⚡ INJEÇÃO LIGHTNING (Telemetria)
	rolloutID := "roll-" + uuid.NewString()
	attemptID := "att-1"
	cmd.Env = append(cmd.Env, "LIGHTNING_ROLLOUT_ID="+rolloutID)
	cmd.Env = append(cmd.Env, "LIGHTNING_ATTEMPT_ID="+attemptID)
	// ACP Agent se comunica narivamente pelo CLI usando OAuth.
	// O tráfego do agente não deve passar pelo ResilienceFleet (que usa API Keys).

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
		ID:             sessionID,
		AgentName:      agent,
		Cmd:            cmd,
		Stdin:          stdin,
		Cancel:         cancel,
		initDone:       make(chan struct{}, 1),
		AgentID:        agentID,
		CurrentIssueID: issueID,
		RolloutID:      rolloutID,
		AttemptID:      attemptID,
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

	// Aguarda initialize responder (Aumentado para 60s devido a 503/429)
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 1 (initialize) concluído.")
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout no 'initialize' do Gemini (API instável)")
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

	// Aguarda autenticação responder (Aumentado para 60s devido a 503/429)
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 2 (authenticate) concluído.")
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout no 'authenticate' do Gemini (Autenticação lenta)")
	}

	// 3. Criar ou Carregar Sessão: "newSession" ou "loadSession" (Estágio 3/3)
	if loadSessionID != "" {
		targetID := loadSessionID
		
		// A partir de v0.36, a CLI do Gemini removeu o método listSessions/session/list. 
		// O Lumaestro agora sempre iniciará uma nova sessão de forma silenciosa por padrão até implementarmos 
		// a restauração de ID puro pelo banco do projeto.
		if loadSessionID == "LATEST" {
			fmt.Println("[ACP] Buscando a última sinfonia para restauração automática...")
			fmt.Println("[ACP] Restore nativo no ACP deprecado na v0.36. Assumindo modo clean start.")
			targetID = ""
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
		if targetID != "" {
			e.SendRPC(session, JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      e.getNextID(),
				Method:  "session/load",
				Params:  json.RawMessage(`{"sessionId":"` + targetID + `","cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `"}`),
			})
		}
	}

	// Aguarda session/new ou session/load responder (Aumentado para 60s devido a 503/429)
	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 3 concluído. Sessão pronta!")
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout no estágio 3 do Gemini (Criação de sessão lenta)")
	}

	// 🔑 Estágio 4: Auto-Approve - Libera as mãos do Maestro
	// Envia setSessionMode para que o Gemini CLI execute ferramentas sem pedir permissão
	fmt.Println("[ACP] Enviando setSessionMode (auto-approve)...")
	modeParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"modeId":    "yolo",
	})
	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "session/set_mode",
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
	fmt.Printf("<< [ACP RECV Notify] %s: %s\n", method, string(params))
	// 1. Notificações de Progresso (Trabalho em background)
	if method == "agent/progress" || method == "agentProgress" {
		var p struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil {
			if !strings.Contains(h.Session.ID, "-background-") {
				h.Executor.LogChan <- ExecutionLog{
					Source:  h.Session.AgentName,
					Content: fmt.Sprintf("⏳ %s...", p.Message),
				}
			}
		}
	}

	// 2. Notificações de Streaming de Sessão (O texto real da resposta)
	if method == "session/update" || method == "sessionUpdate" {
		h.Executor.NetLog.LogRequest() // 📡 Registra atividade de rede silenciosamente

		fmt.Println("[ACP TRACE] RAW Update:", string(params))

		var p struct {
			SessionId string `json:"sessionId"`
			Update    struct {
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				Text string `json:"text"` // Suporte para formato plano v0.36
			} `json:"update"`
		}
		if json.Unmarshal(params, &p) == nil {
			update := p.Update
			isBg := strings.Contains(h.Session.ID, "-background-")
			
			if update.SessionUpdate == "agent_message_chunk" || update.SessionUpdate == "message_chunk" || update.SessionUpdate == "content_chunk" {
				txt := update.Content.Text
				if txt == "" {
					txt = update.Text // Fallback v0.36
				}
				
				if txt != "" && !isBg {
					if !h.Session.isLoggingMessage {
						utils.LogInfo("O Maestro está orquestrando a resposta...", "💬")
						h.Session.isLoggingMessage = true
						h.Session.isLoggingThought = false 
					}

					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: txt,
						Type:    "message",
					}
				}
			} else if update.SessionUpdate == "agent_thought_chunk" || update.SessionUpdate == "thought_chunk" {
				txt := update.Content.Text
				if txt == "" {
					txt = update.Text 
				}
				
				if txt != "" && !isBg {
					if !h.Session.isLoggingThought {
						utils.LogInfo(fmt.Sprintf("Processando raciocínio: %s...", strings.ToUpper(h.Session.AgentName)), "🧠")
						h.Session.isLoggingThought = true
						h.Session.isLoggingMessage = false 
					}
					
					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: txt,
						Type:    "thought",
					}
				}
			} else if update.SessionUpdate == "agent_turn_complete" {
				// Reset total ao fim do turno
				h.Session.isLoggingThought = false
				h.Session.isLoggingMessage = false

				// 🪙 LOGICA DE CUSTO (Paperclip Auto-Report)
				// Em V1, simulamos um gasto fixo por turno enquanto o CLI não expõe usage.usage
				// Isso aciona o Hard-Stop Budget se o agente gastar demais.
				if h.Session.AgentID != uuid.Nil {
					_ = orchestration.RegistrarCusto(h.Session.AgentID, h.Session.CurrentIssueID, "google", "gemini-1.5-flash", 800, 400, 2) // ~2 centavos por turno
				}

				// Sinaliza para o AskSync informando que o texto acabou (EOF virtual)
				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					close(ch)
					delete(h.Executor.turnChannels, h.Session.ID)
				}
				h.Executor.turnMu.Unlock()

			} else if update.SessionUpdate == "agent_message_error" || update.SessionUpdate == "error" {
				// Relatar qualquer erro ou notificação inesperada
				if !strings.Contains(h.Session.ID, "-background-") {
					h.Executor.LogChan <- ExecutionLog{
						Source:  "ERROR",
						Content: "⚠️ Aviso do Gemini: O formato da sua mensagem (prompt) pode ter sido rejeitado internamente.",
					}
				}

				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					close(ch)
					delete(h.Executor.turnChannels, h.Session.ID)
				}
				h.Executor.turnMu.Unlock()
			}

			// Se houver um AskSync aguardando, envia o chunk para o canal correspondente (Dentro do escopo de p.Update)
			if update.SessionUpdate == "agent_message_chunk" && update.Content.Text != "" {
				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					ch <- update.Content.Text
				}
				h.Executor.turnMu.Unlock()
			}
		}
	}
}

// HandleRequest lida com os pedidos de ferramenta (hands) da IA.
func (h *ACPRpcHandler) HandleRequest(id interface{}, method string, params json.RawMessage) {
	h.Executor.execLock <- struct{}{}
	defer func() { <-h.Executor.execLock }()

	utils.LogSection(fmt.Sprintf("FERRAMENTA: %s", method))
	utils.LogInfo(fmt.Sprintf("IA solicitou acesso a: %s", method), "🔧")
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

						// 🧬 AUTOREWARD (Aprendizado Técnico Autônomo)
						go func() {
							lowerCmd := strings.ToLower(details)
							isTest := strings.Contains(lowerCmd, "test") || strings.Contains(lowerCmd, "build") || strings.Contains(lowerCmd, "compile")
							if isTest && h.Executor.LStore != nil && h.Executor.RewardEngine != nil {
								// Sucesso técnico (zero exit code implicitamente se err == nil)
								h.Executor.RewardEngine.EmitReward(h.Session.RolloutID, h.Session.AttemptID, 0.5, "technical_success_auto", map[string]interface{}{
									"cmd": details,
								})
							}
						}()
					} else {
						rpcErr = &RPCError{Code: -32004, Message: err.Error()}
						// 🧬 AUTOREWARD (Penalidade por falha técnica)
						go func() {
							if h.Executor.LStore != nil && h.Executor.RewardEngine != nil {
								h.Executor.RewardEngine.EmitReward(h.Session.RolloutID, h.Session.AttemptID, -0.5, "technical_failure_auto", map[string]interface{}{
									"cmd": details,
									"err": err.Error(),
								})
							}
						}()
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
			} else if toolName == "delegate_task" {
				// 🤖 FERRAMENTA NATIVA: HANDOFF (DELEGAÇÃO)
				var p struct {
					ToAgentID   string `json:"to_agent_id"`
					Title       string `json:"title"`
					Description string `json:"description"`
				}
				if json.Unmarshal(params, &p) == nil {
					targetID, _ := uuid.Parse(p.ToAgentID)
					_, err := orchestration.DelegateTask(h.Session.AgentID, targetID, h.Session.CurrentIssueID, p.Title, p.Description)
					if err == nil {
						result = map[string]string{"success": "Trabalho delegado com sucesso!"}
					} else {
						rpcErr = &RPCError{Code: -32006, Message: err.Error()}
					}
				}
			} else if toolName == "complete_task" {
				// 🏁 FERRAMENTA NATIVA: CONCLUSÃO
				if h.Session.CurrentIssueID != nil {
					err := orchestration.CompleteTask(h.Session.AgentID, *h.Session.CurrentIssueID)
					if err == nil {
						result = map[string]string{"success": "Tarefa marcada como concluída e arquivada."}
					} else {
						rpcErr = &RPCError{Code: -32007, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 404, Message: "Nenhum ticket ativo vinculado a esta sessão para encerrar."}
				}
			} else if toolName == "request_approval" {
				// ✋ FERRAMENTA NATIVA: PEDIDO DE APROVAÇÃO (PROATIVO)
				var p struct {
					Topic   string `json:"topic"`
					Details string `json:"details"`
				}
				if json.Unmarshal(params, &p) == nil {
					// Cria solicitação de aprovação no banco
					approval := db.Approval{
						Type:               "agent_request",
						RequestedByAgentID: &h.Session.AgentID,
						Payload:            fmt.Sprintf("TÓPICO: %s\n\nDETALHES: %s", p.Topic, p.Details),
					}
					if err := db.InstanceDB.Create(&approval).Error; err == nil {
						// PAUSA O AGENTE IMEDIATAMENTE (Portão Ativo)
						db.InstanceDB.Model(&db.Agent{}).Where("id = ?", h.Session.AgentID).Update("status", "paused")
						result = map[string]string{
							"success": "Solicitação enviada. A execução permanecerá pausada até que o humano aprove.",
							"approval_id": approval.ID.String(),
						}
					} else {
						rpcErr = &RPCError{Code: -32008, Message: err.Error()}
					}
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
	if !strings.Contains(h.Session.ID, "-background-") {
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
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
	fmt.Printf("<< [ACP RECV Resp] ID %v: %s\n", id, string(result))
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
		
		// 🚨 CORREÇÃO CRÍTICA: Se o stream terminar vazio, precisamos avisar a UI para parar de carregar
		if strings.Contains(rpcErr.Message, "Model stream ended with empty response") {
			h.Executor.LogChan <- ExecutionLog{
				Source:  "SYSTEM",
				Content: "O Gemini decidiu não responder agora (Stream Vazio). Tente perguntar de outra forma.",
			}
			runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
		} else {
			// Reporta o erro no chat para o usuário não ficar no vácuo
			h.Executor.LogChan <- ExecutionLog{
				Source:  "ERROR",
				Content: fmt.Sprintf("❌ Erro na Sinfonia ACP: %s", rpcErr.Message),
			}
		}

		// 🚨 EVITAR TIMEOUT (HANG) DO ALGORÍTMO: Se ocorreu um erro no turno, fechar canal
		h.Executor.turnMu.Lock()
		if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
			close(ch)
			delete(h.Executor.turnChannels, h.Session.ID)
		}
		h.Executor.turnMu.Unlock()

		return
	}

	var response map[string]interface{}
	errJson := json.Unmarshal(result, &response)
	
	if errJson == nil && response != nil {
		// 1. Captura de SessionID (Somente Handshake Estágio 3 ou Load)
		if sessID, ok := response["sessionId"].(string); ok {
			h.Session.ACPSessID = sessID
			fmt.Printf("<< SessionID Capturado: %s\n", sessID)
		}
	}
	
	// 🚀 SINCRONIZAÇÃO DE HANDSHAKE: IDs 1 (init), 2 (auth) e 3 (new/load) 
	// Não podemos travar o boot se não houver sessionId neles.
	if idInt <= 3 {
		fmt.Printf("[ACP] Handshake Stage %d concluído com sucesso.\n", idInt)
		select {
		case h.Session.initDone <- struct{}{}:
		default:
		}
		return
	}
	
	// PRINT DE EMERGÊNCIA E ARQUIVO
	debugMsg := fmt.Sprintf("ID: %v\nResult: %s\nJsonErr: %v\n==================\n", id, string(result), errJson)
	fmt.Print("[ACP Debug] ", debugMsg)
	if f, errFile := os.OpenFile("acp_debug.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); errFile == nil {
		f.WriteString(debugMsg)
		f.Close()
	}

	select {
	case h.Session.initDone <- struct{}{}:
	default:
	}
	// Sincroniza lista após carregar uma sessão ou finalizar um turno
	if !strings.Contains(h.Session.ID, "-background-") {
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
}

// SendInput envia texto para uma sessão ativa da IA via RPC 'prompt'.
func (e *ACPExecutor) SendInput(sessionID string, input string, images []map[string]string) error {
	fmt.Printf("[ACP] >> SendInput recebido! Session: %s, Msg: %s...\n", sessionID, input)
	
	e.Mu.Lock()
	session, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok || session == nil {
		fmt.Printf("[ACP] ❌ Erro: Sessão %s não encontrada no mapa de sessões ativas!\n", sessionID)
		return fmt.Errorf("sessão %s não encontrada", sessionID)
	}

	// ⏳ Aguarda o Handshake terminar se ele ainda estiver rolando em background
	if session.ACPSessID == "" {
		fmt.Printf("[ACP] ⏳ Sessão %s ainda sem ID ACP. Aguardando estabilização...\n", sessionID)
		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			if session.ACPSessID != "" {
				break
			}
		}
		// Se ainda assim continuar vazio, então desistimos.
		if session.ACPSessID == "" {
			return fmt.Errorf("sessão não initializada completamente (sem ACP sessionId)")
		}
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

// AskSync envia um prompt (com imagens opcionais) e aguarda a resposta completa da IA (Bloqueante).
func (e *ACPExecutor) AskSync(sessionID string, prompt string, images []map[string]string) (string, error) {
	e.Mu.Lock()
	_, ok := e.ActiveSessions[sessionID]
	e.Mu.Unlock()

	if !ok {
		return "", fmt.Errorf("sessão '%s' não encontrada para AskSync", sessionID)
	}

	// Prepara o canal para receber os chunks
	ch := make(chan string, 512)
	e.turnMu.Lock()
	e.turnChannels[sessionID] = ch
	e.turnMu.Unlock()

	// Envia o prompt (suporta imagens)
	err := e.SendInput(sessionID, prompt, images)
	if err != nil {
		return "", err
	}

	// Coleta os chunks até o canal ser fechado pelo HandleNotification
	var fullResponse strings.Builder
	timeout := time.After(60 * time.Second)

	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				// Turno completo com sucesso
				return fullResponse.String(), nil
			}
			fullResponse.WriteString(chunk)
		case <-timeout:
			return "", fmt.Errorf("timeout aguardando resposta completa do agente")
		}
	}
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

// ListSessions recupera a lista de conversas salvas diretamente do sistema de arquivos.
// O Gemini CLI v0.36.0 removeu o suporte a 'session/list' via RPC, então usamos este Fallback.
func (e *ACPExecutor) ListSessions(s *ACPSession) ([]SessionInfo, error) {
	// 1. Determinar o diretório de base (.gemini)
	userHome, _ := os.UserHomeDir()
	sessionHome := filepath.Join(userHome, ".gemini") // Padrão global da CLI
	
	cwd, _ := os.Getwd()
	if cfg, errCfg := config.Load(); errCfg == nil {
		for _, acc := range cfg.GeminiAccounts {
			if acc.Active && acc.HomeDir != "" {
				sessionHome = acc.HomeDir
				break
			}
		}
	} else {
		// Fallback para .gemini local no projeto (como visto em @[c:\git\IA\Lumaestro\.gemini])
		if _, err := os.Stat(filepath.Join(cwd, ".gemini")); err == nil {
			sessionHome = filepath.Join(cwd, ".gemini")
		}
	}
	
	// ⚡ DESCOBERTA DE PROJETO (Gemini v0.36 style)
	projectID := "lumaestro"
	projectsPath := filepath.Join(sessionHome, "projects.json")
	if data, err := os.ReadFile(projectsPath); err == nil {
		var p struct {
			Projects map[string]string `json:"projects"`
		}
		if json.Unmarshal(data, &p) == nil {
			// Procura o ID para o diretório atual (cwd)
			for path, id := range p.Projects {
				if strings.EqualFold(path, cwd) {
					projectID = id
					break
				}
			}
		}
	}

	sessionsDirs := []string{
		filepath.Join(sessionHome, "history", projectID),
		filepath.Join(sessionHome, "history", "ia"),
		filepath.Join(sessionHome, "history", "lumaestro"),
		filepath.Join(sessionHome, "history", "lumaestro-1"),
		filepath.Join(sessionHome, "tmp", "lumaestro", "chats"),
		filepath.Join(sessionHome, "tmp", "lumaestro-1", "chats"),
		filepath.Join(sessionHome, "sessions"),
	}

	var finalList []SessionInfo
	visited := make(map[string]bool)

	for _, dirPath := range sessionsDirs {
		if _, err := os.Stat(dirPath); err != nil {
			continue
		}

		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".json") && f.Name() != "index.json" {
				path := filepath.Join(dirPath, f.Name())
				if visited[path] {
					continue
				}
				visited[path] = true

				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				var meta struct {
					ID        string `json:"id"`
					SessID    string `json:"sessionId"` // Novo campo na v0.36.0
					Title     string `json:"title"`
					DispName  string `json:"displayName"` // Algumas versões usam displayName
					UpdatedAt string `json:"updatedAt"`
					CreatedAt string `json:"createdAt"`
				}
				if err := json.Unmarshal(data, &meta); err == nil {
					// Fallback de ID: Prioriza sessionId se disponível
					finalID := meta.ID
					if meta.SessID != "" {
						finalID = meta.SessID
					}

					// Fallback de Título
					title := meta.Title
					if title == "" {
						title = meta.DispName
					}
					if title == "" {
						title = strings.TrimSuffix(f.Name(), ".json")
					}

					info, _ := f.Info()
					updatedAt := meta.UpdatedAt
					if updatedAt == "" {
						updatedAt = info.ModTime().Format(time.RFC3339)
					}

					finalList = append(finalList, SessionInfo{
						SessionID: finalID,
						Title:     title,
						UpdatedAt: updatedAt,
						File:      path, // 🚩 Caminho físico para deleção
					})
				}
			}
		}
	}

	// Ordenar por data (mais recente primeiro)
	sort.Slice(finalList, func(i, j int) bool {
		return finalList[i].UpdatedAt > finalList[j].UpdatedAt
	})

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

// DeleteSession remove o arquivo físico de uma Sinfonia.
func (e *ACPExecutor) DeleteSession(filePath string) error {
	e.Mu.Lock()
	defer e.Mu.Unlock()

	// 🛡️ Segurança: Garantir que o arquivo está dentro da pasta .gemini do projeto atual
	cwd, _ := os.Getwd()
	geminiPath := filepath.Join(cwd, ".gemini")
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(geminiPath)) {
		return fmt.Errorf("🛡️ BLOQUEIO DE SEGURANÇA: Não é permitido deletar arquivos fora da pasta .gemini do projeto")
	}

	fmt.Printf("[ACP] Deletando Sinfonia: %s\n", filePath)
	
	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("falha ao deletar arquivo: %v", err)
	}

	// Notifica a UI para atualizar a lista
	runtime.EventsEmit(e.Ctx, "agent:turn_complete", "system")
	
	return nil
}
