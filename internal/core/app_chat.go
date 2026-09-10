package core

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/config"
	"Lumaestro/internal/utils"
)

func (a *App) emitAgentStatus(agent string, action string, kind string) {
	if strings.TrimSpace(action) == "" {
		return
	}
	if strings.TrimSpace(kind) == "" {
		kind = "status"
	}
	a.emitEvent("agent:status", map[string]string{
		"agent":  agent,
		"action": action,
		"kind":   kind,
	})
}

// AskAgent processa a pergunta em segundo plano para permitir Streaming Real
func (a *App) AskAgent(agentName string, prompt string) string {
	fmt.Printf("[BACKEND] AskAgent chamado para: %s\n", agentName)
	a.emitAgentStatus(agentName, "Preparando orquestração do chat", "status")

	if a.chat == nil {
		a.emitAgentStatus(agentName, "Inicializando motor de chat", "status")
		fmt.Println("[App] ⚠️ Motor de Chat nulo. Tentando inicialização de emergência...")
		if err := a.initServices(); err != nil || a.chat == nil {
			return "⚠️ O motor do Maestro está desligado. Verifique sua Gemini API Key nas configurações."
		}
	}

	if agentName == "" {
		agentName = "gemini"
	}

	go func() {
		ctx := a.ctx // Ancoragem de segurança
		fmt.Printf("[BACKEND] Iniciando chamada de Chat para: %s\n", agentName)
		a.emitAgentStatus(agentName, "Orquestrando contexto e intenção do usuário", "status")

		// 🛡️ Prevenção contra contexto nulo ou cancelado
		if ctx == nil {
			return
		}

		// Usamos "default" como sessionID para manter o histórico em memória nesta sessão do app.
		response, err := a.chat.Ask(ctx, agentName, "default", prompt)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO no Chat: %v\n", err)
			a.emitEvent("agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha na Sinfonia: " + err.Error(),
			})
			return
		}

		fmt.Printf("[BACKEND] Resposta da Orquestração recebida. Injetando na sessão ACP...\n")
		a.emitAgentStatus(agentName, "Encaminhando plano para o agente ativo", "status")

		// Injeta a pergunta (prompt completo com RAG e histórico) na sessão ACP ativa
		err = a.executor.SendInput(agentName, response, nil)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO ao enviar para o agente: %v\n", err)
			a.emitEvent("agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha ao comunicar com o agente: " + err.Error(),
			})
			return
		}
		_ = ctx // Mantém referência viva
	}()

	return "Orquestrando..."
}

