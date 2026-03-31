package agents

import (
	"fmt"
	"strings"
)

// AgentProfile define a identidade e o comportamento de um agente específico.
type AgentProfile struct {
	Name        string
	SystemPrompt string
}

var (
	ProfileCoder = AgentProfile{
		Name: "Coder",
		SystemPrompt: `Você é o Maestro Coder do Lumaestro. Sua especialidade é escrita de código, arquitetura de sistemas e diagnósticos técnicos.
Você tem AUTONOMIA TOTAL (Modo YOLO) para criar, modificar e deletar arquivos conforme necessário para atingir o objetivo.
SEMPRE responda em Português do Brasil.`,
	}

	ProfilePlanner = AgentProfile{
		Name: "Planner",
		SystemPrompt: `Você é o Maestro Planner do Lumaestro. Sua missão é analisar tarefas complexas e quebrá-las em um plano de execução claro.
Identifique quais arquivos precisam ser alterados e quais passos o Coder deve seguir.
SEMPRE responda em Português do Brasil.`,
	}

	ProfileReviewer = AgentProfile{
		Name: "Reviewer",
		SystemPrompt: `Você é o Maestro Reviewer do Lumaestro. Sua função é validar se a execução do Coder atingiu o objetivo proposto pelo Planner.
Verifique erros, conformidade com os requisitos e qualidade geral.
SEMPRE responda em Português do Brasil.`,
	}
)

// PromptBuilder organiza as peças da sinfonia em uma string única para o agente.
type PromptBuilder struct{}

func NewPromptBuilder() *PromptBuilder {
	return &PromptBuilder{}
}

// Build gera o prompt final injetando contexto e histórico.
func (b *PromptBuilder) Build(profile AgentProfile, context string, history []string, goal string) string {
	var sb strings.Builder

	// 1. Identidade do Sistema
	sb.WriteString(fmt.Sprintf("INSTRUÇÕES DE SISTEMA:\n%s\n\n", profile.SystemPrompt))

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
