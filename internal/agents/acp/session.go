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
func (e *ACPExecutor) StartSession(ctx context.Context, agent string, sessionID string, loadSessionID string, agentID uuid.UUID, issueID *uuid.UUID, planMode bool, parent *ACPSession) error {
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

	// 📂 Workspace: Usa o diretório de projeto ativo
	cwd := e.Workspace
	
	// 🛡️ Sincroniza o CPI com o Workspace da sessão
	// Se e.Workspace estiver vazio, o CPI entra em modo DESARMADO (Fail-Closed)
	e.CPI = NewCPIValidator(cwd, e.CPI.VaultOrbit)
	e.Proxy.CPI = e.CPI // 🔄 Sincroniza o proxy
	fmt.Printf("[ACP] Protocolo CPI Sincronizado: %s\n", e.CPI.ActiveOrbit)

	// 🛡️ SEGURANÇA: Se o workspace está vazio, o agente NÃO deve saber onde estamos.
	sessionHome := cwd
	if sessionHome == "" {
		// Em modo de contenção, usamos um caminho nulo para o agente
		sessionHome = "" 
	}
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
		
		// 🛡️ ESCURECIMENTO DE AMBIENTE: Não herda todas as variáveis do sistema
		// Deixamos passar apenas o essencial para o Windows/Node e as nossas variáveis.
		var safeEnv []string
		essentialKeys := []string{"SystemRoot", "SystemDrive", "TEMP", "TMP", "COMSPEC", "PATHEXT", "WINDIR", "USERNAME"}
		for _, envVar := range os.Environ() {
			pair := strings.SplitN(envVar, "=", 2)
			if len(pair) < 2 { continue }
			key := pair[0]
			isEssential := false
			for _, ek := range essentialKeys {
				if strings.EqualFold(key, ek) {
					isEssential = true
					break
				}
			}
			// 🕵️ Filtro de PATH: Mantém apenas o essencial para o Node/Git, remove pistas de projetos
			if strings.EqualFold(key, "PATH") {
				isEssential = true
			}

			if isEssential {
				safeEnv = append(safeEnv, envVar)
			}
		}
		cmd.Env = safeEnv

		// 🛰️ ATIVAÇÃO DE TELEMETRIA (Blackbox ACP)
		if agent == "gemini" {
			userHome, _ := os.UserHomeDir()
			logDir := filepath.Join(userHome, ".gemini", "antigravity", "logs")
			_ = os.MkdirAll(logDir, 0755)
			
			telemetryFile := filepath.Join(logDir, "acp-telemetry.json")
			cmd.Env = append(cmd.Env,
				"GEMINI_TELEMETRY_ENABLED=true",
				"GEMINI_TELEMETRY_TARGET=local",
				"GEMINI_TELEMETRY_OUTFILE="+telemetryFile,
			)
			fmt.Printf("[ACP] 🛰️ Telemetria Ativada: %s\n", telemetryFile)
		}

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

			// Se houver uma conta específica ATIVA no pool (Identidades), ela tem prioridade total
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
			Params:  json.RawMessage(`{"protocolVersion":1,"clientInfo":{"name":"Lumaestro","version":"2.0.0"},"clientCapabilities":{"fs":{"readTextFile":false,"writeTextFile":false}}}`),
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
			// 🌐 Lógica de Silêncio: Se já houver credenciais OAuth, não pede login de novo
			methodId = "oauth-personal" // Força o ID correto para modo login
			
			userHome, _ := os.UserHomeDir()
			credsPath := filepath.Join(userHome, ".gemini", "oauth_creds.json")
			tmpDir := filepath.Join(userHome, ".gemini", "tmp")

			// 🚀 HANGAR DE IDENTIDADES (Multi-Account Rotation)
			vaultDir := filepath.Join(userHome, ".gemini", "vault")
			_ = os.MkdirAll(vaultDir, 0755)

			lastIdentityPath := filepath.Join(userHome, ".gemini", "last_identity.txt")
			currentIdentity := "default"
			if cfgLoaded != nil {
				currentIdentity = cfgLoaded.GetActiveGoogleIdentity()
			}

			lastIdentityData, _ := os.ReadFile(lastIdentityPath)
			lastIdentity := strings.TrimSpace(string(lastIdentityData))

			// Se a identidade mudou, salvamos a atual e restauramos a nova
			if lastIdentity != "" && lastIdentity != currentIdentity {
				fmt.Printf("[ACP] 🔄 Rotacionando Identidades (%s -> %s)...\n", lastIdentity, currentIdentity)
				
				// 1. Arquiva a credencial da identidade anterior
				if _, err := os.Stat(credsPath); err == nil {
					oldVaultPath := filepath.Join(vaultDir, lastIdentity + ".json")
					data, _ := os.ReadFile(credsPath)
					_ = os.WriteFile(oldVaultPath, data, 0644)
				}

				// 2. Tenta restaurar a credencial da nova identidade
				newVaultPath := filepath.Join(vaultDir, currentIdentity + ".json")
				if data, err := os.ReadFile(newVaultPath); err == nil {
					_ = os.WriteFile(credsPath, data, 0644)
					fmt.Printf("[ACP] ✅ Credencial de %s restaurada do Hangar.\n", currentIdentity)
				} else {
					// Se não temos no vault, removemos a antiga para forçar novo login uma única vez
					_ = os.Remove(credsPath)
				}
				
				_ = os.RemoveAll(tmpDir) // Limpa cache para evitar conflitos de cookies
			}
			_ = os.WriteFile(lastIdentityPath, []byte(currentIdentity), 0644)

			if _, err := os.Stat(credsPath); err == nil {
				fmt.Printf("[ACP] 🛡️ Credenciais OAuth detectadas em %s.\n", credsPath)
				shouldAuthenticate = false
			} else {
				fmt.Println("[ACP] 🔑 Nenhuma credencial válida. Iniciando fluxo de login no navegador...")
				shouldAuthenticate = true
				_ = os.RemoveAll(tmpDir) 
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
				Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(e.Workspace, "\\", "\\\\") + `","mcpServers":[]}`),
			})
		}
	} else {
		// Modo padrão: Criar nova se não houver flag de restauração
		sessionCreationID = e.getNextID()
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      sessionCreationID,
			Method:  "session/new",
			Params:  json.RawMessage(`{"cwd":"` + strings.ReplaceAll(e.Workspace, "\\", "\\\\") + `","mcpServers":[]}`),
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

	fmt.Println("[ACP] Enviando setSessionMode (auto-approve)...")
	modeParams, _ := json.Marshal(map[string]interface{}{
		"sessionId": session.ACPSessID,
		"modeId":    "auto-approve", // Nome real que o motor processa internamente
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

	utils.SafeEmit(e.Ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "ACP (JSON-RPC)",
		"isRealPTY": false,
	})

	return nil
}


// ListSessions recupera a lista de conversas salvas diretamente do sistema de arquivos.
func (e *ACPExecutor) ListSessions(s *ACPSession) ([]SessionInfo, error) {
	// 1. Determinar o diretório de base (.gemini) e incluir TODOS os pilotos conhecidos
	userHome, _ := os.UserHomeDir()
	
	// Lista de diretórios para varredura
	var sessionHomes []string
	sessionHomes = append(sessionHomes, filepath.Join(userHome, ".gemini"))

	cwd, _ := os.Getwd()
	if cfg, errCfg := config.Load(); errCfg == nil {
		for _, id := range cfg.Identities {
			if id.Provider == "google" && id.HomeDir != "" {
				// 🛡️ Adiciona o diretório de cada piloto à varredura global
				sessionHomes = append(sessionHomes, filepath.Join(id.HomeDir, ".gemini"))
			}
		}
	} else {
		if _, err := os.Stat(filepath.Join(cwd, ".gemini")); err == nil {
			sessionHomes = append(sessionHomes, filepath.Join(cwd, ".gemini"))
		}
	}

	projectID := "lumaestro"
	// 🎯 Varredura Dinâmica de Project ID em todos os Homes
	for _, sHome := range sessionHomes {
		projectsPath := filepath.Join(sHome, "projects.json")
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
	}

	var sessionsDirs []string
	for _, sHome := range sessionHomes {
		sessionsDirs = append(sessionsDirs,
			filepath.Join(sHome, "history", projectID),
			filepath.Join(sHome, "history", "ia"),
			filepath.Join(sHome, "history", "lumaestro"),
			filepath.Join(sHome, "history", "lumaestro-1"),
			filepath.Join(sHome, "tmp", "lumaestro", "chats"),
			filepath.Join(sHome, "tmp", "lumaestro-1", "chats"),
			filepath.Join(sHome, "sessions"),
		)
	}

	var finalList []SessionInfo
	visited := make(map[string]bool)

	fmt.Printf("[ListSessions] 🛰️ Iniciando varredura global em %d diretórios base...\n", len(sessionHomes))

	for _, dirPath := range sessionsDirs {
		if _, err := os.Stat(dirPath); err != nil {
			continue
		}

		fmt.Printf("[ListSessions] 📂 Varrendo: %s\n", dirPath)
		files, err := os.ReadDir(dirPath)
		if err != nil {
			fmt.Printf("[ListSessions] ❌ Erro ao ler diretório %s: %v\n", dirPath, err)
			continue
		}

		foundInDir := 0
		for _, f := range files {
			name := f.Name()
			if !f.IsDir() && (strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonl")) && name != "index.json" {
				foundInDir++
				path := filepath.Join(dirPath, f.Name())
				if visited[path] {
					continue
				}
				visited[path] = true
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}

				// 🛠️ Suporte a JSONL: Se for .jsonl, pegamos apenas a primeira linha para o Unmarshal de meta
				jsonToParse := data
				if strings.HasSuffix(name, ".jsonl") {
					lines := strings.Split(string(data), "\n")
					if len(lines) > 0 {
						jsonToParse = []byte(lines[0])
					}
				}

				var meta struct {
					ID        string `json:"id"`
					SessID    string `json:"sessionId"`
					Title     string `json:"title"`
					DispName  string `json:"displayName"`
					UpdatedAt string `json:"updatedAt"`
					CreatedAt string `json:"createdAt"`
				}
				if err := json.Unmarshal(jsonToParse, &meta); err == nil {
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
		if foundInDir > 0 {
			fmt.Printf("[ListSessions] ✅ Encontradas %d Sinfonias em %s\n", foundInDir, dirPath)
		}
	}

	sort.Slice(finalList, func(i, j int) bool {
		return finalList[i].UpdatedAt > finalList[j].UpdatedAt
	})

	fmt.Printf("[ListSessions] ✨ Varredura completa. Total unificado: %d Sinfonias.\n", len(finalList))
	return finalList, nil
}

// LoadSession restaura uma sessão específica (Checkpoint).
func (e *ACPExecutor) LoadSession(s *ACPSession, acpSessionID string) error {
	s.ACPSessID = acpSessionID

	id := e.getNextID()
	// 🛡️ SEGURANÇA: Usar o workspace autorizado do executor, não o CWD do processo
	params := map[string]interface{}{
		"sessionId":  acpSessionID,
		"cwd":        e.Workspace,
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
	userHome, _ := os.UserHomeDir()
	globalGeminiPath := filepath.Join(userHome, ".gemini")

	cleanPath := filepath.Clean(filePath)
	allowedLocal := strings.HasPrefix(cleanPath, filepath.Clean(geminiPath))
	allowedGlobal := strings.HasPrefix(cleanPath, filepath.Clean(globalGeminiPath))

	if !allowedLocal && !allowedGlobal {
		return fmt.Errorf("🛡️ BLOQUEIO DE SEGURANÇA: Não é permitido deletar arquivos fora das pastas .gemini autorizadas")
	}

	fmt.Printf("[ACP] Deletando Sinfonia: %s\n", filePath)

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("falha ao deletar arquivo: %v", err)
	}

	utils.SafeEmit(e.Ctx, "agent:turn_complete", "system")

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
		// Procuramos por arquivos .json ou .jsonl dentro de diretórios 'chats'
		if !info.IsDir() && (strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".jsonl")) && strings.Contains(path, "chats") {
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