func (a *App) SendAgentInput(agent string, input string, images []map[string]string) error {
	// 🧭 MODEL STEERING: Detecta se o motor está ocupado e envia como dica em tempo real
	if a.executor != nil && a.executor.IsTurnPending(agent) {
		fmt.Printf("[App] 🧭 Direcionamento (Steering) detectado para %s. Enviando dica...\n", agent)
		a.emitAgentStatus(agent, "Direcionando motor em tempo real (Steering Hint)", "status")

		// Envia diretamente sem passar por RAG/Orquestrador para garantir latência zero na dica
		return a.executor.SendInput(agent, "[DIRECIONAMENTO DO USUÁRIO]: "+utils.SanitizePath(input), images)
	}

	// 🚀 Suporte a Slash Commands de Alto Raciocínio (Antigravity CLI)
	trimmedInput := strings.TrimSpace(input)
	if strings.HasPrefix(trimmedInput, "/rename ") {
		newTitle := strings.TrimSpace(strings.TrimPrefix(trimmedInput, "/rename"))
		if newTitle != "" {
			activeSessID := ""
			if a.executor != nil {
				activeSessID = a.executor.GetActiveACPSessionID(agent)
			}
			if activeSessID == "" {
				activeSessID = agent
			}
			_ = a.RenameSession(activeSessID, newTitle)
			a.emitAgentStatus(agent, fmt.Sprintf("✏️ Sessão renomeada para: '%s'", newTitle), "status")
			return nil
		}
	} else if trimmedInput == "/fork" || strings.HasPrefix(trimmedInput, "/fork ") {
		activeSessID := ""
		if a.executor != nil {
			activeSessID = a.executor.GetActiveACPSessionID(agent)
		}
		newID, errFork := a.ForkSession(agent, activeSessID)
		if errFork == nil {
			prefixLen := 8
			if len(newID) < prefixLen {
				prefixLen = len(newID)
			}
			a.emitAgentStatus(agent, fmt.Sprintf("🌿 Sessão ramificada: %s", newID[:prefixLen]), "status")
			return nil
		}
	} else if strings.HasPrefix(trimmedInput, "/boost ") || trimmedInput == "/boost" {
		task := strings.TrimSpace(strings.TrimPrefix(trimmedInput, "/boost"))
		a.emitAgentStatus(agent, "🚀 Boost: Ativando pipeline de raciocínio profundo de 3 fases...", "status")
		input = fmt.Sprintf("[PIPELINE DE RACIOCÍNIO PROFUNDO — BOOST ATIVO]\nSua missão é resolver o desafio a seguir utilizando raciocínio multi-etapas com verificação independente:\n\nFASE 1: FORMULAÇÃO E ESTRATÉGIA\n- Inspecione o contexto do workspace, determine a causa raiz e elabore uma estratégia executável.\n\nFASE 2: EXECUÇÃO PARALELA E VERIFICAÇÃO LOCAL\n- Construa a solução, aplique os refatoramentos necessários e execute os testes locais para validar as hipóteses.\n\nFASE 3: SÍNTESE E ENTREGA VERIFICADA\n- Valide que todos os testes passaram e entregue um resumo conciso com as mudanças validadas.\n\nTAREFA: %s", task)
	} else if strings.HasPrefix(trimmedInput, "/teamwork-preview ") || strings.HasPrefix(trimmedInput, "/teamwork ") || trimmedInput == "/teamwork" || trimmedInput == "/teamwork-preview" {
		task := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(trimmedInput, "/teamwork-preview"), "/teamwork"))
		a.emitAgentStatus(agent, "👥 Teamwork: Orquestrando equipe colaborativa com portões de verificação...", "status")
		input = fmt.Sprintf("[EQUIPE COLABORATIVA — TEAMWORK ATIVO]\nVocê está operando como o Sentinel e Project Orchestrator de uma equipe multi-agente:\n- Sentinel: Coordena a execução, roteia tarefas e posta atualizações de progresso.\n- Project Orchestrator: Divide o escopo em marcos claros, delega unidades de trabalho e previne degradação de contexto.\n- Explorers: Investigam o repositório sem modificar arquivos.\n- Workers: Constroem e refatoram código em faixas não sobrepostas.\n- Portões de Verificação (Critic, Challenger, Auditor e Success Auditor): Validam a integridade e realizam stress-testing antes de concluir cada marco.\n\nPROJETO / OBJETIVO: %s", task)
	} else if strings.HasPrefix(trimmedInput, "/") {
		parts := strings.SplitN(trimmedInput, " ", 2)
		cmdName := strings.TrimPrefix(parts[0], "/")
		userArgs := ""
		if len(parts) > 1 {
			userArgs = strings.TrimSpace(parts[1])
		}

		ws := a.getActiveWorkspace()
		// 1. Verifica se corresponde a uma Habilidade Dinâmica (Skill)
		matchedSkill := false
		if skillsList, err := acp.DiscoverSkills(ws); err == nil {
			for _, sk := range skillsList {
				if strings.EqualFold(sk.Name, cmdName) {
					matchedSkill = true
					a.emitAgentStatus(agent, fmt.Sprintf("🛠️ Habilidade: Ativando '%s'...", sk.Name), "status")
					if userArgs == "" {
						userArgs = "Aplique os procedimentos, ferramentas e diretrizes desta habilidade ao projeto."
					}
					input = fmt.Sprintf("[HABILIDADE ATIVA: %s]\n%s\n\nDIRETRIZES DA HABILIDADE:\n%s\n\nSOLICITAÇÃO DO USUÁRIO:\n%s",
						sk.Name, sk.Description, sk.Content, userArgs)
					break
				}
			}
		}

		// 2. Se não foi skill, verifica se corresponde a um Custom Agent
		if !matchedSkill {
			if customAgents, err := acp.DiscoverCustomAgents(ws); err == nil {
				for _, ca := range customAgents {
					if strings.EqualFold(ca.Name, cmdName) {
						a.emitAgentStatus(agent, fmt.Sprintf("🤖 Agente Customizado: Ativando '%s'...", ca.Name), "status")
						if userArgs == "" {
							userArgs = "Atue conforme suas instruções e diretrizes de persona no workspace atual."
						}
						input = fmt.Sprintf("[AGENTE CUSTOMIZADO: %s]\n%s\n\nINSTRUÇÕES DO AGENTE:\n%s\n\nSOLICITAÇÃO DO USUÁRIO:\n%s",
							ca.Name, ca.Description, ca.Prompt, userArgs)
						break
					}
				}
			}
		}
	}

	// ⚡ Log Premium e Limpo
	previewInput := input
	if len(previewInput) > 60 {
		previewInput = previewInput[:57] + "..."
	}
	fmt.Printf("[App] 📡 Sincronizando Mensagem >> Motor: %s | Preview: '%s'\n", agent, previewInput)
	a.emitAgentStatus(agent, "Preparando contexto da conversa", "status")

	// ⚡ Idioma Dinâmico
	lang := a.GetConfig().AgentLanguage
	if lang == "" {
		lang = "Português do Brasil"
	}

	// 🧠 Injetor de Memória Semântica com Ligações Nervosas (RAG + Grafo)
	contextInfo := a.buildHybridContext(agent, input, previewInput)

	// 🧠 Orquestração Soberana: Decide o Agente e monta o Prompt Contextual (RAG + Skills)
	a.emitAgentStatus(agent, "Definindo estratégia e montando prompt final", "status")
	
	// 🛡️ SEGURANÇA: Sanitizar caminhos reais do contexto e da pergunta antes da orquestração
	safeContext := utils.SanitizePath(contextInfo)
	safeInput := utils.SanitizePath(input)
	
	agentName, finalPrompt, profile, err := a.orchestrator.Execute(a.ctx, "default", safeInput, safeContext)
	if err != nil {
		fmt.Printf("[App] ERRO na Orquestração: %v\n", err)
		return fmt.Errorf("falha ao orquestrar sinfonia: %v", err)
	}

	// Se o usuário escolheu explicitamente um motor no chat, respeita a escolha.
	forcedAgent := strings.ToLower(strings.TrimSpace(agent))
	if forcedAgent == "gemini" || forcedAgent == "antigravity" || forcedAgent == "agy" || forcedAgent == "claude" || forcedAgent == "lmstudio" {
		agentName = forcedAgent
	}

	// 💎 Nomenclatura Oficial: Se for o motor do Google/Antigravity CLI (agy.exe), exibe 'Antigravity'
	displayEngine := "Antigravity"
	if forcedAgent == "lmstudio" {
		displayEngine = "LM Studio"
	} else if forcedAgent == "claude" {
		displayEngine = "Claude"
	}

	profileName := profile.Name
	if forcedAgent == "gemini" || forcedAgent == "antigravity" || forcedAgent == "agy" {
		profileName = "Antigravity"
	} else if forcedAgent == "claude" {
		profileName = "Claude"
	} else if forcedAgent == "lmstudio" {
		profileName = "LM Studio"
	}

	// 📡 Identidade Visual: Avisa o Frontend qual Perfil assumiu a palavra
	fmt.Printf("[App] 🎭 Identidade Visual EMITIDA: %s (%s)\n", profileName, displayEngine)

	a.emitEvent("agent:profile", map[string]string{
		"name":   profileName,
		"engine": displayEngine,
	})
	a.emitAgentStatus(displayEngine, "Perfil ativo definido. Preparando execução", "status")

	// 🚀 Disparo ACP via Protocolo ndJSON: Garante que o motor está online (Auto-Start apenas se offline)
	a.executor.Mu.Lock()
	_, isOnline := a.executor.ActiveSessions[agentName]
	a.executor.Mu.Unlock()

	if !isOnline {
		a.emitAgentStatus(displayEngine, "Iniciando motor '"+displayEngine+"'...", "status")
		if err := a.StartAgentSession(agentName); err != nil {
			fmt.Printf("[App] ERRO ao iniciar sessão ACP do motor %s: %v\n", agentName, err)
			return fmt.Errorf("erro ao iniciar motor %s em modo ACP: %v", agentName, err)
		}
	}

	a.emitAgentStatus(displayEngine, "Enviando instruções para o agente", "status")
	err = a.executor.SendInput(agentName, finalPrompt, images)
	if err != nil {
		fmt.Printf("[App] ERRO no SendAgentInput: %v\n", err)
		return fmt.Errorf("erro ao enviar input para ACP: %v", err)
	}

	fmt.Printf("[App] ✅ Sinfonia roteada para %s (%s) com sucesso via streaming NDJSON!\n", agent, displayEngine)

	// ✨ Auto-Naming de Sinfonia: Se a sessão atual ainda não tem título, a IA a batiza em background
	go func(targetAgent string, userMsg string) {
		time.Sleep(3 * time.Second)
		if a.executor == nil {
			return
		}
		acpSessID := a.executor.GetActiveACPSessionID(targetAgent)
		if acpSessID != "" {
			titles := acp.LoadSessionTitles()
			if _, hasTitle := titles[acpSessID]; !hasTitle {
				_, _ = a.AutoNameSession(acpSessID)
			}
		}
	}(agentName, input)

	// 📡 Feedback Imediato: Reseta o timer do frontend e avisa que o processamento começou
	a.emitEvent("agent:log", map[string]string{
		"source":  "SYSTEM",
		"content": "🧠 Maestro processando sinapses e raciocinando...",
		"type":    "progress",
	})

	return nil
}

