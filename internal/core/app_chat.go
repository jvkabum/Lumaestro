package core

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) emitAgentStatus(agent string, action string, kind string) {
	if a.ctx == nil || strings.TrimSpace(action) == "" {
		return
	}
	if strings.TrimSpace(kind) == "" {
		kind = "status"
	}
	runtime.EventsEmit(a.ctx, "agent:status", map[string]string{
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
		fmt.Printf("[BACKEND] Iniciando chamada de Chat para: %s\n", agentName)
		a.emitAgentStatus(agentName, "Orquestrando contexto e intenção do usuário", "status")
		// Usamos "default" como sessionID para manter o histórico em memória nesta sessão do app.
		response, err := a.chat.Ask(a.ctx, agentName, "default", prompt)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO no Chat: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha na Sinfonia: " + err.Error(),
			})
			return
		}

		fmt.Printf("[BACKEND] Resposta da Orquestração recebida. Injetando na sessão ACP...\n")
		a.emitAgentStatus(agentName, "Encaminhando plano para o agente ativo", "status")

		// Injeta a pergunta (prompt completo com RAG e histórico) na sessão ACP ativa
		// O executor cuidará de enviar via StdIn seguindo o protocolo ndJSON
		err = a.executor.SendInput(agentName, response, nil)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO ao enviar para o agente: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha ao comunicar com o agente: " + err.Error(),
			})
			return
		}
	}()

	return "Orquestrando..."
}

func (a *App) SendAgentInput(agent string, input string, images []map[string]string) error {
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

		vector, err := a.embedder.GenerateEmbedding(a.ctx, input, true)
		if err == nil {
			a.emitAgentStatus(agent, "Buscando referências semânticas relevantes", "memory")
			// 1. Busca as notas âncoras (Top 3)
			nodes, err := a.qdrant.Search("obsidian_knowledge", vector, 3)
			if err == nil && len(nodes) > 0 {
				a.emitAgentStatus(agent, "Expandindo contexto com memória conectada", "memory")
				// 2. Navegação de Sinapses: Expandir o contexto seguindo os links neurais
				fullContext := a.navigator.ExpandContext(a.ctx, nodes)
				contextInfo = "\n\n[CONHECIMENTO ORQUESTRADO (OBSIDIAN + SINAPSES)]\n"
				for _, ctxPart := range fullContext {
					contextInfo += ctxPart + "\n\n"
				}
				fmt.Printf("[RAG] Contexto expandido via Grafo com %d fontes.\n", len(fullContext))
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

	runtime.EventsEmit(a.ctx, "agent:profile", map[string]string{
		"name":   profileName,
		"engine": agentName,
	})
	a.emitAgentStatus(agentName, "Perfil ativo definido. Preparando execução", "status")

	// 🚀 Disparo ACP via Protocolo ndJSON
	if agentName == "lmstudio" {
		a.emitAgentStatus(agentName, "Inicializando sessão ACP do LM Studio", "status")
		if err := a.StartAgentSession("lmstudio"); err != nil {
			fmt.Printf("[App] ERRO ao iniciar sessão ACP do LM Studio: %v\n", err)
			return fmt.Errorf("erro ao iniciar lmstudio em modo ACP: %v", err)
		}
	}

	a.emitAgentStatus(agentName, "Enviando instruções para o agente", "status")
	err = a.executor.SendInput(agentName, finalPrompt, images)
	if err != nil {
		fmt.Printf("[App] ERRO no SendAgentInput: %v\n", err)
		return fmt.Errorf("erro ao enviar input para ACP: %v", err)
	}

	fmt.Printf("[App] ✅ Sinfonia roteada para %s com sucesso via JSON-RPC!\n", agent)

	// 📡 Feedback Imediato: Reseta o timer do frontend e avisa que o processamento começou
	runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
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

		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "RESOLVER",
			"content": fmt.Sprintf("✅ Conflito resolvido: '%s' agora é a verdade sobre '%s'.", newValue, subject),
		})
	} else {
		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
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
