package core

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"time"

	"Lumaestro/internal/config"
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
		if ctx == nil { return }

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
		return a.executor.SendInput(agent, "[DIRECIONAMENTO DO USUÁRIO]: "+input, images)
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
	contextInfo := ""
	if a.embedder != nil && a.navigator != nil && a.config.ObsidianVaultPath != "" {
		a.emitAgentStatus(agent, "Consultando memória e contexto do projeto", "memory")
		// 🏁 FAST-TRACK: Se o pool estiver em hibernação forçada, pulamos o RAG imediatamente
		// para não travar o envio da mensagem por 30s.
		fmt.Println("[RAG] Explorando ligações nervosas no Grafo de Conhecimento...")
		
		var fastNodeFound bool
		// 🏁 FAST-PATH: DuckDB (Busca Léxica Instantânea por Nome)
		if a.LStore != nil {
			fastNodeId, err := a.LStore.FindNodeInText(a.executor.Workspace, input)
			if err == nil && fastNodeId != "" {
				fmt.Printf("[RAG] ⚡ Fast-Path DuckDB: Match no texto -> Focando %s\n", fastNodeId)
				a.emitEvent("node:active", fastNodeId)
				fastNodeFound = true
			}
		}

		vector, err := a.embedder.GenerateEmbedding(a.ctx, input, true)
		if err == nil {
			a.emitAgentStatus(agent, "Buscando referências semânticas relevantes", "memory")
			nodes, err := a.qdrant.Search("obsidian_knowledge", vector, 3)
			if err == nil && len(nodes) > 0 {
				var finalNodeId string
				
				// Tenta ID primeiro, depois Nome (como fallback seguro)
				rawId := nodes[0]["id"]
				if rawId == nil {
					rawId = nodes[0]["name"] // Se o ID não está no payload, o Nome geralmente serve como ID no Grafo
				}

				switch v := rawId.(type) {
				case string:
					finalNodeId = v
				case float64:
					finalNodeId = fmt.Sprintf("%.0f", v)
				case int, int64:
					finalNodeId = fmt.Sprintf("%v", v)
				}

				if finalNodeId != "" && !fastNodeFound {
					fmt.Printf("[RAG] 🎯 Focando no nó semântico: %s\n", finalNodeId)
					a.emitEvent("node:active", finalNodeId)
				}

				a.emitAgentStatus(agent, "Expandindo contexto com memória conectada", "memory")
				// 2. Navegação de Sinapses: Expandir o contexto seguindo os links neurais
				fullContext := a.navigator.ExpandContext(a.ctx, nodes)
				contextInfo = "\n\n[CONHECIMENTO ORQUESTRADO (OBSIDIAN + SINAPSES)]\n"
				maxContextChars := 3000000 // Limite seguro para ~800k tokens do Gemini
				for _, ctxPart := range fullContext {
					if len(contextInfo)+len(ctxPart) > maxContextChars {
						contextInfo += "\n\n[⚠️ CONTEÚDO ADICIONAL TRUNCADO POR LIMITE DE MEMÓRIA SEMÂNTICA]"
						break
					}
					contextInfo += ctxPart + "\n\n"
				}
				fmt.Printf("[RAG] Contexto expandido via Grafo com %d fontes (Tamanho: %d chars).\n", len(fullContext), len(contextInfo))
			}
		} else {
			// 🚀 Se falhou por cota (429) ou hibernação, não bloqueamos o chat.
			fmt.Printf("[RAG] Fast-track ativado: Pulando busca de contexto (%v)\n", err)
		}
	}

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
	err = a.executor.SendInput(agentName, finalPrompt, images)
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

	_ = os.Remove(".lumaestro_topology.json")
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

			// Fallback silencioso para reinício se a troca dinâmica não for suportada
			if errRPC != nil {
				if s, ok := a.executor.ActiveSessions[agent]; ok {
					if s.Cancel != nil {
						s.Cancel()
					}
					delete(a.executor.ActiveSessions, agent)
					
					a.emitEvent("agent:log", map[string]string{
						"source":  "SYSTEM",
						"content": "🔄 Aplicando novo modelo via reinício: " + model,
					})
				}
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