// buildHybridContext constrói o contexto de inteligência mesclando Busca Lexical, Semântica e Memórias em um pipeline unificado.
func (a *App) buildHybridContext(agent string, input string, previewInput string) string {
	contextInfo := ""

	targetWs := a.getActiveWorkspace()
	if targetWs == "" {
		targetWs = a.GetConfig().ObsidianVaultPath
	}

	// 🔒 Validação de Resiliência: Precisa do navegador e de um workspace ou vault ativo.
	if a.navigator == nil || targetWs == "" {
		return contextInfo
	}

	a.emitAgentStatus(agent, "Consultando memória e contexto do projeto", "memory")
	fmt.Println("[RAG] Explorando ligações nervosas no Grafo de Conhecimento...")

	// 📡 1. ENGINE LEXICAL: Radar de Palavra-Chave (Prioridade Máxima de UX e Relevância Exata)
	nodes := a.navigator.SearchByKeyword(a.ctx, input)
	foundByRadar := len(nodes) > 0
	discoveryEmitted := ""

	// 🎬 FAST-TRACK ZOOM
	if foundByRadar {
		if id, ok := nodes[0]["id"].(string); ok {
			discoveryEmitted = id
			fmt.Printf("[RAG] 🎬 [FAST-TRACK] Radar identificou alvo: \"%s\". Disparando Zoom!\n", id)
			a.emitAgentStatus(agent, "Alvo identificado pelo Radar Neural! Iniciando Voo...", "memory")
			utils.SafeEmit(a.ctx, "node:active", discoveryEmitted)
		}
	} else {
		fmt.Printf("[RAG] 🔍 Radar Neural não encontrou correspondências diretas para: \"%s\"\n", previewInput)
	}

	// 🧠 2. ENGINE SEMÂNTICO (IA): Enriquecimento Vectorial e Memórias
	if a.embedder != nil && a.qdrant != nil {
		vector, err := a.embedder.GenerateEmbedding(a.ctx, input, true)
		if err == nil {
			a.emitAgentStatus(agent, "Buscando referências semânticas relevantes", "memory")

			semanticNotes, _ := a.qdrant.Search("obsidian_knowledge", vector, 5) // Busca expandida (Top 5)
			semanticMems, _ := a.qdrant.Search("knowledge_graph", vector, 3)     // Memórias recentes (Top 3)

			seen := make(map[string]bool)
			for _, n := range nodes {
				if name, ok := n["name"].(string); ok {
					seen[name] = true
				}
			}

			// 🛡️ RANKING DE MERGE E LIMITE DE NÓS (Prevenção de Context Overflow)
			const MAX_NODES = 12 // Teto absoluto de nós injetados no prompt

			for _, sn := range semanticNotes {
				if len(nodes) >= MAX_NODES {
					break
				}
				if name, ok := sn["name"].(string); ok && !seen[name] {
					nodes = append(nodes, sn)
					seen[name] = true
				}
			}

			for _, sm := range semanticMems {
				if len(nodes) >= MAX_NODES {
					break
				}
				if subj, ok := sm["subject"].(string); ok && !seen[subj] {
					sm["name"] = subj
					sm["document-type"] = "memory"
					nodes = append(nodes, sm)
					seen[subj] = true
				}
			}
		} else {
			fmt.Printf("[RAG] ⚠️ Falha na API de Vetores (%v). Operando apenas no modo Degredado (Radar).\n", err)
		}
	}

	// 🎬 3. MONTAGEM FINAL DO CONTEXTO
	if len(nodes) > 0 {
		statusMsg := "Expandindo contexto com memória conectada"
		if foundByRadar {
			statusMsg = "Alvo identificado pelo Radar Neural!"
		}

		// Fallback semântico para zoom (se o radar falhou, voamos para o Top-1 semântico)
		if discoveryEmitted == "" {
			if topID, ok := nodes[0]["id"].(string); ok {
				fmt.Printf("[RAG] 🎬 Zoom semântico (fallback): %s\n", topID)
				utils.SafeEmit(a.ctx, "node:active", topID)
			}
		}

		a.emitAgentStatus(agent, statusMsg, "memory")

		// O navigator expande a estrutura com sinapses conectadas
		fullContext := a.navigator.ExpandContext(a.ctx, nodes)

		contextInfo = "\n\n[CONHECIMENTO ORQUESTRADO (OBSIDIAN + SINAPSES)]\n"

		// 🛡️ Limite Absoluto de Caracteres (Proteção de Custos API LLM)
		const MAX_CHARS = 100000
		for _, ctxPart := range fullContext {
			if len(contextInfo)+len(ctxPart) > MAX_CHARS {
				contextInfo += "\n\n[⚠️ CONTEÚDO ADICIONAL TRUNCADO POR LIMITE DE MEMÓRIA (Max Tokens Atingido)]"
				break
			}
			contextInfo += ctxPart + "\n\n"
		}
		fmt.Printf("[RAG] 🧠 Contexto Híbrido Finalizado: %d Fontes Base -> Tamanho total: %d caracteres.\n", len(nodes), len(contextInfo))
	}

	return contextInfo
}

