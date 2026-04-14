package provider

import (
	"context"
	"fmt"
	"strings"
)

// CollaborativeGenerator coordina dois motores locais: Qwen (Raciocínio) e Gemma (Linguística).
type CollaborativeGenerator struct {
	Reasoning *NativeGenerator // Geralmente Qwen (8086)
	General   *NativeGenerator // Geralmente Gemma (8087)
}

func NewCollaborativeGenerator(reasoning, general *NativeGenerator) *CollaborativeGenerator {
	return &CollaborativeGenerator{
		Reasoning: reasoning,
		General:   general,
	}
}

// GenerateText executa uma estratégia de distribuição de tarefas entre os motores locais.
func (c *CollaborativeGenerator) GenerateText(ctx context.Context, prompt string) (string, error) {
	// Fallback de segurança se algum motor falhar no boot
	if c.Reasoning != nil && c.General == nil {
		return c.Reasoning.GenerateText(ctx, prompt)
	}
	if c.General != nil && c.Reasoning == nil {
		return c.General.GenerateText(ctx, prompt)
	}
	if c.Reasoning == nil && c.General == nil {
		return "", fmt.Errorf("nenhum motor local disponível para o cérebro colaborativo")
	}

	// 🧠 ESTRATÉGIA DE DISTRIBUIÇÃO:
	
	// 1. Extração de Triplas e Lógica (Qwen é o Cirurgião)
	isExtraction := strings.Contains(strings.ToLower(prompt), "extraia") || 
					strings.Contains(strings.ToLower(prompt), "triplas") || 
					strings.Contains(strings.ToLower(prompt), "json")

	if isExtraction {
		return c.Reasoning.GenerateText(ctx, prompt)
	}

	// 2. Validação de Conflitos e Resumos (Gemma é o Curador)
	return c.General.GenerateText(ctx, prompt)
}

func (c *CollaborativeGenerator) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	// llama-server GGUF nativo ainda não suporta multimodal via API v1/chat de forma padronizada sem mmproj
	return "", fmt.Errorf("o cérebro colaborativo local suporta apenas texto/RAG por enquanto")
}
