package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LMStudioEmbedder implementa as interfaces Embedder e ContentGenerator via LM Studio (API OpenAI-compatível).
// Para Embeddings: requer um modelo de embeddings carregado (ex: nomic-embed-text).
// Para Texto: usa o modelo de chat carregado (ex: google/gemma-4-26b-a4b).
type LMStudioEmbedder struct {
	Client    *LMStudioClient
	EmbedModel string // Modelo para embeddings (ex: nomic-embed-text, text-embedding-nomic-embed-text-v1.5)
	ChatModel  string // Modelo para geração de texto (ex: google/gemma-4-26b-a4b)
}

// NewLMStudioEmbedder cria um novo embedder apontando para o servidor LM Studio.
func NewLMStudioEmbedder(baseURL, embedModel, chatModel string) *LMStudioEmbedder {
	return &LMStudioEmbedder{
		Client:     NewLMStudioClient(baseURL),
		EmbedModel: embedModel,
		ChatModel:  chatModel,
	}
}

// ─── Embedder interface ──────────────────────────────────────────────────────

// GenerateEmbedding gera um vetor denso via LM Studio /v1/embeddings.
func (e *LMStudioEmbedder) GenerateEmbedding(ctx context.Context, text string, fastTrack bool) ([]float32, error) {
	model := strings.TrimSpace(e.EmbedModel)
	if model == "" {
		return nil, fmt.Errorf("modelo de embeddings do LM Studio nao configurado")
	}

	payload := lmEmbeddingRequest{
		Model: model,
		Input: text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Client.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LM Studio embeddings inacessível: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio embeddings retornou %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp lmEmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("erro ao parsear embedding LM Studio: %v", err)
	}
	if embResp.Error != nil {
		return nil, fmt.Errorf("LM Studio embeddings error: %s", embResp.Error.Message)
	}
	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("LM Studio retornou embedding vazio")
	}

	return embResp.Data[0].Embedding, nil
}

// GenerateMultimodalEmbedding — LM Studio não suporta embeddings multimodais.
// Retorna erro informativo para que o chamador possa fazer fallback.
func (e *LMStudioEmbedder) GenerateMultimodalEmbedding(ctx context.Context, data []byte, mimeType string, fastTrack bool) ([]float32, error) {
	return nil, fmt.Errorf("LM Studio não suporta embeddings multimodais (tipo: %s)", mimeType)
}

// ─── ContentGenerator interface ─────────────────────────────────────────────

// GenerateText utiliza o modelo de chat do LM Studio para geração de texto.
func (e *LMStudioEmbedder) GenerateText(ctx context.Context, prompt string) (string, error) {
	return e.Client.Chat(ctx, e.ChatModel, "", prompt)
}

// GenerateMultimodalText — LM Studio não suporta entrada multimodal neste cliente.
// Retorna erro informativo; o chamador deve usar o fallback Gemini para mídia.
func (e *LMStudioEmbedder) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	return "", fmt.Errorf("LM Studio não suporta geração multimodal (tipo: %s); use Gemini para processar mídia", mimeType)
}