// ConsolidateChatKnowledge analisa o diálogo recente e cria ligações nervosas (sinapses).
func (a *App) ConsolidateChatKnowledge(sessionID string, chatText string) string {
	if a.weaver == nil {
		return "⚠️ Motor de memórias não inicializado."
	}

	fmt.Printf("[App] Consolidando ligações nervosas para sessão %s...\n", sessionID)
	a.emitAgentStatus("memory", "Consolidando memória da conversa no grafo", "memory")
	err := a.weaver.WeaveChatKnowledge(a.ctx, sessionID, chatText)
	if err != nil {
		return "Erro ao tecer sinapses: " + err.Error()
	}

	_ = os.Remove(filepath.Join(".lumaestro", "cache", "topology.json"))
	a.emitAgentStatus("memory", "Memória consolidada e mapa 3D atualizado", "memory")

	return "✅ Sinapses consolidadas com sucesso no Grafo de Conhecimento."
}

// SetAgentModel altera o modelo de um agente e persiste a configuração.
func (a *App) SetAgentModel(agent string, model string) error {
	fmt.Printf("[App] ⚙️ Iniciando troca de modelo do motor %s para: %s\n", agent, model)

	cfg, _ := config.Load()
	if agent == "gemini" || agent == "antigravity" || agent == "agy" {
		cfg.GeminiModel = model
	}
	err := config.Save(*cfg)
	if err != nil {
		return err
	}

	a.config = cfg

	// 🚀 Tenta troca via RPC Dinâmico (Zero-Restart)
	if a.executor != nil {
		if _, ok := a.executor.ActiveSessions[agent]; ok {
			fmt.Printf("[App] ⚡ Tentando troca dinâmica via RPC (unstable_setSessionModel)...\n")
			errRPC := a.executor.SetSessionModel(agent, model)

			if errRPC == nil {
				fmt.Printf("[App] ✅ Troca dinâmica concluída com sucesso para %s!\n", model)
				a.emitEvent("agent:log", map[string]string{
					"source":  "SYSTEM",
					"content": "⚡ Modelo alterado dinamicamente para: " + model,
				})
				return nil
			}

			// Se falhou (ex: método não suportado no binário atual), fazemos o fallback para Reinício
			fmt.Printf("[App] ⚠️ Falha na troca dinâmica (%v). Fazendo fallback para reinício do motor...\n", errRPC)
			if s, ok := a.executor.ActiveSessions[agent]; ok {
				if s.Cancel != nil {
					s.Cancel()
				}
				delete(a.executor.ActiveSessions, agent)

				a.emitEvent("agent:log", map[string]string{
					"source":  "SYSTEM",
					"content": "🔄 Reiniciando motor para aplicar novo modelo: " + model,
				})
			}
		}
	}

	return nil
}

