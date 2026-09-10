package acp

import (
	"strings"
	"testing"
)

func TestPromptBuilder_RAGAwareness(t *testing.T) {
	builder := NewPromptBuilder()

	t.Run("Active Orbit with RAG Context", func(t *testing.T) {
		ctx := BuildContext{
			Orbit:      `C:\Projects\MyRepo`,
			RAGContext: "Nota 1: Arquitetura do backend em Go\nNota 2: Rotas de API",
			Goal:       "Adicionar nova rota",
			HasGraph:   true,
		}

		prompt := builder.Build(ProfileCoder, ctx)

		if !strings.Contains(prompt, "[RAG/CONHECIMENTO]") {
			t.Errorf("expected prompt to contain [RAG/CONHECIMENTO], got: %s", prompt)
		}
		if !strings.Contains(prompt, "Qdrant") {
			t.Errorf("expected prompt to mention Qdrant")
		}
		if !strings.Contains(prompt, "Grafo 3D") {
			t.Errorf("expected prompt to mention Grafo 3D")
		}
		if !strings.Contains(prompt, "view_file, grep_search") {
			t.Errorf("expected prompt to instruct using inspection tools to prevent lazy agent")
		}
		if !strings.Contains(prompt, "CONTEXTO:\nNota 1: Arquitetura") {
			t.Errorf("expected prompt to contain CONTEXTO block with data")
		}
	})

	t.Run("Active Orbit without RAG Context", func(t *testing.T) {
		ctx := BuildContext{
			Orbit:      `C:\Projects\MyRepo`,
			RAGContext: "",
			Goal:       "Olá, tudo bem?",
			HasGraph:   true,
		}

		prompt := builder.Build(ProfileCoder, ctx)

		if strings.Contains(prompt, "[RAG/CONHECIMENTO]") {
			t.Errorf("did not expect [RAG/CONHECIMENTO] when RAGContext is empty")
		}
		if strings.Contains(prompt, "CONTEXTO:") {
			t.Errorf("did not expect CONTEXTO: when RAGContext is empty")
		}
	})

	t.Run("Zero Orbit / Sandbox with RAG Context", func(t *testing.T) {
		ctx := BuildContext{
			Orbit:      "",
			RAGContext: "Alguma nota vazada",
			Goal:       "Quem é você?",
		}

		prompt := builder.Build(ProfileCoder, ctx)

		if strings.Contains(prompt, "[RAG/CONHECIMENTO]") {
			t.Errorf("did not expect [RAG/CONHECIMENTO] in zero orbit / sandbox")
		}
		if strings.Contains(prompt, "CONTEXTO:") {
			t.Errorf("did not expect CONTEXTO block in zero orbit / sandbox")
		}
		if !strings.Contains(prompt, "[AMNÉSIA/SANDBOX]") {
			t.Errorf("expected [AMNÉSIA/SANDBOX] in zero orbit")
		}
	})
}
