package rag

import (
	"context"
	"fmt"
	"Lumaestro/internal/agents"
	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/tools"
	"strings"

	"Lumaestro/internal/utils"
	"time"
)

// ChatService orquestra a inteligência via processos CLI.
type ChatService struct {
	ctx          context.Context // Contexto persistente do Wails
	Executor     *agents.Executor
	Orchestrator *acp.Orchestrator
	Search       *SearchService
	Nav          *GraphNavigator
	Embedder     provider.Embedder
	Installer    *tools.Installer
}

// SetContext injeta o contexto oficial do Wails.
func (s *ChatService) SetContext(ctx context.Context) {
	s.ctx = ctx
}

// NewChatService inicializa o orquestrador de chat baseado em CLI.
func NewChatService(executor *agents.Executor, orchestrator *acp.Orchestrator, search *SearchService, nav *GraphNavigator, embedder provider.Embedder, installer *tools.Installer) *ChatService {
	return &ChatService{
		Executor:     executor,
		Orchestrator: orchestrator,
		Search:       search,
		Nav:          nav,
		Embedder:     embedder,
		Installer:    installer,
	}
}

// Ask orquestra o fluxo GUI-CLI: Query -> RAG -> Orchestrator -> CLI Stdin -> Output
func (s *ChatService) Ask(ctx context.Context, agent string, sessionID string, question string) (string, error) {
	// 1. Detectar Intenção de Instalação (Pula o RAG se for comando de terminal)
	q := strings.ToLower(question)
	if strings.Contains(q, "instala") || strings.Contains(q, "download") || strings.Contains(q, "configura") {
		if strings.Contains(q, "gemini") {
			s.Installer.InstallGemini()
			return "Iniciando instalação do Gemini CLI... Acompanhe o progresso no terminal ao lado.", nil
		}
		if strings.Contains(q, "claude") {
			s.Installer.InstallClaude()
			return "Iniciando instalação do Claude Code... Acompanhe o progresso no terminal ao lado.", nil
		}
	}

	// 2. Gerar vetor da pergunta e relatar início do raciocínio
	now := time.Now().Format("15:04")
	utils.SafeEmit(s.ctx, "graph:log", fmt.Sprintf("[%s] 🔍 buscando '%s'...", now, question))

	contextData := ""
	if s.Embedder == nil {
		utils.SafeEmit(s.ctx, "graph:log", fmt.Sprintf("[%s] ⚠️ RAG semântico indisponível. Prosseguindo sem contexto vetorial.", now))
	} else {
		vector, err := s.Embedder.GenerateEmbedding(ctx, question, true)
		if err != nil {
			utils.SafeEmit(s.ctx, "graph:log", fmt.Sprintf("[%s] ⚠️ Semântica indisponível no momento. Prosseguindo sem RAG.", now))
		} else {
			// 2. Busca Vetorial (RAG): Busca por proximidade semântica
			// Aumentado de 3 para 5 para maior cobertura de contexto
			notes, _ := s.Search.SearchNote(ctx, vector, 5)
			if len(notes) > 0 {
				utils.SafeEmit(s.ctx, "graph:log", fmt.Sprintf("[%s] 📄 encontradas %d notas matrizes para a resposta.", now, len(notes)))
			}

			fullContext := s.Nav.ExpandContext(ctx, notes)
			contextData = strings.Join(fullContext, "\n---\n")

			// 3. Brilhar as notas iniciais encontradas no Grafo e lançar Log
			for i, note := range notes {
				if noteName, ok := note["name"].(string); ok {
					targetID := strings.ToLower(noteName)
					utils.SafeEmit(s.ctx, "graph:log", fmt.Sprintf("[%s] ✨ lendo notas mestre -> %s", time.Now().Format("15:04"), noteName))
					
					// Apenas a nota mais relevante (Top 1) ganha o foco automático da câmera
					if i == 0 {
						utils.SafeEmit(s.ctx, "node:active", targetID)
					}

					// Envia o próprio nó principal para o painel se ele não foi carregado
					utils.SafeEmit(s.ctx, "graph:node", map[string]string{"id": targetID, "name": noteName})
				}
			}
		}
	}

	// 4. Delegar ao Orquestrador Inteligente
	// Ele decidirá se usa Coder ou Planner e montará o prompt com histórico
	selectedAgent, finalPrompt, profile, err := s.Orchestrator.Execute(ctx, sessionID, question, contextData)
	if err != nil {
		return "", err
	}

	// 📡 Identidade Visual: Avisa o Frontend qual Perfil assumiu a palavra (Modo Silent RAG)
	utils.SafeEmit(s.ctx, "agent:profile", map[string]string{
		"name":   profile.Name,
		"engine": selectedAgent,
	})

	// 5. Execução via CLI (Modo YOLO Automático via ACP)
	// Como o AskAgent em app.go já gerencia a sessão, retornamos o prompt finalizado.
	// O app.go injetará este prompt na sessão do agente correto.
	
	_ = selectedAgent // Por enquanto, o app.go decide o canal (legacy ou ACP)
	return finalPrompt, nil
}