// ResolveConflict executa a decisão do usuário sobre uma contradição semântica detectada.
func (a *App) ResolveConflict(decision string, subject string, predicate string, oldID uint64, newValue string, sessionID string) string {
	if decision == "new" {
		// 1. Marcar o antigo como LEGADO
		a.qdrant.SetPayload("knowledge_graph", oldID, map[string]interface{}{
			"status":      "legacy",
			"archived_at": time.Now().Format(time.RFC3339),
		})

		// 2. Salvar o NOVO como ativo
		factText := fmt.Sprintf("%s %s %s", subject, predicate, newValue)
		vector, _ := a.crawler.Embedder.GenerateEmbedding(a.ctx, factText, false)

		h := fnv.New64a()
		h.Write([]byte(factText + sessionID))
		newID := h.Sum64()

		payload := map[string]interface{}{
			"id":         newID,
			"session_id": sessionID,
			"subject":    subject,
			"predicate":  predicate,
			"object":     newValue,
			"source":     "chat_memory",
			"status":     "active",
			"timestamp":  time.Now().Format(time.RFC3339),
			"content":    factText,
		}

		a.qdrant.UpsertPoint("knowledge_graph", newID, vector, payload)

		a.emitEvent("agent:log", map[string]string{
			"source":  "RESOLVER",
			"content": fmt.Sprintf("✅ Conflito resolvido: '%s' agora é a verdade sobre '%s'.", newValue, subject),
		})
	} else {
		a.emitEvent("agent:log", map[string]string{
			"source":  "RESOLVER",
			"content": "🗺️ Conflito resolvido: Mantida a informação histórica para '" + subject + "'.",
		})
	}

	return "Conflito resolvido."
}

