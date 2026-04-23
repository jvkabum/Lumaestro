package acp

import (
	"fmt"
	"strings"

	"Lumaestro/internal/prompts"
)

// AgentProfile define a identidade e o comportamento de um agente específico.
type AgentProfile struct {
	Name         string
	SystemPrompt string
}

var (
	ProfileCoder = AgentProfile{
		Name:         "Coder",
		SystemPrompt: prompts.GetCoderSystemPrompt(),
	}

	ProfilePlanner = AgentProfile{
		Name:         "Planner",
		SystemPrompt: prompts.GetPlannerSystemPrompt(),
	}

	ProfileReviewer = AgentProfile{
		Name:         "Reviewer",
		SystemPrompt: prompts.GetReviewerSystemPrompt(),
	}

	ProfileDocMaster = AgentProfile{
		Name:         "Doc-Master",
		SystemPrompt: prompts.GetDocMasterSystemPrompt(),
	}
)

// PromptBuilder organiza as peças da sinfonia em uma string única para o agente.
type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// Build gera o prompt final injetando contexto e histórico.
func (b *PromptBuilder) Build(profile AgentProfile, context string, history []string, goal string, autonomous bool) string {
	var sb strings.Builder

	// 1. Identidade e Idioma do Sistema
	sb.WriteString(fmt.Sprintf("%s\n\n", prompts.GetLanguageDirective()))
	sb.WriteString(fmt.Sprintf("%s\n\n", prompts.GetEnvironmentDirective()))
	sb.WriteString(fmt.Sprintf("%s\n\n", prompts.GetAutonomyDirective(autonomous)))
	sb.WriteString(fmt.Sprintf("INSTRUÇÕES DE SISTEMA:\n%s\n\n", profile.SystemPrompt))
	sb.WriteString(fmt.Sprintf("%s\n\n", prompts.GetLightningDirective()))
	sb.WriteString(fmt.Sprintf("%s\n\n", prompts.GetNavigationDirective()))

	// 2. Contexto do Obsidian (RAG)
	if context != "" {
		sb.WriteString("CONTEXTO DO CONHECIMENTO (OBSIDIAN):\n")
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}

	// 3. Histórico Recente (Memória Viva)
	if len(history) > 0 {
		sb.WriteString("HISTÓRICO DA CONVERSA:\n")
		for _, h := range history {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}
		sb.WriteString("\n")
	}

	// 4. A Grande Meta (O que fazer agora)
	sb.WriteString("OBJETIVO ATUAL:\n")
	sb.WriteString(goal)

	return sb.String()
}
