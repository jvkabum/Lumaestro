package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"Lumaestro/internal/config"
	"Lumaestro/internal/db"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartSession inicia o Gemini CLI com a flag --acp. Se loadSessionID for fornecido, tenta restaurar essa sessão em vez de criar uma nova.
func (e *ACPExecutor) StartSession(ctx context.Context, agent string, sessionID string, loadSessionID string, agentID uuid.UUID, issueID *uuid.UUID, planMode bool, parent *ACPSession) error {
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
	cfgLoaded, _ := config.Load()

	// Resolver binário de forma robusta
	binaryPath := agent
	approvalMode := "yolo"
	if planMode {
		approvalMode = "plan"
	}
	args := []string{"--acp", "--approval-mode=" + approvalMode}

	// 💎 Injeção Dinâmica de Modelo (Gemini)
	if agent == "gemini" && cfgLoaded != nil && cfgLoaded.GeminiModel != "" {
		if !strings.HasPrefix(cfgLoaded.GeminiModel, "auto-") {
			args = append(args, "--model="+cfgLoaded.GeminiModel)
			fmt.Printf("[ACP] 🎯 Forçando modelo Gemini: %s\n", cfgLoaded.GeminiModel)
		}
	}

	if agent == "lmstudio" {
		binaryPath = "go"
		args = []string{"run", "./cmd/lmstudio-acp"}
	}

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
		baseDir := filepath.Dir(binaryPath)
		jsPathGlobalDist := filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "dist", "index.js")
		jsPathLocalDist := filepath.Join(baseDir, "..", "@google", "gemini-cli", "dist", "index.js")
		jsPathGlobalBundle := filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "bundle", "gemini.js")
		jsPathLocalBundle := filepath.Join(baseDir, "..", "@google", "gemini-cli", "bundle", "gemini.js")

		jsTarget := ""
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
			args = []string{"--no-warnings=DEP0040", jsTarget, "--acp", "--approval-mode=" + approvalMode}
			fmt.Printf("[ACP] Bypass CMD ativado: Rodando diretamente Node em %s (Modo: %s)\n", jsTarget, approvalMode)
		}
	}

	if absPath, errAbs := filepath.Abs(binaryPath); errAbs == nil && binaryPath != "node" && binaryPath != "go" {
		binaryPath = absPath
	}

	fmt.Printf("[ACP] Executando: %s %v\n", binaryPath, args)

	cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
	cmd.Dir, _ = os.Getwd()
	cmd.Env = os.Environ()

	cwd, _ := os.Getwd()
	sessionHome := cwd
	isUsingOAuth := true
	if cfgLoaded != nil && cfgLoaded.UseGeminiAPIKey {
		isUsingOAuth = false
	}

	// 🌐 Lógica de Autenticação Híbrida (Lumaestro 2.0)
	userHome, _ := os.UserHomeDir()
	globalGeminiHome := filepath.Join(userHome, ".gemini")

	if isUsingOAuth {
		if agent == "gemini" {
			// Motores principais: Usar o Home do usuário onde reside a pasta .gemini
			sessionHome = userHome
			fmt.Printf("[ACP] 🌐 Motor Central: Usando Perfil em %s (Base .gemini)\n", sessionHome)
		} else {
			// Contas Gemini do Projeto/Sub-agentes: Tentar local primeiro
			if _, err := os.Stat(filepath.Join(cwd, ".gemini")); err == nil {
				sessionHome = cwd
				fmt.Printf("[ACP] 📂 Conta de Projeto: Usando Login Local em %s\n", sessionHome)
			} else {
				sessionHome = globalGeminiHome // Fallback se não houver isolamento local
			}
		}

		// Se houver uma conta específica ATIVA no pool (GeminiAccounts), ela tem prioridade total
		if cfgLoaded != nil {
			for _, acc := range cfgLoaded.GeminiAccounts {
				if acc.Active && acc.HomeDir != "" {
					sessionHome = acc.HomeDir
					fmt.Printf("[ACP] 👤 Conta Pool Ativa: Direcionando para %s\n", sessionHome)
					break
				}
			}
		}
	}

	cmd.Env = append(cmd.Env, "GEMINI_CLI_HOME="+sessionHome)
	if agent == "lmstudio" && cfgLoaded != nil {
		cmd.Env = append(cmd.Env, "LUMAESTRO_LMSTUDIO_URL="+cfgLoaded.LMStudioURL)
		cmd.Env = append(cmd.Env, "LUMAESTRO_LMSTUDIO_MODEL="+cfgLoaded.LMStudioModel)
	}

	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_ENABLED=true")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_TARGET=local")
	diagLog := filepath.Join(sessionHome, "gemini-telemetry.json")
	cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_OUTFILE="+diagLog)

	if cfgLoaded != nil {
		if agent == "gemini" && cfgLoaded.GeminiAPIKey != "" {
			apiKey := cfgLoaded.GetActiveGeminiKey()
			cmd.Env = append(cmd.Env, "GOOGLE_API_KEY="+apiKey)
			cmd.Env = append(cmd.Env, "GEMINI_API_KEY="+apiKey)
			fmt.Printf("[ACP] 🔑 Chave de API ativada via Env (Pool Index: %d)\n", cfgLoaded.GeminiKeyIndex)
		}
		if agent == "claude" && cfgLoaded.ClaudeAPIKey != "" {
			cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+cfgLoaded.ClaudeAPIKey)
		}
	}

	if agentID != uuid.Nil {
		var secrets []db.AgentSecret
		if err := db.InstanceDB.Where("agent_id = ?", agentID).Find(&secrets).Error; err == nil {
			for _, s := range secrets {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", s.Key, s.Value))
			}
		}
	}

	rolloutID := "roll-" + uuid.NewString()
	attemptID := "att-1"
	cmd.Env = append(cmd.Env, "LIGHTNING_ROLLOUT_ID="+rolloutID)
	cmd.Env = append(cmd.Env, "LIGHTNING_ATTEMPT_ID="+attemptID)

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
		ACPSessID:      uuid.NewString(), // Geração de ID interno para o protocolo
		AgentName:      agent,
		Cmd:            cmd,
		Stdin:          stdin,
		Cancel:         cancel,
		Ctx:            cmdCtx,
		initDone:       make(chan struct{}, 1),
		SteeringChan:   make(chan string, 5),
		AgentID:        agentID,
		CurrentIssueID: issueID,
		RolloutID:      rolloutID,
		AttemptID:      attemptID,
		PlanMode:       planMode,
		Subagents:      make(map[string]*ACPSession),
	}

	if parent != nil {
		session.ParentSessionID = parent.ID
		parent.SubagentMu.Lock()
		parent.Subagents[sessionID] = session
		parent.SubagentMu.Unlock()
		fmt.Printf("[Subagent] 🌳 Sessão %s vinculada ao pai: %s\n", sessionID, parent.ID)
	}

	e.ActiveSessions[sessionID] = session

	// 📡 Monitor de Steering: Escuta dicas de direcionamento enquanto a sessão está ativa
	go func(s *ACPSession) {
		for {
			select {
			case hint, ok := <-s.SteeringChan:
				if !ok { return }
				fmt.Printf("[Steering] ⚡ Recebido hint para %s: %s\n", s.ID, hint)
				
				// Emite log para a UI para feedback visual imediato
				e.LogChan <- ExecutionLog{
					Source:  "SYSTEM",
					Content: fmt.Sprintf("⚡ Direcionamento: %s", hint),
					Type:    "system",
				}
				
				// TODO: Se o binário suportar sinal de steering (v0.37+), enviar aqui.
				// Por enquanto, o log sistêmico e o re-prompting manual no próximo turno 
				// servem como fallback estável.
				
			case <-s.Ctx.Done():
				return // Encerra monitor quando o processo morre
			}
		}
	}(session)

	runtime.EventsEmit(e.Ctx, "agent:starting", agent)

	go e.runRPCListener(session, stdout)
	go e.runStderrMonitor(session, stderr)

	time.Sleep(3500 * time.Millisecond)

	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      e.getNextID(),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":1,"clientInfo":{"name":"Lumaestro","version":"2.0.0"},"clientCapabilities":{"fs":{"readTextFile":true,"writeTextFile":true}}}`),
	})

	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 1 (initialize) concluído.")
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout no 'initialize' do Gemini (API instável)")
	}

	methodId := "oauth-personal"
	shouldAuthenticate := true

	if agent == "lmstudio" {
		methodId = "lmstudio-local"
	} else if cfgLoaded != nil && cfgLoaded.UseGeminiAPIKey {
		methodId = "gemini-api-key"
	} else {
		// 🌐 Lógica de Silêncio: Se já houver credenciais OAuth, não pede login de novo
		userHome, _ := os.UserHomeDir()
		credsPath := filepath.Join(userHome, ".gemini", "oauth_creds.json")
		if _, err := os.Stat(credsPath); err == nil {
			fmt.Printf("[ACP] 🛡️ Credenciais OAuth detectadas em %s. Pulando login redundante.\n", credsPath)
			shouldAuthenticate = false
		}
	}

	if shouldAuthenticate {
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      e.getNextID(),
			Method:  "authenticate",
			Params:  json.RawMessage(`{"methodId":"` + methodId + `"}`),
		})

		select {
		case <-session.initDone:
			fmt.Println("[ACP] Estágio 2 (authenticate) concluído.")
		case <-time.After(60 * time.Second):
			return fmt.Errorf("timeout no 'authenticate' do Gemini (Autenticação lenta)")
		}
	}

	if loadSessionID != "" {
		targetID := loadSessionID
		if loadSessionID == "LATEST" {
			// 🚀 BUSCA DINÂMICA: Tenta achar a sessão mais recente no sistema de arquivos
			targetID = e.findLatestSessionID(sessionHome)
			if targetID != "" {
				fmt.Printf("[ACP] 🕰️ Última sessão detectada: %s. Tentando restauração...\n", targetID)
			} else {
				fmt.Println("[ACP] Nenhuma sessão anterior encontrada. Iniciando conversa limpa.")
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
				targetID = ""
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
		// Modo padrão: Criar nova se não houver flag de restauração
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      e.getNextID(),
			Method:  "session/new",
			Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `","mcpServers":[]}`),
		})
	}

	select {
	case <-session.initDone:
		fmt.Println("[ACP] Estágio 3 concluído. Sessão pronta!")
	case <-time.After(60 * time.Second):
		return fmt.Errorf("timeout no estágio 3 do Gemini (Criação de sessão lenta)")
	}

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

	runtime.EventsEmit(e.Ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "ACP (JSON-RPC)",
		"isRealPTY": false,
	})

	go func() {
		err := cmd.Wait()

		e.Mu.Lock()
		currentSession, isCurrentlyActive := e.ActiveSessions[sessionID]
		stillActive := isCurrentlyActive && currentSession.Cmd == cmd
		if stillActive {
			delete(e.ActiveSessions, sessionID)
		}
		e.Mu.Unlock()

		if err != nil && stillActive {
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


// ListSessions recupera a lista de conversas salvas diretamente do sistema de arquivos.
func (e *ACPExecutor) ListSessions(s *ACPSession) ([]SessionInfo, error) {
	// 1. Determinar o diretório de base (.gemini)
	userHome, _ := os.UserHomeDir()
	sessionHome := filepath.Join(userHome, ".gemini")

	cwd, _ := os.Getwd()
	if cfg, errCfg := config.Load(); errCfg == nil {
		for _, acc := range cfg.GeminiAccounts {
			if acc.Active && acc.HomeDir != "" {
				sessionHome = acc.HomeDir
				break
			}
		}
	} else {
		if _, err := os.Stat(filepath.Join(cwd, ".gemini")); err == nil {
			sessionHome = filepath.Join(cwd, ".gemini")
		}
	}

	projectID := "lumaestro"
	projectsPath := filepath.Join(sessionHome, "projects.json")
	if data, err := os.ReadFile(projectsPath); err == nil {
		var p struct {
			Projects map[string]string `json:"projects"`
		}
		if json.Unmarshal(data, &p) == nil {
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
					SessID    string `json:"sessionId"`
					Title     string `json:"title"`
					DispName  string `json:"displayName"`
					UpdatedAt string `json:"updatedAt"`
					CreatedAt string `json:"createdAt"`
				}
				if err := json.Unmarshal(data, &meta); err == nil {
					finalID := meta.ID
					if meta.SessID != "" {
						finalID = meta.SessID
					}

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
						File:      path,
					})
				}
			}
		}
	}

	sort.Slice(finalList, func(i, j int) bool {
		return finalList[i].UpdatedAt > finalList[j].UpdatedAt
	})

	return finalList, nil
}