// SendTerminalData envia input do usuário para o processo do terminal (stdin).
func (a *App) SendTerminalData(agent string, data string) {
	sessionID := "acp-session-" + agent
	a.executor.SendInput(sessionID, data, nil)
}

// SendSteeringHint envia uma dica de direcionamento em tempo real para a sessão ativa do agente.
func (a *App) SendSteeringHint(agent string, input string) string {
	fmt.Printf("[App] ⚡ Enviando Steering Hint para %s: '%s'\n", agent, input)

	if a.executor == nil {
		return "Executor não inicializado"
	}

	// No Lumaestro, a sessão ACP de chat principal usa o nome do agente como ID.
	sessionID := agent
	err := a.executor.SendSteeringHint(sessionID, input)
	if err != nil {
		return "Erro ao enviar direcionamento: " + err.Error()
	}
	return "Dica enviada!"
}

// SetExecutionMode define o modo de autonomia: "default", "accept-edits" ou "plan"
func (a *App) SetExecutionMode(agent string, mode string) bool {
	validModes := map[string]bool{
		"default":      true,
		"accept-edits": true,
		"plan":         true,
	}
	if !validModes[mode] {
		mode = "default"
	}

	if a.executor == nil {
		return false
	}

	a.executor.ExecutionMode = mode

	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[agent]
	a.executor.Mu.Unlock()

	if ok {
		prevMode := session.ExecutionMode
		session.ExecutionMode = mode
		session.PlanMode = (mode == "plan")
		fmt.Printf("[App] 🛡️ Modo de Execução alterado para '%s' na sessão %s\n", mode, agent)

		// Se for sessão Antigravity e o modo mudou, reinicia o processo com a nova flag
		if session.IsAntigravity && prevMode != mode {
			go func() {
				_ = a.executor.StartSession(a.ctx, session.AgentName, session.ID, session.ACPSessID, session.AgentID, session.CurrentIssueID, session.PlanMode, nil)
			}()
		}

		statusLabel := map[string]string{
			"default":      "Modo Padrão (Interativo) [default]",
			"accept-edits": "Modo Edição Automática [accept-edits]",
			"plan":         "Modo Planejamento (Leitura) [plan]",
		}[mode]

		a.emitAgentStatus(agent, statusLabel, "status")
		utils.SafeEmit(a.ctx, "mode:changed", map[string]string{
			"agent": agent,
			"mode":  mode,
		})
		return true
	}
	return false
}

