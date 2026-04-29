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
	"Lumaestro/internal/utils"
)

// StartSession inicia o Gemini CLI com a flag --acp. Se loadSessionID for fornecido, tenta restaurar essa sessão em vez de criar uma nova.
func (e *ACPExecutor) StartSession(ctx context.Context, agent string, sessionID string, loadSessionID string, agentID uuid.UUID, issueID *uuid.UUID, planMode bool, parent *ACPSession, agentCWD string) error {
	e.Mu.Lock()
	e.Ctx = ctx
	e.Tools.Ctx = ctx

	var session *ACPSession
	var isHotSwap bool

	if s, ok := e.ActiveSessions[sessionID]; ok {
		if s.Cmd != nil && s.Cmd.ProcessState == nil {
			isHotSwap = true
			session = s
			fmt.Printf("[ACP] ♻️ Hot Swap: Reutilizando processo CLI nativo para o agente: %s\n", sessionID)
		} else {
			if s.Cancel != nil {
				s.Cancel()
			}
			delete(e.ActiveSessions, sessionID)
		}
	}
	e.Mu.Unlock()

	// 📂 Workspace: Usa o diretório de projeto ativo, ou fallback para CWD do Lumaestro
	cwd := agentCWD
	if cwd == "" {
		cwd = e.Workspace
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	sessionHome := cwd
	cfgLoaded, _ := config.Load()

	if !isHotSwap {
		cmdCtx, cancel := context.WithCancel(ctx)

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

		if agent == "native" {
			binaryPath = "go"
			args = []string{"run", "./cmd/lmstudio-acp"}
		}

		// 1. Tenta binário global (LookPath)
		if globalPath, errGL := exec.LookPath(binaryPath); errGL == nil {
			binaryPath = globalPath
		} else {
			// 2. Fallback para node_modules local (estilo dev)
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
				// Adiciona --debug para logs detalhados
				args = []string{"--no-warnings=DEP0040", jsTarget, "--acp", "--approval-mode=" + approvalMode, "--debug"}
				fmt.Printf("[ACP] Bypass CMD ativado: Rodando diretamente Node em %s (Modo: %s)\n", jsTarget, approvalMode)
			}
		}

		if absPath, errAbs := filepath.Abs(binaryPath); errAbs == nil && binaryPath != "node" && binaryPath != "go" {
			binaryPath = absPath
		}

		fmt.Printf("[ACP] Executando: %s %v\n", binaryPath, args)

		cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
		cmd.Dir = cwd
		cmd.Env = os.Environ()

		isUsingOAuth := true
		if cfgLoaded != nil && cfgLoaded.UseGeminiAPIKey {
			isUsingOAuth = false
		}

		// 🌐 LÓGICA MOTHERSHIP: Tudo converge para o diretório raiz do Lumaestro
		baseAppPath, _ := os.Getwd()
		mothershipHome := filepath.Join(baseAppPath, ".lumaestro")
		os.MkdirAll(mothershipHome, 0755)

		sessionHome = mothershipHome
		fmt.Printf("[SOBERANIA] 🛸 Arquitetura Mothership Ativada: Gravando em %s\n", sessionHome)

		if isUsingOAuth {
			// 🌉 PONTE DE CREDENCIAIS (Mothership): Copia as credenciais do usuário para a Nave Mãe
			userHome, _ := os.UserHomeDir()
			globalCreds := filepath.Join(userHome, ".gemini", "oauth_creds.json")
			localCreds := filepath.Join(sessionHome, "oauth_creds.json")

			if _, err := os.Stat(globalCreds); err == nil {
				if _, errLocal := os.Stat(localCreds); errLocal != nil {
					data, _ := os.ReadFile(globalCreds)
					os.WriteFile(localCreds, data, 0600)
					fmt.Println("[SOBERANIA] 🌉 Ponte de Credenciais transferida para a raiz da Mothership.")
				}
			}

			// Se houver uma conta específica ATIVA no pool (Identidades), ela pode apontar para outro local (Avançado)
			if cfgLoaded != nil {
				for _, id := range cfgLoaded.Identities {
					if id.Provider == "google" && id.Active && id.HomeDir != "" {
						sessionHome = id.HomeDir
						fmt.Printf("[ACP] 👤 Identidade Google Ativa: Direcionando para %s\n", sessionHome)
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

		if agent == "native" {
			// No modo Cloud-Local, o agente 'native' (chat) é desativado para economizar VRAM.
			// O usuário deve usar Gemini ou Claude para o chat/ACP.
			return fmt.Errorf("o motor de chat nativo (8087) foi desativado em favor do modo Híbrido Cloud-Local. Use Gemini ou Claude")
		}

		cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_ENABLED=true")
		cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_TARGET=local")
		// Salva telemetria na pasta do projeto para fácil inspeção
		cmd.Env = append(cmd.Env, "GEMINI_TELEMETRY_OUTFILE=.lumaestro/telemetry.json")

		if cfgLoaded != nil {
			// 🔑 Injeção de Chave de API apenas se o usuário explicitamente optou por este modo
			if agent == "gemini" && cfgLoaded.UseGeminiAPIKey && cfgLoaded.GeminiAPIKey != "" {
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

		session = &ACPSession{
			ID:             sessionID,
			ACPSessID:      "",               // Aguarda o ID real retornado pelo comando 'newSession'
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

		e.Mu.Lock()
		e.ActiveSessions[sessionID] = session
		e.Mu.Unlock()

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

		utils.SafeEmit(e.Ctx, "agent:starting", agent)

		go e.runRPCListener(session, stdout)
		go e.runStderrMonitor(session, stderr)

		go func() {
			errWait := cmd.Wait()

			e.Mu.Lock()
			currentSession, isCurrentlyActive := e.ActiveSessions[sessionID]
			stillActive := isCurrentlyActive && currentSession.Cmd == cmd
			if stillActive {
				delete(e.ActiveSessions, sessionID)
			}
			e.Mu.Unlock()

			if errWait != nil && stillActive {
				if cmdCtx.Err() == nil {
					e.LogChan <- ExecutionLog{
						Source:  "ERROR",
						Content: fmt.Sprintf("⚠️ Sinfonia interrompida abruptamente: %v", errWait),
					}
				}
			}

			if stillActive {
				fmt.Printf("[ACP] Sessão processo OS %s encerrada.\n", agent)
				utils.SafeEmit(e.Ctx, "terminal:closed", agent)
				e.LogChan <- ExecutionLog{
					Source:  "SYSTEM",
					Content: "Sessão ACP " + agent + " encerrada do sistema.",
				}
			}
		}()

		initID := e.getNextID()
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      initID,
			Method:  "initialize",
			Params:  json.RawMessage(`{"protocolVersion":1,"clientInfo":{"name":"Lumaestro","version":"2.0.0"},"clientCapabilities":{"fs":{"readTextFile":true,"writeTextFile":true}}}`),
		})

		if _, err := e.waitForResponse(initID, 60*time.Second); err != nil {
			return fmt.Errorf("timeout/erro no 'initialize' do Gemini: %v", err)
		}
		fmt.Println("[ACP] Estágio 1 (initialize) concluído.")

		methodId := ""
		shouldAuthenticate := true
		if agent == "claude" {
			methodId = "claude-api-key"
		} else if agent == "lmstudio" || agent == "native" {
			methodId = "lmstudio-local"
		} else if cfgLoaded != nil && cfgLoaded.UseGeminiAPIKey {
			methodId = "gemini-api-key"
		} else {
			// 🌐 Força Autenticação: O motor exige o comando authenticate para validar o arquivo na memória
			methodId = "oauth-personal" // Força o ID correto para modo login
			credsPath := filepath.Join(sessionHome, "oauth_creds.json")
			if _, err := os.Stat(credsPath); err == nil {
				fmt.Printf("[ACP] 🛡️ Credenciais OAuth detectadas na Mothership (%s). Validando no motor...\n", credsPath)
			}
		}

		if shouldAuthenticate {
			authID := e.getNextID()
			e.SendRPC(session, JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      authID,
				Method:  "authenticate",
				Params:  json.RawMessage(`{"methodId":"` + methodId + `"}`),
			})
			if _, err := e.waitForResponse(authID, 60*time.Second); err != nil {
				return fmt.Errorf("timeout/erro no 'authenticate': %v", err)
			}
			fmt.Println("[ACP] Estágio 2 (authenticate) concluído.")
		}
	}

	var sessionCreationID int
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
				fmt.Printf("[ACP] Sessão anterior (%s) restaurada com sucesso!\n", targetID)
			} else {
				fmt.Printf("[ACP] Erro ao carregar sessão anterior (tentando nova): %v\n", errLoad)
				targetID = ""
				session.ACPSessID = "" // Limpa o ID inválido
			}
		}

		if targetID == "" {
			sessionCreationID = e.getNextID()
			e.SendRPC(session, JSONRPCMessage{
				JSONRPC: JSONRPCVersion,
				ID:      sessionCreationID,
				Method:  "session/new",
				Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `","mcpServers":[]}`),
			})
		}
	} else {
		// Modo padrão: Criar nova se não houver flag de restauração
		sessionCreationID = e.getNextID()
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      sessionCreationID,
			Method:  "session/new",
			Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(cwd, "\\", "\\\\") + `","mcpServers":[]}`),
		})
	}

	if sessionCreationID != 0 {
		if msg, err := e.waitForResponse(sessionCreationID, 60*time.Second); err != nil {
			return fmt.Errorf("timeout/erro no estágio 3 do Gemini (Criação de sessão lenta): %v", err)
		} else {
			var response map[string]interface{}
			if json.Unmarshal(msg.Result, &response) == nil && response != nil {
				if sessID, ok := response["sessionId"].(string); ok {
					session.ACPSessID = sessID
				}
			}
		}
		fmt.Println("[ACP] Estágio 3 concluído. Sessão pronta!")
	}

	// 🛸 Sincronização inicial da Mothership realizada via ensureMothershipSync no início do fluxo.

	fmt.Println("[ACP] Enviando setSessionMode (yolo)...")
	modeParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"modeId":    "yolo", // Gemini v0.40.0 usa 'yolo' em vez de 'auto-approve'
	})
	
	setModeID := e.getNextID()
	e.SendRPC(session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      setModeID,
		Method:  "session/set_mode",
		Params:  modeParams,
	})
	// Espera e engole silenciosamente se a CLI der Internal Error para newly created sessions.
	_, _ = e.waitForResponse(setModeID, 5*time.Second)

	// 🔓 SINALIZAÇÃO DE PRONTIDÃO: Avisa que o boot foi concluído com sucesso (Apenas uma vez)
	select {
	case <-session.initDone:
		// Já fechado, ignora
	default:
		close(session.initDone)
	}

	utils.SafeEmit(e.Ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "ACP (JSON-RPC)",
		"isRealPTY": false,
	})

	return nil
}


