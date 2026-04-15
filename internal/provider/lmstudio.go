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

	"Lumaestro/internal/prompts"
)

// LMStudioClient é um cliente HTTP para o servidor LM Studio (API OpenAI-compatível).
type LMStudioClient struct {
	BaseURL    string
	httpClient *http.Client
}

// NewLMStudioClient cria um novo cliente apontando para a URL base do LM Studio.
func NewLMStudioClient(baseURL string) *LMStudioClient {
	url := strings.TrimRight(baseURL, "/")
	if url == "" {
		url = "http://localhost:1234"
	}
	return &LMStudioClient{
		BaseURL: url,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ─── Tipos internos (OpenAI-compatíveis) ────────────────────────────────────

type lmMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type lmChatRequest struct {
	Model       string      `json:"model"`
	Messages    []lmMessage `json:"messages"`
	Temperature float64     `json:"temperature"`
	MaxTokens   int         `json:"max_tokens,omitempty"`
	Stream      bool        `json:"stream"`
}

type lmChoice struct {
	Message lmMessage `json:"message"`
}

type lmChatResponse struct {
	Choices []lmChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type lmModel struct {
	ID string `json:"id"`
}

type lmModelsResponse struct {
	Data []lmModel `json:"data"`
}

type lmEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type lmEmbeddingData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type lmEmbeddingResponse struct {
	Data  []lmEmbeddingData `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ─── Métodos públicos ────────────────────────────────────────────────────────

// ListModels retorna os IDs dos modelos carregados no LM Studio.
func (c *LMStudioClient) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LM Studio inacessível: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio retornou %d: %s", resp.StatusCode, string(body))
	}

	var modelsResp lmModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("erro ao parsear modelos: %v", err)
	}

	ids := make([]string, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

// Chat envia uma conversa única ao modelo e retorna o texto de resposta.
func (c *LMStudioClient) Chat(ctx context.Context, model, systemPrompt, userMessage string) (string, error) {
	messages := []lmMessage{}
	if systemPrompt != "" {
		messages = append(messages, lmMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, lmMessage{Role: "user", Content: userMessage})

	payload := lmChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.0,
		Stream:      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LM Studio inacessível: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LM Studio retornou %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp lmChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("erro ao parsear resposta: %v", err)
	}
	if chatResp.Error != nil {
		return "", fmt.Errorf("LM Studio error: %s", chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("LM Studio retornou resposta vazia")
	}
	return chatResp.Choices[0].Message.Content, nil
}

// DetectEmbeddingDimension consulta /v1/embeddings com o modelo indicado e retorna a dimensão real do vetor.
func (c *LMStudioClient) DetectEmbeddingDimension(ctx context.Context, model string) (int, error) {
	payload := lmEmbeddingRequest{
		Model: model,
		Input: "dimension_probe",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("LM Studio embeddings inacessivel: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("LM Studio embeddings retornou %d: %s", resp.StatusCode, string(respBody))
	}

	var embResp lmEmbeddingResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return 0, fmt.Errorf("erro ao parsear embedding: %v", err)
	}
	if embResp.Error != nil {
		return 0, fmt.Errorf("LM Studio embeddings error: %s", embResp.Error.Message)
	}
	if len(embResp.Data) == 0 || len(embResp.Data[0].Embedding) == 0 {
		return 0, fmt.Errorf("LM Studio retornou embedding vazio")
	}

	return len(embResp.Data[0].Embedding), nil
}

// LMTestResult é o resultado do teste de capacidade do modelo.
type LMTestResult struct {
	Success      bool     `json:"success"`
	ModelID      string   `json:"model_id"`
	LatencyMs    int64    `json:"latency_ms"`
	Capabilities []string `json:"capabilities"`
	Warnings     []string `json:"warnings"`
	ErrorMsg     string   `json:"error,omitempty"`
}

// TestModel executa um conjunto de verificações de capacidade no modelo:
//  1. Conectividade — consegue chamar /v1/chat/completions
//  2. Seguimento de instruções — responde ao pedido de saída JSON estruturado
//  3. Injeção de contexto — aceita system prompt + user prompt separados
func (c *LMStudioClient) TestModel(ctx context.Context, model string) LMTestResult {
	result := LMTestResult{ModelID: model}
	start := time.Now()

	// 1. Teste de seguimento de instrução + saída JSON estruturada
	systemPrompt := prompts.GetLMStudioTestSystemPrompt()
	userMessage := prompts.GetLMStudioTestUserPrompt()

	response, err := c.Chat(ctx, model, systemPrompt, userMessage)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.ErrorMsg = err.Error()
		return result
	}

	// 2. Verificar se a resposta é JSON válido
	cleanResponse := strings.TrimSpace(response)
	// Remove cerca de blocos markdown se o modelo insistir em adicioná-los
	if strings.HasPrefix(cleanResponse, "```") {
		lines := strings.Split(cleanResponse, "\n")
		var inner []string
		for _, l := range lines {
			if strings.HasPrefix(l, "```") {
				continue
			}
			inner = append(inner, l)
		}
		cleanResponse = strings.Join(inner, "\n")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(cleanResponse), &parsed); err != nil {
		// Modelo retornou algo que não é JSON — ainda pode ser útil, mas é uma limitação
		result.Success = true // Conectividade confirmada
		result.Capabilities = []string{"connectivity", "text_generation"}
		result.Warnings = append(result.Warnings,
			"O modelo não seguiu a instrução de retornar JSON puro. Funcionalidades de orquestração estruturada podem ser limitadas.")
		return result
	}

	// 3. Verificar campos esperados
	result.Capabilities = []string{"connectivity", "text_generation", "instruction_following", "json_output"}
	if parsed["status"] != "ok" {
		result.Warnings = append(result.Warnings, "Campo 'status' não retornou 'ok' — modelo pode estar interpretando o prompt de forma alternativa.")
	}

	result.Success = true
	return result
}