// GetExecutionMode retorna o modo de autonomia ativo para a sessão.
func (a *App) GetExecutionMode(agent string) string {
	if a.executor == nil {
		return "default"
	}
	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[agent]
	a.executor.Mu.Unlock()

	if ok && session.ExecutionMode != "" {
		return session.ExecutionMode
	}
	if a.executor.ExecutionMode != "" {
		return a.executor.ExecutionMode
	}
	return "default"
}

// SetPlanMode ativa ou desativa o modo de planejamento para a sessão (compatibilidade legado).
func (a *App) SetPlanMode(agent string, enabled bool) bool {
	if enabled {
		return a.SetExecutionMode(agent, "plan")
	}
	return a.SetExecutionMode(agent, "accept-edits")
}

// GetPlanMode retorna o estado atual do Plan Mode para a sessão (compatibilidade legado).
func (a *App) GetPlanMode(agent string) bool {
	return a.GetExecutionMode(agent) == "plan"
}

// ReadGeminiConfig lê o conteúdo do arquivo GEMINI.md na raiz do projeto.
func (a *App) ReadGeminiConfig() (string, error) {
	data, err := os.ReadFile("GEMINI.md")
	if err != nil {
		if os.IsNotExist(err) {
			return "# Diretrizes do Gemini\n\nAdicione suas instruções globais aqui...", nil
		}
		return "", err
	}
	return string(data), nil
}

// WriteGeminiConfig salva as novas diretrizes no arquivo GEMINI.md.
func (a *App) WriteGeminiConfig(content string) error {
	return os.WriteFile("GEMINI.md", []byte(content), 0644)
}

// LogNeuralActivity permite que o Frontend reporte eventos de navegação para o terminal de processamento.
func (a *App) LogNeuralActivity(source string, content string, isError bool) {
	logType := "status"
	if isError {
		logType = "error"
	}
	a.emitEvent("agent:log", map[string]string{
		"source":  strings.ToUpper(source),
		"content": content,
		"type":    logType,
	})
}

// TriggerZoom permite que o Frontend solicite um foco de câmera manualmente (ex: Zoom Cinematográfico via IA)
func (a *App) TriggerZoom(nodeID string) {
	cleanID := strings.ToLower(strings.TrimSpace(nodeID))
	if cleanID != "" {
		fmt.Printf("[App] 🎬 Zoom Cinematográfico via IA disparado para: %s\n", cleanID)
		a.emitEvent("node:active", cleanID)
	}
}