// ListSessions recupera a lista de conversas salvas diretamente do sistema de arquivos.
func (e *ACPExecutor) ListSessions(s *ACPSession) ([]SessionInfo, error) {
	baseAppPath, _ := os.Getwd()
	sessionHome := filepath.Join(baseAppPath, ".lumaestro")
	
	// 🚀 Sincroniza a Mothership antes de listar (Garate que histórico global apareça)
	e.ensureMothershipSync(sessionHome)

	// Se não houver Mothership, tentamos o fallback para a identidade global (legado)
	if _, err := os.Stat(sessionHome); err != nil {
		if cfg, errCfg := config.Load(); errCfg == nil {
			for _, id := range cfg.Identities {
				if id.Provider == "google" && id.Active && id.HomeDir != "" {
					sessionHome = id.HomeDir
					break
				}
			}
		}
	}

	cwd := e.Workspace
	if cwd == "" {
		cwd = baseAppPath
	}

	// 2. Tentar identificar o ID do Projeto para filtrar o histórico
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

	// 🚀 UNIFICAÇÃO MOTHERSHIP: O Lumaestro agora prioriza a estrutura nativa do motor dentro da pasta soberana
	sessionsDirs := []string{
		filepath.Join(sessionHome, ".gemini", "tmp", "lumaestro", "chats"),
		filepath.Join(sessionHome, ".gemini", "history", projectID),
		filepath.Join(sessionHome, ".gemini", "history", "ia"),
		filepath.Join(sessionHome, ".gemini", "history", "lumaestro"),
		filepath.Join(sessionHome, "history"), // Fallback para migrações antigas
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
			isHistory := strings.HasSuffix(f.Name(), ".json") || strings.HasSuffix(f.Name(), ".jsonl")
			if !f.IsDir() && isHistory && f.Name() != "index.json" {
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
				
				if err := json.Unmarshal(data, &meta); err != nil {
					// Fallback para JSONL: Tenta ler apenas a primeira linha
					lines := strings.Split(string(data), "\n")
					if len(lines) > 0 {
						json.Unmarshal([]byte(lines[0]), &meta)
					}
				}

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
					title = strings.TrimSuffix(title, ".jsonl")
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
	localGeminiPath := filepath.Join(cwd, ".gemini")
	mothershipPath := filepath.Join(cwd, ".lumaestro")
	userHome, _ := os.UserHomeDir()
	globalGeminiPath := filepath.Join(userHome, ".gemini")

	cleanPath := filepath.Clean(filePath)
	allowedLocal := strings.HasPrefix(cleanPath, filepath.Clean(localGeminiPath))
	allowedGlobal := strings.HasPrefix(cleanPath, filepath.Clean(globalGeminiPath))
	allowedMothership := strings.HasPrefix(cleanPath, filepath.Clean(mothershipPath))

	if !allowedLocal && !allowedGlobal && !allowedMothership {
		return fmt.Errorf("🛡️ BLOQUEIO DE SEGURANÇA: Não é permitido deletar arquivos fora das pastas autorizadas (.gemini ou .lumaestro)")
	}

	fmt.Printf("[ACP] Deletando Sinfonia: %s\n", filePath)

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("falha ao deletar arquivo: %v", err)
	}

	utils.SafeEmit(e.Ctx, "agent:turn_complete", "system")

	return nil
}

// findLatestSessionID vasculha recursivamente a pasta tmp em busca do chat JSON mais recente.
func (e *ACPExecutor) findLatestSessionID(sessionHome string) string {
	var latestFile string
	var latestTime time.Time

	// 🕵️ No Modo Híbrido, sessionHome já aponta para a base correta (.lumaestro ou ~/.gemini)
	// Como o motor sempre cria uma subpasta .gemini, verificamos ambas as possibilidades.
	tmpDirs := []string{
		filepath.Join(sessionHome, ".gemini", "tmp"),
		filepath.Join(sessionHome, "tmp"),
	}

	for _, tmpDir := range tmpDirs {
		if _, err := os.Stat(tmpDir); err == nil {
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
		}
	}

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

// ForceSyncGlobalHistory remove o marcador e executa a sincronização forçada.
func (e *ACPExecutor) ForceSyncGlobalHistory() error {
	baseAppPath, _ := os.Getwd()
	sessionHome := filepath.Join(baseAppPath, ".lumaestro")
	markerPath := filepath.Join(sessionHome, ".migration_done")
	
	// Remove o marcador para permitir nova migração
	os.Remove(markerPath)
	
	fmt.Println("[SOBERANIA] 🔄 Gatilho de Sincronização Manual ativado pelo usuário.")
	e.ensureMothershipSync(sessionHome)
	return nil
}

// ensureMothershipSync garante que os dados vitais da pasta global sejam migrados para a Mothership apenas uma vez.
func (e *ACPExecutor) ensureMothershipSync(sessionHome string) {
	markerPath := filepath.Join(sessionHome, ".migration_done")
	if _, err := os.Stat(markerPath); err == nil {
		return // Migração já realizada anteriormente
	}

	userHome, _ := os.UserHomeDir()
	globalGemini := filepath.Join(userHome, ".gemini")
	
	migrationOccurred := false

	// 1. Migrar projects.json
	globalProjects := filepath.Join(globalGemini, "projects.json")
	localProjects := filepath.Join(sessionHome, ".gemini", "projects.json")
	if _, err := os.Stat(globalProjects); err == nil {
		if _, errEx := os.Stat(localProjects); os.IsNotExist(errEx) {
			os.MkdirAll(filepath.Dir(localProjects), 0755)
			utils.CopyFile(globalProjects, localProjects)
			fmt.Println("[SOBERANIA] 🗺️ Mapa de projetos migrado para a Mothership (.gemini).")
			migrationOccurred = true
		}
	}

	// 2. Migrar histórico de conversas
	globalHistory := filepath.Join(globalGemini, "history")
	localHistory := filepath.Join(sessionHome, ".gemini", "history")
	if _, err := os.Stat(globalHistory); err == nil {
		if entries, errRead := os.ReadDir(globalHistory); errRead == nil && len(entries) > 0 {
			os.MkdirAll(localHistory, 0755)
			for _, entry := range entries {
				oldPath := filepath.Join(globalHistory, entry.Name())
				newPath := filepath.Join(localHistory, entry.Name())
				if _, errEx := os.Stat(newPath); os.IsNotExist(errEx) {
					if entry.IsDir() {
						utils.CopyDir(oldPath, newPath)
					} else {
						utils.CopyFile(oldPath, newPath)
					}
					migrationOccurred = true
				}
			}
			if migrationOccurred {
				fmt.Printf("[SOBERANIA] 📚 %d itens de histórico migrados.\n", len(entries))
			}
		}
	}

	// Cria o marcador para nunca mais re-migrar (Soberania de via única)
	os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)
}