// LoadSession restaura uma sessão específica (Checkpoint).
func (e *ACPExecutor) LoadSession(s *ACPSession, acpSessionID string) error {
	s.ACPSessID = acpSessionID

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

	runtime.EventsEmit(e.Ctx, "agent:turn_complete", "system")

	return nil
}

// findLatestSessionID vasculha recursivamente a pasta .gemini/tmp em busca do chat JSON mais recente.
func (e *ACPExecutor) findLatestSessionID(sessionHome string) string {
	var latestFile string
	var latestTime time.Time

	// 🕵️ Sempre buscar dentro de .gemini/tmp, mesmo que o sessionHome seja a raiz do perfil
	userHome, _ := os.UserHomeDir()
	geminiHome := filepath.Join(userHome, ".gemini")
	
	tmpDir := filepath.Join(geminiHome, "tmp")
	if _, err := os.Stat(tmpDir); err != nil {
		return ""
	}

	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Procuramos por arquivos .json dentro de diretórios 'chats'
		if !info.IsDir() && strings.HasSuffix(path, ".json") && strings.Contains(path, "chats") {
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = path
			}
		}
		return nil
	})

	if latestFile != "" {
		data, err := os.ReadFile(latestFile)
		if err == nil {
			var meta struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(data, &meta) == nil && meta.SessionID != "" {
				return meta.SessionID
			}
		}
	}
	return ""
}
