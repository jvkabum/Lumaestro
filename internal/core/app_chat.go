package core

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

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

		// 🧠 [SOBERANIA] Obtém o workspace ativo e as referências para isolamento inteligente
		activeWorkspace := a.executor.Workspace
		allowedPaths := []string{}
		agentCWD := ""

		if activeWorkspace != "" {
			allowedPaths = append(allowedPaths, activeWorkspace)
			agentCWD = activeWorkspace // No projeto atual, ele nasce na raiz do projeto
		} else {
			// 🕶️ MODO CEGO: Cria uma câmara de vácuo (pasta temporária vazia)
			tempDir, err := os.MkdirTemp("", "lumaestro_vacuum_*")
			if err == nil {
				agentCWD = tempDir
				fmt.Printf("[SOBERANIA] 🕶️ Câmara de Vácuo criada em: %s\n", agentCWD)
			}
		}
		
		// 📡 Adiciona projetos externos como Órbitas de Referência
		if a.config != nil {
			for _, p := range a.config.ExternalProjects {
				pAbs, _ := filepath.Abs(p.Path)
				if pAbs != "" && pAbs != activeWorkspace {
					allowedPaths = append(allowedPaths, pAbs)
				}
			}
		}

		// Usamos "default" como sessionID para manter o histórico em memória
		response, err := a.chat.Ask(ctx, agentName, "default", prompt, allowedPaths)
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

		// Injeta a pergunta na sessão ACP ativa, garantindo o CWD isolado se necessário
		err = a.executor.SendInput(agentName, response, nil, agentCWD)
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
		return a.executor.SendInput(agent, "[DIRECIONAMENTO DO USUÁRIO]: "+input, images, "")
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
	agentName, finalPrompt, profile, err := a.orchestrator.Execute(a.ctx, "default", input, contextInfo)
	if err != nil {
		fmt.Printf("[App] ERRO na Orquestração: %v\n", err)
		return fmt.Errorf("falha ao orquestrar sinfonia: %v", err)
	}

	// Se o usuário escolheu explicitamente um motor no chat, respeita a escolha.
	forcedAgent := strings.ToLower(strings.TrimSpace(agent))
	if forcedAgent == "gemini" || forcedAgent == "claude" || forcedAgent == "lmstudio" {
		agentName = forcedAgent
	}

	// 📡 Identidade Visual: Avisa o Frontend qual Perfil assumiu a palavra
	fmt.Printf("[App] 🎭 Identidade Visual EMITIDA: %s (%s)\n", profile.Name, agentName)
	profileName := profile.Name
	if forcedAgent == "lmstudio" {
		profileName = "LM Studio"
	} else if forcedAgent == "claude" {
		profileName = "Claude"
	} else if forcedAgent == "gemini" {
		profileName = "Gemini"
	}

	a.emitEvent("agent:profile", map[string]string{
		"name":   profileName,
		"engine": agentName,
	})
	a.emitAgentStatus(agentName, "Perfil ativo definido. Preparando execução", "status")

	// 🚀 Disparo ACP via Protocolo ndJSON: Garante que o motor está online (Auto-Start)
	a.emitAgentStatus(agentName, "Garantindo que o motor '"+agentName+"' está online", "status")
	if err := a.StartAgentSession(agentName); err != nil {
		fmt.Printf("[App] ERRO ao iniciar sessão ACP do motor %s: %v\n", agentName, err)
		return fmt.Errorf("erro ao iniciar motor %s em modo ACP: %v", agentName, err)
	}

	a.emitAgentStatus(agentName, "Enviando instruções para o agente", "status")
	err = a.executor.SendInput(agentName, finalPrompt, images, "")
	if err != nil {
		fmt.Printf("[App] ERRO no SendAgentInput: %v\n", err)
		return fmt.Errorf("erro ao enviar input para ACP: %v", err)
	}

	fmt.Printf("[App] ✅ Sinfonia roteada para %s com sucesso via JSON-RPC!\n", agent)

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
	cfg := a.GetConfig()
	activeWorkspace := cfg.ActiveWorkspace

	// 🔒 [SOBERANIA DO COMANDANTE] Strict Context Isolation
	// Se nenhuma órbita ativa estiver selecionada, a IA deve ser cega para o sistema de arquivos.
	if activeWorkspace == "" {
		fmt.Println("[RAG] 🕶️ MODO CEGO: Nenhuma órbita ativa selecionada. IA operando sem contexto de arquivos.")
		return contextInfo
	}

	// 📡 Monta a lista de Órbitas Autorizadas (Ativa + Referências)
	allowedPaths := []string{activeWorkspace}
	if a.config != nil {
		for _, p := range a.config.ExternalProjects {
			pAbs, _ := filepath.Abs(p.Path)
			if pAbs != activeWorkspace && pAbs != "" {
				allowedPaths = append(allowedPaths, pAbs)
			}
		}
	}

	// 🔒 Validação de Resiliência: Só executa o RAG se o Vault estiver mapeado e os motores estiverem online.
	if a.embedder == nil || a.navigator == nil {
		return contextInfo
	}

	a.emitAgentStatus(agent, "Consultando memória e contexto da órbita ativa", "memory")
	fmt.Printf("[RAG] 🌌 Explorando órbita ativa: %s\n", activeWorkspace)

	// 📡 1. ENGINE LEXICAL: Radar de Palavra-Chave (Prioridade Máxima)
	// Agora passamos as órbitas autorizadas para filtrar internamente via DuckDB (Ativa + Referências)
	rawNodes := a.navigator.SearchByKeyword(a.ctx, input, allowedPaths)
	
	// Filtro de Segurança: Garante que apenas nós das órbitas autorizadas cheguem à IA
	var nodes []map[string]interface{}
	for _, n := range rawNodes {
		nodePath, _ := n["workspace_path"].(string)
		
		found := false
		if nodePath == "" { // Memórias/Sinapses são globais por enquanto
			found = true
		} else {
			for _, p := range allowedPaths {
				if strings.HasPrefix(nodePath, p) {
					found = true
					break
				}
			}
		}

		if !found {
			continue
		}
		nodes = append(nodes, n)
	}

	foundByRadar := len(nodes) > 0
	discoveryEmitted := ""

	// 🎬 FAST-TRACK ZOOM (Apenas se o alvo for legítimo da órbita)
	if foundByRadar {
		if id, ok := nodes[0]["id"].(string); ok {
			discoveryEmitted = id
			fmt.Printf("[RAG] 🎬 [FAST-TRACK] Radar identificou alvo na órbita: \"%s\".\n", id)
			a.emitAgentStatus(agent, "Alvo identificado na órbita ativa! Focando...", "memory")
			utils.SafeEmit(a.ctx, "node:active", discoveryEmitted)
		}
	}

	// 🧠 2. ENGINE SEMÂNTICO (IA): Enriquecimento Vectorial e Memórias
	vector, err := a.embedder.GenerateEmbedding(a.ctx, input, true)
	if err == nil {
		a.emitAgentStatus(agent, "Buscando referências semânticas na órbita", "memory")

		// Busca expandida no Qdrant (Top 10 para ter margem de filtro)
		semanticNotes, _ := a.qdrant.Search("obsidian_knowledge", vector, 10)
		semanticMems, _ := a.qdrant.Search("knowledge_graph", vector, 3)

		seen := make(map[string]bool)
		for _, n := range nodes {
			if name, ok := n["name"].(string); ok {
				seen[name] = true
			}
		}

		// 🛡️ RANKING DE MERGE E FILTRO DE ÓRBITA
		const MAX_NODES = 12

		for _, sn := range semanticNotes {
			if len(nodes) >= MAX_NODES {
				break
			}
			nodePath, _ := sn["path"].(string)
			
			// FILTRO DE SOBERANIA MULTI-ÓRBITA
			found := false
			for _, p := range allowedPaths {
				if nodePath != "" && strings.HasPrefix(nodePath, p) {
					found = true
					break
				}
			}
			if !found {
				continue
			}

			if name, ok := sn["name"].(string); ok && !seen[name] {
				nodes = append(nodes, sn)
				seen[name] = true
			}
		}

		// Memórias (Sinapses) — O Comandante quer limitar ao que está na órbita.
		// Atualmente as memórias não guardam o path do projeto. 
		// [TODO] Vincular memórias ao workspace_id no futuro.
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
	}

	// 🎬 3. MONTAGEM FINAL DO CONTEXTO
	if len(nodes) > 0 {
		statusMsg := "Expandindo contexto da órbita selecionada"
		if foundByRadar {
			statusMsg = "Alvo identificado pelo Radar Neural!"
		}

		// Fallback semântico para zoom
		if discoveryEmitted == "" {
			if topID, ok := nodes[0]["id"].(string); ok {
				utils.SafeEmit(a.ctx, "node:active", topID)
			}
		}

		a.emitAgentStatus(agent, statusMsg, "memory")

		// O navigator expande a estrutura com sinapses conectadas (Ativa + Referências)
		fullContext := a.navigator.ExpandContext(a.ctx, nodes, allowedPaths)

		contextInfo = "\n\n[CONHECIMENTO DA ÓRBITA ATIVA: " + filepath.Base(activeWorkspace) + "]\n"

		const MAX_CHARS = 100000
		for _, ctxPart := range fullContext {
			if len(contextInfo)+len(ctxPart) > MAX_CHARS {
				contextInfo += "\n\n[⚠️ CONTEÚDO ADICIONAL TRUNCADO POR LIMITE DE MEMÓRIA]"
				break
			}
			contextInfo += ctxPart + "\n\n"
		}
		fmt.Printf("[RAG] 🧠 Contexto de Órbita Finalizado: %d Fontes -> %d caracteres.\n", len(nodes), len(contextInfo))
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
	
	activeWorkspace := a.executor.Workspace
	err := a.weaver.WeaveChatKnowledge(a.ctx, sessionID, chatText, activeWorkspace)
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
	if agent == "gemini" {
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
			"id":             newID,
			"session_id":     sessionID,
			"workspace_path": a.executor.Workspace,
			"subject":        subject,
			"predicate":      predicate,
			"object":         newValue,
			"source":         "chat_memory",
			"status":         "active",
			"timestamp":      time.Now().Format(time.RFC3339),
			"content":        factText,
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
	a.executor.SendInput(sessionID, data, nil, "")
}

// SendSteeringHint envia uma dica de direcionamento em tempo real para a sessão ativa do agente.
func (a *App) SendSteeringHint(agent string, input string) string {
	fmt.Printf("[App] ⚡ Enviando Steering Hint para %s: '%s'\n", agent, input)

	// No Lumaestro, a sessão ACP de chat principal usa o nome do agente como ID.
	sessionID := agent
	err := a.executor.SendSteeringHint(sessionID, input)
	if err != nil {
		return "Erro ao enviar direcionamento: " + err.Error()
	}
	return "Dica enviada!"
}

// SetPlanMode ativa ou desativa o modo de planejamento para a sessão.
func (a *App) SetPlanMode(agent string, enabled bool) bool {
	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[agent]
	a.executor.Mu.Unlock()

	if ok {
		session.PlanMode = enabled
		fmt.Printf("[App] 🛡️ Plan Mode alterado para %v na sessão %s\n", enabled, agent)

		status := "Modo Execução (⚡) ativado"
		if enabled {
			status = "Modo Plano (📝) ativado — Escrita bloqueada"
		}
		a.emitAgentStatus(agent, status, "status")
		return true
	}
	return false
}

// GetPlanMode retorna o estado atual do Plan Mode para a sessão.
func (a *App) GetPlanMode(agent string) bool {
	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[agent]
	a.executor.Mu.Unlock()

	if ok {
		return session.PlanMode
	}
	return false
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
