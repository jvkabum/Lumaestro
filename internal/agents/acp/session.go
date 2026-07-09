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

	"Lumaestro/internal/utils"

	"github.com/google/uuid"
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

	cfgLoaded, _ := config.Load()

	// 📂 Workspace: Usa o diretório de projeto ativo
	if e.Workspace == "" && cfgLoaded != nil && cfgLoaded.ActiveWorkspace != "" {
		e.Workspace = cfgLoaded.ActiveWorkspace
	}
	cwd := e.Workspace

	// 🛡️ Sincroniza o CPI com o Workspace da sessão
	e.CPI = NewCPIValidator(cwd, e.CPI.VaultOrbit)
	e.Proxy.CPI = e.CPI
	fmt.Printf("[ACP] Protocolo CPI Sincronizado: %s\n", e.CPI.ActiveOrbit)

	// 🛡️ ISOLAMENTO FÍSICO: Define a Célula de Isolamento (Sandbox)
	sandboxPath := filepath.Join(e.Workspace, ".lumaestro", "sandbox")
	_ = os.MkdirAll(sandboxPath, 0755)

	// 🛡️ SEGURANÇA: Se o workspace está vazio, usamos a Célula fixa
	sessionHome := cwd
	if cwd == "" || cwd == "." {
		cwd = sandboxPath
		sessionHome = sandboxPath
		fmt.Printf("[ACP] 🛡️ MODO HERMÉTICO: Agente isolado na Célula: %s\n", sandboxPath)
	}

	if !isHotSwap {
		cmdCtx, cancel := context.WithCancel(ctx)

		// Resolver binário de forma robusta
		binaryPath := agent
		approvalMode := "yolo"
		if planMode {
			approvalMode = "plan"
		}
		args := []string{"--acp", "--approval-mode=" + approvalMode, "--skip-trust"}

		// 🚀 Resolução do Google Antigravity CLI / Servidor ACP Oficial
		userHome, _ := os.UserHomeDir()
		isGoogleAgent := (agent == "gemini" || agent == "antigravity" || agent == "agy")
		if isGoogleAgent {
			// 1. Variável de ambiente explícita AGY_BIN
			agyBinEnv := os.Getenv("AGY_BIN")
			if agyBinEnv != "" {
				if _, err := os.Stat(agyBinEnv); err == nil {
					binaryPath = agyBinEnv
					fmt.Printf("[ACP] 🚀 Usando AGY_BIN: %s\n", binaryPath)
				}
			}

			// 2. Servidor ACP oficial de primeira entidade (agy_acp_server / agy_acp_server.exe)
			if binaryPath == agent {
				possibleServers := []string{
					"agy_acp_server",
					filepath.Join(userHome, "AppData", "Local", "agy", "bin", "agy_acp_server.exe"),
					filepath.Join(userHome, ".gemini", "antigravity-cli", "bin", "agy_acp_server.exe"),
					filepath.Join(e.Workspace, "bin", "agy_acp_server.exe"),
				}
				for _, srv := range possibleServers {
					if path, err := exec.LookPath(srv); err == nil {
						binaryPath = path
						args = []string{} // Servidor autônomo já opera nativamente em JSON-RPC sobre stdio
						fmt.Printf("[ACP] 💎 Servidor oficial Antigravity ACP detectado: %s\n", binaryPath)
						break
					} else if _, errStat := os.Stat(srv); errStat == nil {
						binaryPath = srv
						args = []string{}
						fmt.Printf("[ACP] 💎 Servidor oficial Antigravity ACP detectado: %s\n", binaryPath)
						break
					}
				}
			}

			// 3. Se for 'antigravity' ou 'agy' e não tiver agy_acp_server, usa o fallback estável para gemini (com ACP)
			if binaryPath == "antigravity" || binaryPath == "agy" {
				binaryPath = "gemini"
				fmt.Println("[ACP] 🔄 Modo ACP do Antigravity chaveado para motor compatível Gemini CLI")
			}
		}

		// Injeção de argumentos adicionais via AGY_EXTRA_ARGS
		if extraArgs := os.Getenv("AGY_EXTRA_ARGS"); extraArgs != "" {
			args = append(args, strings.Fields(extraArgs)...)
		}

		// 💎 Injeção Dinâmica de Modelo (Gemini / Antigravity)
		if isGoogleAgent && cfgLoaded != nil && cfgLoaded.GeminiModel != "" {
			if !strings.HasPrefix(cfgLoaded.GeminiModel, "auto-") {
				args = append(args, "--model="+cfgLoaded.GeminiModel)
				fmt.Printf("[ACP] 🎯 Forçando modelo: %s\n", cfgLoaded.GeminiModel)
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
			// IMPORTANTE: Busca no e.Workspace (root real)
			binaryPath = filepath.Join(e.Workspace, "node_modules", ".bin", binaryPath+".cmd")
		}

		// [TRUQUE DE SINFONIA] Se estivermos no Windows e for o Gemini, o .cmd (tanto local quanto global)
		// costuma engolir o Stdin em Pipes IPC, quebrando o JSON-RPC. Precisamos bypassar rodando via 'node'.
		jsTarget := ""
		if agent == "gemini" && strings.HasSuffix(binaryPath, ".cmd") {
			baseDir := filepath.Dir(binaryPath)
			// Tenta localizar o index.js ou gemini.js real
			pathsToTry := []string{
				filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "bundle", "gemini.js"),
				filepath.Join(baseDir, "..", "@google", "gemini-cli", "bundle", "gemini.js"),
				filepath.Join(baseDir, "node_modules", "@google", "gemini-cli", "dist", "index.js"),
				filepath.Join(baseDir, "..", "@google", "gemini-cli", "dist", "index.js"),
			}

			for _, p := range pathsToTry {
				if _, err := os.Stat(p); err == nil {
					if abs, errAbs := filepath.Abs(p); errAbs == nil {
						jsTarget = abs
						break
					}
				}
			}

			if jsTarget != "" {
				binaryPath = "node"
				args = []string{jsTarget, "--acp", "--approval-mode=" + approvalMode, "--skip-trust"}
				fmt.Printf("[ACP] 🚀 Bypass Windows: Rodando via Node com caminho absoluto: %s\n", jsTarget)
			}
		}

		if absPath, errAbs := filepath.Abs(binaryPath); errAbs == nil && binaryPath != "node" && binaryPath != "go" {
			binaryPath = absPath
		}

		fmt.Printf("[ACP] Executando: %s %v\n", binaryPath, args)

		cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
		
		// 🛡️ ISOLAMENTO FÍSICO REAL: Agora o processo inicia dentro do Sandbox.
		// Como usamos caminhos absolutos acima, ele não se perderá mais.
		cmd.Dir = sandboxPath
		fmt.Printf("[ACP] 🛡️ MODO HERMÉTICO: Processo iniciado na Célula: %s\n", sandboxPath)

		// 🛡️ HERANÇA E AJUSTE DE MÓDULOS: Mantém o ambiente e garante que o Node ache as bibliotecas
		cmd.Env = os.Environ()
		
		// Adiciona a node_modules da raiz ao NODE_PATH para que o processo no sandbox encontre as dependências
		rootNodeModules, _ := filepath.Abs(filepath.Join(e.Workspace, "node_modules"))
		cmd.Env = append(cmd.Env, "NODE_PATH="+rootNodeModules)
		
		absSessionHome, _ := filepath.Abs(sessionHome)
		_ = absSessionHome // Variável preparada para uso posterior se necessário, mas não injetada agora

		// 🛰️ ATIVAÇÃO DE TELEMETRIA (Blackbox ACP)
		if isGoogleAgent {
			// Centraliza telemetria no Hangar de Sinfonias
			absSinfoniaPath, _ := filepath.Abs(filepath.Join(e.Workspace, ".lumaestro", "sinfonias", "gemini"))
			_ = os.MkdirAll(absSinfoniaPath, 0755)

			telemetryFile := filepath.Join(absSinfoniaPath, "telemetry.json")
			cmd.Env = append(cmd.Env,
				"GEMINI_TELEMETRY_ENABLED=true",
				"AGY_TELEMETRY_ENABLED=true",
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
		userHome, _ = os.UserHomeDir()
		globalGeminiHome := filepath.Join(userHome, ".gemini")

		// 🎼 SINFONIAS: Centraliza o histórico de todos os agentes Gemini/Antigravity
		sinfoniaPath := filepath.Join(e.Workspace, ".lumaestro", "sinfonias", "gemini")
		_ = os.MkdirAll(sinfoniaPath, 0755)

		if isUsingOAuth {
			if isGoogleAgent {
				// Motores principais: Usar o Hangar de Sinfonias centralizado
				sessionHome = sinfoniaPath
				fmt.Printf("[ACP] 🎼 Sinfonia Central: Usando histórico unificado em %s\n", sessionHome)
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

						// 🚀 SINFONIA GLOBAL: Cria Junctions para centralizar o histórico e cache
						targetGemini := sinfoniaPath
						_ = os.MkdirAll(filepath.Join(targetGemini, "history"), 0755)
						_ = os.MkdirAll(filepath.Join(targetGemini, "tmp"), 0755)

						identityGemini := filepath.Join(sessionHome, ".gemini")
						_ = os.MkdirAll(identityGemini, 0755)

						linkDirs := []string{"history", "tmp"}
						for _, dirName := range linkDirs {
							linkPath := filepath.Join(identityGemini, dirName)
							targetPath := filepath.Join(targetGemini, dirName)

							info, err := os.Lstat(linkPath)
							if err == nil {
								if info.Mode()&os.ModeSymlink != 0 {
									continue
								}
								_ = os.RemoveAll(linkPath)
							}

							cmdMklink := exec.Command("cmd", "/c", "mklink", "/J", linkPath, targetPath)
							_ = cmdMklink.Run()
						}
						fmt.Printf("[ACP] 🔗 Sinfonia Junctions ativas para %s\n", sessionHome)
						break
					}
				}
			}
		}

		absFinalSessionHome, _ := filepath.Abs(sessionHome)
		cmd.Env = append(cmd.Env, "GEMINI_CLI_HOME="+absFinalSessionHome)
		cmd.Env = append(cmd.Env, "AGY_HOME="+absFinalSessionHome)

		if agent == "lmstudio" && cfgLoaded != nil {
			cmd.Env = append(cmd.Env, "LUMAESTRO_LMSTUDIO_URL="+cfgLoaded.LMStudioURL)
			cmd.Env = append(cmd.Env, "LUMAESTRO_LMSTUDIO_MODEL="+cfgLoaded.LMStudioModel)
		}

		if agent == "native" {
			return fmt.Errorf("o motor de chat nativo (8087) foi desativado em favor do modo Híbrido Cloud-Local. Use Gemini ou Claude")
		}

		if cfgLoaded != nil {
			// 🔑 Injeção de Chave de API apenas se o usuário explicitamente optou por este modo
			if isGoogleAgent && cfgLoaded.UseGeminiAPIKey && cfgLoaded.GeminiAPIKey != "" {
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
			ACPSessID:      "", // Aguarda o ID real retornado pelo comando 'newSession'
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
					if !ok {
						return
					}
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
			Params:  json.RawMessage(`{"protocolVersion":1,"clientInfo":{"name":"Lumaestro","version":"15.0.0"},"capabilities":{"fileSystem":{"readTextFile":true,"writeTextFile":true},"terminal":{"create":true}},"clientCapabilities":{"fs":{"readTextFile":true,"writeTextFile":true}}}`),
		})

		if _, err := e.waitForResponse(initID, 60*time.Second); err != nil {
			return fmt.Errorf("timeout/erro no 'initialize' do agente: %v", err)
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

			// 🛡️ O Hangar de Identidades agora opera dentro da Sinfonia selecionada (sessionHome)
			// Isso garante que o histórico e as credenciais fiquem centralizados no projeto
			credsPath := filepath.Join(sessionHome, ".gemini", "oauth_creds.json")
			tmpDir := filepath.Join(sessionHome, ".gemini", "tmp")

			// 🚀 HANGAR DE IDENTIDADES (Multi-Account Rotation)
			vaultDir := filepath.Join(sessionHome, ".gemini", "vault")
			_ = os.MkdirAll(vaultDir, 0755)

			lastIdentityPath := filepath.Join(sessionHome, ".gemini", "last_identity.txt")
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
					oldVaultPath := filepath.Join(vaultDir, lastIdentity+".json")
					data, _ := os.ReadFile(credsPath)
					_ = os.WriteFile(oldVaultPath, data, 0644)
				}

				// 2. Tenta restaurar a credencial da nova identidade
				newVaultPath := filepath.Join(vaultDir, currentIdentity+".json")
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

	// 🕰️ RESTAURAÇÃO DE SESSÃO: Se loadSessionID for "LATEST", busca a mais recente.
	if loadSessionID == "LATEST" || loadSessionID == "" {
		lastSessionPath := filepath.Join(e.Workspace, ".lumaestro", "last_session.json")
		if data, err := os.ReadFile(lastSessionPath); err == nil {
			var lastSession struct {
				SessionID string `json:"sessionId"`
			}
			if json.Unmarshal(data, &lastSession) == nil && lastSession.SessionID != "" {
				loadSessionID = lastSession.SessionID
				fmt.Printf("[ACP] 🕰️ Última sessão recuperada de last_session.json: %s\n", loadSessionID)
			}
		}

		if loadSessionID == "" || loadSessionID == "LATEST" {
			fmt.Printf("[ACP] 🕰️ Buscando última Sinfonia para o agente %s...\n", agent)
			sessions, err := e.ListSessions(nil)
			if err == nil && len(sessions) > 0 {
				loadSessionID = sessions[0].SessionID
				if loadSessionID == "" {
					loadSessionID = sessions[0].File
				}
				fmt.Printf("[ACP] 🕰️ Última sessão detectada: %s. Tentando restauração...\n", loadSessionID)
			} else {
				fmt.Println("[ACP] ℹ️ Nenhuma sessão anterior encontrada para restauração automática.")
				loadSessionID = ""
			}
		}
	}

	var sessionCreationID int
	if loadSessionID != "" {
		errLoad := e.LoadSession(session, loadSessionID)
		if errLoad == nil {
			fmt.Printf("[ACP] Sessão anterior (%s) restaurada com sucesso!\n", loadSessionID)
			// Persistir a sessão restaurada como a última utilizada
			lastSessionPath := filepath.Join(e.Workspace, ".lumaestro", "last_session.json")
			_ = os.WriteFile(lastSessionPath, []byte(fmt.Sprintf(`{"sessionId":"%s"}`, loadSessionID)), 0644)
		} else {
			fmt.Printf("[ACP] ❌ Erro ao carregar sessão anterior (%s): %v. Tentando nova sessão.\n", loadSessionID, errLoad)
			loadSessionID = ""
		}
	}

	if loadSessionID == "" {
		// 🚀 CRIAR NOVA SESSÃO: Se não houve restauração ou ela falhou
		absSandboxPath, _ := filepath.Abs(filepath.Join(e.Workspace, ".lumaestro", "sandbox"))
		_ = os.MkdirAll(absSandboxPath, 0755)
		_ = os.WriteFile(filepath.Join(absSandboxPath, "GEMINI.md"), []byte("# Sandbox\nAmbiente Isolado."), 0644)
		_ = os.WriteFile(filepath.Join(absSandboxPath, "package.json"), []byte("{\"name\": \"sandbox\"}"), 0644)

		cleanSandboxPath := strings.ReplaceAll(absSandboxPath, "\\", "\\\\")

		sessionCreationID = e.getNextID()
		e.SendRPC(session, JSONRPCMessage{
			JSONRPC: JSONRPCVersion,
			ID:      sessionCreationID,
			Method:  "session/new",
			Params:  json.RawMessage(`{"cwd":"` + cleanSandboxPath + `","mcpServers":[]}`),
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
	// Espera e engole silenciosamente se a CLI dar Internal Error para newly created sessions.
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

	// 🎼 Sinfonias: O diretório central do projeto tem prioridade total
	sinfoniaPath := filepath.Join(e.Workspace, ".lumaestro", "sinfonias", "gemini")
	sessionHomes = append(sessionHomes, sinfoniaPath)
	sessionHomes = append(sessionHomes, filepath.Join(sinfoniaPath, ".gemini"))

	sessionHomes = append(sessionHomes, filepath.Join(userHome, ".gemini"))

	cwd, _ := os.Getwd()
	if e.Workspace != "" {
		cwd = e.Workspace
	}

	if cfg, errCfg := config.Load(); errCfg == nil {
		// Se o workspace estiver vazio no config mas tivermos um ativo em memória, usa ele
		if cfg.ActiveWorkspace == "" && e.Workspace != "" {
			cfg.ActiveWorkspace = e.Workspace
		}
		
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
	// 🎯 Varredura Dinâmica de Project ID em todos os Home
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
			filepath.Join(sHome, "tmp", "sandbox", "chats"),
			filepath.Join(sHome, "sessions"),
		)
	}

	var rawList []SessionInfo
	visited := make(map[string]bool)

	fmt.Printf("[ListSessions] 🛰️ Iniciando varredura global em %d diretórios base...\n", len(sessionHomes))
	fmt.Printf("[ListSessions] 🎯 Workspace Ativo: %s | ProjectID Sugerido: %s\n", cwd, projectID)

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
				// 🛡️ DEDUPLICAÇÃO DE JUNCTIONS: Usa o nome único do arquivo (com hash)
				// em vez do caminho. Se a mesma sessão for vista pelo caminho real e pela junction, ignora.
				if visited[name] {
					continue
				}
				visited[name] = true

				foundInDir++
				path := filepath.Join(dirPath, f.Name())
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

					rawList = append(rawList, SessionInfo{
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

	// 🛡️ DEDUPLICAÇÃO AGRESSIVA: O Gemini CLI cria múltiplos checkpoints.
	// Agrupamos por SessionID (ID lógico) e mantemos apenas o arquivo mais recente.
	sessionMap := make(map[string]SessionInfo)
	for _, s := range rawList {
		if s.SessionID == "" {
			// Se não tem ID, usa o título como chave de agrupamento (fallback)
			key := "title:" + s.Title
			existing, exists := sessionMap[key]
			if !exists || s.UpdatedAt > existing.UpdatedAt {
				sessionMap[key] = s
			}
			continue
		}
		
		existing, exists := sessionMap[s.SessionID]
		if !exists || s.UpdatedAt > existing.UpdatedAt {
			sessionMap[s.SessionID] = s
		}
	}

	finalList := make([]SessionInfo, 0, len(sessionMap))
	for _, s := range sessionMap {
		finalList = append(finalList, s)
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
	// 🛡️ SEGURANÇA: Usar o workspace isolado (sandbox) para impedir que a IA
	// leia o código-fonte do próprio Lumaestro e crie ramificações no histórico.
	absSandboxPath, _ := filepath.Abs(filepath.Join(e.Workspace, ".lumaestro", "sandbox"))
	_ = os.MkdirAll(absSandboxPath, 0755)
	cleanSandboxPath := strings.ReplaceAll(absSandboxPath, "\\", "\\\\")

	params := map[string]interface{}{
		"sessionId":  acpSessionID,
		"cwd":        cleanSandboxPath,
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
	lumaestroPath := filepath.Join(cwd, ".lumaestro") // 🛡️ Autoriza o Hangar de Identidades
	sinfoniaPath := filepath.Join(e.Workspace, ".lumaestro", "sinfonias", "gemini")
	userHome, _ := os.UserHomeDir()
	globalGeminiPath := filepath.Join(userHome, ".gemini")

	// 🛡️ Validação de Segurança: Só permite deletar se o caminho for dentro de uma órbita autorizada
	authorized := false
	pathsToVerify := []string{geminiPath, lumaestroPath, sinfoniaPath, globalGeminiPath}
	for _, p := range pathsToVerify {
		if strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(p)) {
			authorized = true
			break
		}
	}

	if !authorized {
		return fmt.Errorf("tentativa de exclusão fora das órbitas autorizadas: %s", filePath)
	}

	return os.Remove(filePath)
}
