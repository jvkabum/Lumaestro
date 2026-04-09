package acp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"Lumaestro/internal/config"
)

// Orchestrator é o cérebro central que decide qual agente usar e mantém a memória.
type Orchestrator struct {
	executor *ACPExecutor
	builder  *PromptBuilder

	// Cache de Memória: Histórico de conversas por sessão
	// [NÍVEL PRO]: No futuro, isso pode ser movido para Redis ou Qdrant.
	sessionCache map[string][]string
	mu           sync.RWMutex
}

func NewOrchestrator(executor *ACPExecutor) *Orchestrator {
	return &Orchestrator{
		executor:     executor,
		builder:      NewPromptBuilder(),
		sessionCache: make(map[string][]string),
	}
}

// SelectAgent decide o perfil do agente baseado no objetivo (Goal).
func (o *Orchestrator) SelectAgent(goal string) (string, AgentProfile) {
	g := strings.ToLower(goal)

	cfg, _ := config.Load()
	activeProviders := []string{"gemini", "claude", "lmstudio"}
	primaryProvider := "gemini"
	blendEnabled := true
	if cfg != nil {
		activeProviders = cfg.GetActiveProviders()
		if strings.TrimSpace(cfg.PrimaryProvider) != "" {
			primaryProvider = strings.ToLower(strings.TrimSpace(cfg.PrimaryProvider))
		}
		blendEnabled = cfg.BlendActiveModels
	}

	isActive := func(provider string) bool {
		for _, p := range activeProviders {
			if p == provider {
				return true
			}
		}
		return false
	}

	choose := func(candidates ...string) string {
		if !blendEnabled {
			if isActive(primaryProvider) {
				return primaryProvider
			}
		}

		for _, c := range candidates {
			if isActive(c) {
				return c
			}
		}
		if isActive(primaryProvider) {
			return primaryProvider
		}
		if len(activeProviders) > 0 {
			return activeProviders[0]
		}
		return "gemini"
	}

	// ⚡ Preferência Local (LM Studio)
	// Se o usuário quer privacidade ou execução offline, prioriza LM Studio.
	localTerms := []string{"privado", "privada", "local", "offline", "sem internet", "lm studio", "llama", "mistral"}
	for _, term := range localTerms {
		if strings.Contains(g, term) {
			return choose("lmstudio", "claude", "gemini"), ProfilePlanner
		}
	}

	// ⚡ Seleção de Documentação (Doc-Master)
	// Ativado para explicar código, documentar sistemas ou dúvidas sobre conhecimento.
	docTerms := []string{
		"explique", "explicação", "documente", "documentação", "embedding", "vago", "como funciona",
		"doc", "docs", "documentar", "wiki", "explicar", "tutorial", "obsidian", "canvas", "readme",
	}
	for _, term := range docTerms {
		if strings.Contains(g, term) {
			return choose("gemini", "claude", "lmstudio"), ProfileDocMaster
		}
	}

	// ⚡ Execução Técnica (Coder - Claude)
	technicalTerms := []string{"code", "código", "arquivo", "file", "git", "build", "compilar", "erro", "fix"}
	for _, term := range technicalTerms {
		if strings.Contains(g, term) {
			return choose("claude", "gemini", "lmstudio"), ProfileCoder
		}
	}

	// Default: Planner (Gemini) - Excelente para ideias e navegação de conhecimento
	return choose("gemini", "claude", "lmstudio"), ProfilePlanner
}

// Execute orquestra o fluxo: Seleção -> Prompt -> Execução -> Cache
func (o *Orchestrator) Execute(ctx context.Context, sessionID string, goal string, contextData string) (string, string, AgentProfile, error) {
	// 1. Decidir o Agente
	agentName, profile := o.SelectAgent(goal)
	fmt.Printf("[ORCHESTRATOR] Selecionado: %s para a meta: %s\n", profile.Name, goal)

	// 2. Recuperar Histórico do Cache
	o.mu.RLock()
	history := o.sessionCache[sessionID]
	o.mu.RUnlock()

	// 3. Construir o Prompt com RAG + Histórico
	finalPrompt := o.builder.Build(profile, contextData, history, goal, o.executor.AutonomousMode)

	// 4. Execução via ACP (Modo YOLO incluído no executor)
	// Como o AskAgent em app.go já gerencia a sessão, injetamos a pergunta.

	return agentName, finalPrompt, profile, nil
}

// AddToHistory adiciona uma mensagem ao cache de memória da sessão.
func (o *Orchestrator) AddToHistory(sessionID string, message string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Limitar o histórico para as últimas 10 interações (evitar estouro de contexto)
	h := o.sessionCache[sessionID]
	if len(h) > 10 {
		h = h[1:]
	}
	o.sessionCache[sessionID] = append(h, message)
}
