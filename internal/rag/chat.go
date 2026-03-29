package rag

import (
	"context"
	"fmt"
	"Lumaestro/internal/agents"
	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/tools"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"google.golang.org/genai"
)

// ChatService orquestra a inteligência via processos CLI.
type ChatService struct {
	Executor  *agents.Executor
	Search    *SearchService
	Nav       *GraphNavigator
	Embedder  *provider.EmbeddingService
	Installer *tools.Installer
}

// NewChatService inicializa o orquestrador de chat baseado em CLI.
func NewChatService(executor *agents.Executor, search *SearchService, nav *GraphNavigator, embedder *provider.EmbeddingService, installer *tools.Installer) *ChatService {
	return &ChatService{
		Executor:  executor,
		Search:    search,
		Nav:       nav,
		Embedder:  embedder,
		Installer: installer,
	}
}

// Ask orquestra o fluxo GUI-CLI: Query -> RAG -> CLI Stdin -> Output
func (s *ChatService) Ask(ctx context.Context, agent string, question string) (string, error) {
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
		if strings.Contains(q, "obsidian") {
			s.Installer.InstallObsidian()
			return "Iniciando instalação do Obsidian CLI... Acompanhe o progresso no terminal ao lado.", nil
		}
	}

	// 2. Gerar vetor da pergunta (Usa API via SDK para performance)
	vector, err := s.Embedder.GenerateEmbedding(ctx, question)
	if err != nil {
		return "", err
	}

	// 3. Buscar notas similares (Qdrant)
	notes, _ := s.Search.SearchNote(ctx, vector, 3)

	// 4. Expandir contexto via Grafo (Multi-hop RAG)
	fullContext := s.Nav.ExpandContext(ctx, notes)

	// --- PERFORMANCE SÍNCRONA ---
	// Notificar o Grafo para brilhar as notas encontradas ANTES da IA falar
	for _, note := range notes {
		if path, ok := note["path"].(string); ok {
			noteName := strings.TrimSuffix(filepath.Base(path), ".md")
			runtime.EventsEmit(ctx, "node:highlight", noteName)
		}
	}

	// 5. Decisão de Motor (Sempre respeitando a preferência do usuário)
	cfg, _ := config.Load()
	useSDK := false
	if agent == "gemini" && cfg != nil && cfg.UseGeminiAPIKey {
		useSDK = true
	} else if agent == "claude" && cfg != nil && cfg.UseClaudeAPIKey {
		useSDK = true
	}

	contextData := strings.Join(fullContext, "\n---\n")
	
	if useSDK {
		prompt := fmt.Sprintf("CONTEXTO DO CONHECIMENTO:\n%s\n\nUSUÁRIO: %s", contextData, question)
		res, err := s.Embedder.Client.Models.GenerateContent(ctx, "gemini-2.0-flash", genai.Text(prompt), nil)
		if err != nil {
			return "", fmt.Errorf("erro na sinfonia: %w", err)
		}
		if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
			return fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]), nil
		}
		return "O Maestro não encontrou as notas certas para responder via API.", nil
	}

	// MODO CLI (LOGIN/OAUTH) - O coração orquestrador
	output, err := s.Executor.ExecuteCLI(ctx, agent, contextData, question)
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return fmt.Sprintf("O agente '%s' não parece estar instalado para login. Digite 'instalar %s'!", agent, agent), nil
		}
		return "", err
	}

	return output, nil
}
