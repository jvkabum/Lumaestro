package provider

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// EmbeddingService gerencia a geração de vetores via Gemini.
type EmbeddingService struct {
	Client *genai.Client
}

// NewEmbeddingService inicializa o serviço com a API Key.
func NewEmbeddingService(ctx context.Context, apiKey string) (*EmbeddingService, error) {
	// BackendGeminiAPI é o nome correto no SDK Unificado
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao criar cliente GenAI: %w", err)
	}

	return &EmbeddingService{Client: client}, nil
}

// GenerateEmbedding transforma um texto em um vetor []float32.
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Preparar o conteúdo conforme exigido pelo SDK Unificado
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}

	// Usando o modelo gemini-embedding-2-preview conforme vanguarda da API
	res, err := s.Client.Models.EmbedContent(ctx, "gemini-embedding-2-preview", contents, nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	// No novo SDK Unificado, o resultado é uma lista em res.Embeddings
	if len(res.Embeddings) == 0 || res.Embeddings[0] == nil || len(res.Embeddings[0].Values) == 0 {
		return nil, fmt.Errorf("vetor de embedding vazio na resposta")
	}

	return res.Embeddings[0].Values, nil
}
