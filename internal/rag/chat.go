package rag

import (
	"context"
	"fmt"
	"Lumaestro/internal/agents"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/tools"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"time"
)

// ChatService orquestra a inteligência via processos CLI.
type ChatService struct {
	Executor     *agents.Executor
	Orchestrator *agents.Orchestrator
	Search       *SearchService
	Nav          *GraphNavigator
	Embedder     *provider.EmbeddingService
	Installer    *tools.Installer
}

// NewChatService inicializa o orquestrador de chat baseado em CLI.
func NewChatService(executor *agents.Executor, orchestrator *agents.Orchestrator, search *SearchService, nav *GraphNavigator, embedder *provider.EmbeddingService, installer *tools.Installer) *ChatService {
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
	runtime.EventsEmit(ctx, "graph:log", fmt.Sprintf("[%s] 🔍 buscando '%s'...", now, question))
	
	vector, err := s.Embedder.GenerateEmbedding(ctx, question)
	if err != nil {
		runtime.EventsEmit(ctx, "graph:log", fmt.Sprintf("[%s] ❌ Erro ao criar semântica.", now))
		return "", err
	}
	
	// 2. Busca Vetorial (RAG): Busca por proximidade semântica
	// Aumentado de 3 para 5 para maior cobertura de contexto
	notes, _ := s.Search.SearchNote(ctx, vector, 5)
	if len(notes) > 0 {
		runtime.EventsEmit(ctx, "graph:log", fmt.Sprintf("[%s] 📄 encontradas %d notas matrizes para a resposta.", now, len(notes)))
	}

	fullContext := s.Nav.ExpandContext(ctx, notes)
	contextData := strings.Join(fullContext, "\n---\n")

	// 3. Brilhar as notas iniciais encontradas no Grafo e lançar Log
	for _, note := range notes {
		if noteName, ok := note["name"].(string); ok {
			runtime.EventsEmit(ctx, "graph:log", fmt.Sprintf("[%s] ✨ lendo notas mestre -> %s", time.Now().Format("15:04"), noteName))
			runtime.EventsEmit(ctx, "node:active", noteName)
			
			// Envia o própio nó principal para o painel se ele não foi carregado
			runtime.EventsEmit(ctx, "graph:node", map[string]string{"id": noteName, "name": noteName})
		}
	}

	// 4. Delegar ao Orquestrador Inteligente
	// Ele decidirá se usa Coder ou Planner e montará o prompt com histórico
	selectedAgent, finalPrompt, err := s.Orchestrator.Execute(ctx, sessionID, question, contextData)
	if err != nil {
		return "", err
	}

	// 5. Execução via CLI (Modo YOLO Automático via ACP)
	// Como o AskAgent em app.go já gerencia a sessão, retornamos o prompt finalizado.
	// O app.go injetará este prompt na sessão do agente correto.
	
	_ = selectedAgent // Por enquanto, o app.go decide o canal (legacy ou ACP)
	return finalPrompt, nil
}
