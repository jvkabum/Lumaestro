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
	QuotaManager  *QuotaManager
}

// NewEmbeddingService inicializa o serviço com o pool de chaves configurado e QuotaManager integrado.
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

	qm := GetGlobalQuotaManager(keys)

	return &EmbeddingService{
		Client:        client,
		ctx:           ctx,
		keys:          keys,
		CurrentKeyIdx: 0,
		QuotaManager:  qm,
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

// embedWithRetry é o motor central de fomento a vetores com QuotaManager e resiliência enterprise.
func (s *EmbeddingService) embedWithRetry(ctx context.Context, contents []*genai.Content, fastTrack bool) ([]float32, error) {
	model := "gemini-embedding-2-preview"

	qm := s.QuotaManager
	if qm == nil {
		qm = GetGlobalQuotaManager(s.keys)
		s.QuotaManager = qm
	}

	for {
		for i := 0; i < len(s.keys); i++ {
			s.Mu.Lock()
			actualKeyIdx := (s.CurrentKeyIdx + i) % len(s.keys)
			s.Mu.Unlock()
			key := s.keys[actualKeyIdx]

			if qm.IsExhausted(key, model) {
				continue
			}

			client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
			if err != nil {
				qm.HandleError(key, actualKeyIdx, model, err)
				qm.LogRequest(model, actualKeyIdx, key, "FAILED", err.Error())
				continue
			}

			res, err := client.Models.EmbedContent(ctx, model, contents, nil)
			if err == nil && res != nil && len(res.Embeddings) > 0 && res.Embeddings[0] != nil && len(res.Embeddings[0].Values) > 0 {
				qm.MarkSuccess(key, model)
				qm.LogRequest(model, actualKeyIdx, key, "SUCCESS", "")

				s.Mu.Lock()
				s.CurrentKeyIdx = (actualKeyIdx + 1) % len(s.keys)
				s.Client = client
				s.Mu.Unlock()
				return res.Embeddings[0].Values, nil
			}

			qm.HandleError(key, actualKeyIdx, model, err)
			errMsg := ""
			if err != nil {
				errMsg = err.Error()
			}
			qm.LogRequest(model, actualKeyIdx, key, "FAILED", errMsg)

			if !utils.IsQuotaError(err) && !utils.IsSuspendedError(err) {
				return nil, fmt.Errorf("erro fatal em embedding: %w", err)
			}
		}

		if fastTrack {
			return nil, fmt.Errorf("quota_exhausted: chaves exaustas (fast-track)")
		}

		shortestWait := qm.GetShortestCooldown([]string{model})
		if shortestWait > 60*time.Second {
			shortestWait = 60 * time.Second
		}
		if shortestWait < 5*time.Second {
			shortestWait = 5 * time.Second
		}

		fmt.Printf("⏳ [KeyPool-Embed] 🚨 Todas as chaves em cooldown para embedding! Aguardando %v... 😴\n", shortestWait.Round(time.Second))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(shortestWait):
		}
	}
}
