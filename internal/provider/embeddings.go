package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"Lumaestro/internal/config"
	"Lumaestro/internal/utils"

	"google.golang.org/genai"
)

// EmbeddingService gerencia a geração de vetores via Gemini com suporte a pool de chaves e detecção de quota.
type EmbeddingService struct {
	Client        *genai.Client
	ctx           context.Context
	Mu            sync.Mutex
	keys          []string
	CurrentKeyIdx int
}

// NewEmbeddingService inicializa o serviço com o pool de chaves configurado.
func NewEmbeddingService(ctx context.Context, apiKey string) (*EmbeddingService, error) {
	cfg, _ := config.Load()
	var keys []string
	if cfg != nil {
		keys = cfg.GetGeminiKeys()
	}
	if len(keys) == 0 && apiKey != "" {
		keys = []string{apiKey}
	}

	activeKey := apiKey
	if len(keys) > 0 {
		activeKey = keys[0]
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  activeKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao criar cliente de embeddings GenAI: %w", err)
	}

	return &EmbeddingService{
		Client:        client,
		ctx:           ctx,
		keys:          keys,
		CurrentKeyIdx: 0,
	}, nil
}

// rotateAndRetry tenta rotacionar a chave e recriar o client.
func (s *EmbeddingService) rotateAndRetry() bool {
	cfg, err := config.Load()
	if err != nil || cfg == nil || cfg.GeminiKeyCount() <= 1 {
		return false
	}

	newKey := cfg.RotateGeminiKey()
	if newKey == "" {
		return false
	}

	newClient, err := genai.NewClient(s.ctx, &genai.ClientConfig{
		APIKey:  newKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Printf("[KeyPool-Embed] ❌ Falha ao rotacionar chave: %v\n", err)
		return false
	}

	s.Client = newClient
	return true
}

// GenerateEmbedding transforma um texto em um vetor []float32.
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string, fastTrack bool) ([]float32, error) {
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{Text: text},
			},
		},
	}
	return s.embedWithRetry(ctx, contents, fastTrack)
}

// GenerateMultimodalEmbedding transforma um binário em um vetor []float32.
func (s *EmbeddingService) GenerateMultimodalEmbedding(ctx context.Context, data []byte, mimeType string, fastTrack bool) ([]float32, error) {
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				{
					InlineData: &genai.Blob{
						Data:     data,
						MIMEType: mimeType,
					},
				},
			},
		},
	}
	return s.embedWithRetry(ctx, contents, fastTrack)
}

// embedWithRetry é o motor central de fomento a vetores com Throttle defensivo.
func (s *EmbeddingService) embedWithRetry(ctx context.Context, contents []*genai.Content, fastTrack bool) ([]float32, error) {
	model := "gemini-embedding-2-preview"

	for {
		res, err := s.Client.Models.EmbedContent(ctx, model, contents, nil)
		if err == nil {
			if len(res.Embeddings) > 0 && res.Embeddings[0] != nil && len(res.Embeddings[0].Values) > 0 {
				return res.Embeddings[0].Values, nil
			}
			return nil, fmt.Errorf("vetor de embedding vazio")
		}

		if !utils.IsQuotaError(err) && !utils.IsSuspendedError(err) {
			return nil, fmt.Errorf("erro fatal em embedding: %w", err)
		}

		cfg, _ := config.Load()
		maxRetries := 0
		if cfg != nil {
			maxRetries = cfg.GeminiKeyCount() - 1
		}

		rotatedSuccess := false
		for i := 0; i < maxRetries; i++ {
			if !s.rotateAndRetry() {
				break
			}
			res, err = s.Client.Models.EmbedContent(ctx, model, contents, nil)
			if err == nil {
				rotatedSuccess = true
				break
			}
			if !utils.IsQuotaError(err) {
				return nil, fmt.Errorf("erro fatal em embedding (pós-rotação): %w", err)
			}
		}

		if rotatedSuccess {
			if len(res.Embeddings) > 0 && res.Embeddings[0] != nil && len(res.Embeddings[0].Values) > 0 {
				return res.Embeddings[0].Values, nil
			}
		}

		if fastTrack {
			return nil, fmt.Errorf("quota_exhausted: chaves exaustas (fast-track)")
		}

		fmt.Println("⏳ [KeyPool-Embed] 🚨 Todas as chaves exaustas! Dormindo 30s... 😴")
		time.Sleep(30 * time.Second)
	}
}
