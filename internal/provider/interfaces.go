package provider

import "context"

// Embedder define a capacidade de gerar vetores densos para busca semântica.
// Implementado por: EmbeddingService (Gemini), LMStudioEmbedder.
type Embedder interface {
	GenerateEmbedding(ctx context.Context, text string, fastTrack bool) ([]float32, error)
	GenerateMultimodalEmbedding(ctx context.Context, data []byte, mimeType string, fastTrack bool) ([]float32, error)
}

// ContentGenerator define a capacidade de gerar texto a partir de um prompt.
// Usado pela OntologyService para extração de triplas e validação de conflitos.
// Implementado por: EmbeddingService (Gemini cascade), LMStudioEmbedder.
type ContentGenerator interface {
	GenerateText(ctx context.Context, prompt string) (string, error)
	GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error)
}
