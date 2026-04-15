package provider

import (
	"Lumaestro/internal/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GroqProvider implementa ContentGenerator usando a API do Groq (OpenAI-compatible).
type GroqProvider struct {
	model      string
	httpClient *http.Client
}

func NewGroqProvider(apiKey, model string) *GroqProvider {
	if model == "" {
		model = "qwen/qwen3-32b"
	}
	return &GroqProvider{
		model: model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (g *GroqProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	for {
		cfg, _ := config.Load()
		if cfg == nil {
			return "", fmt.Errorf("falha ao carregar configuração para Groq")
		}

		activeKey := cfg.GetActiveGroqKey()
		if activeKey == "" {
			return "", fmt.Errorf("nenhuma chave Groq configurada no pool")
		}

		payload := map[string]interface{}{
			"model": g.model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"temperature": 0.2,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+activeKey)

		resp, err := g.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("falha ao conectar na Groq: %v", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		// 🔄 Lógica de Rotação & Backoff Inteligente (Rate Limit ou Forbidden)
		if resp.StatusCode == http.StatusTooManyRequests {
			if cfg.GroqKeyCount() > 1 {
				fmt.Printf("[GroqPool] ⚠️ Rate Limit na Chave #%d. Rotacionando...\n", cfg.GroqKeyIndex+1)
				cfg.RotateGroqKey()
				continue // Roteador assume próxima chave
			} else {
				// Se tivermos apenas 1 chave, precisamos domar o Crawler e usar o freio-motor
				fmt.Printf("[GroqPool] ⏳ Limite de %d RPM estourado! Acionando Backoff. O Crawler vai entrar em hibernação por 22 segundos...\n", 6000)
				time.Sleep(22 * time.Second)
				continue // Tenta de novo após o cooldown
			}
		} else if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
			if cfg.GroqKeyCount() > 1 {
				fmt.Printf("[GroqPool] ⚠️ Chave #%d Expirada/Inválida. Rotacionando...\n", cfg.GroqKeyIndex+1)
				cfg.RotateGroqKey()
				continue
			}
			// Se for 401/403 com 1 chave só, quebra o loop senão roda infinito
			return "", fmt.Errorf("Erro fatal 401/403: Chave Groq inválida ou sem saldo")
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("[Groq] ❌ Erro de API (%d): %s\n", resp.StatusCode, string(respBody))
			return "", fmt.Errorf("Groq API retornou %d: %s", resp.StatusCode, string(respBody))
		}

		var chatResp lmChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			fmt.Printf("[Groq] ⚠️ Recebido documento não-JSON (Status %d). Primeiros 200 chars: %s\n", resp.StatusCode, string(respBody)[:min(len(respBody), 200)])
			return "", fmt.Errorf("erro ao parsear resposta da Groq: %v", err)
		}

		if len(chatResp.Choices) > 0 {
			return chatResp.Choices[0].Message.Content, nil
		}

		return "", fmt.Errorf("Groq retornou resposta vazia")
	}
}

func (g *GroqProvider) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	// Por enquanto a API Groq via LPU foca em texto de alta velocidade. 
	// Suporte a multimodal pode variar por modelo, mas mantemos o fallback de erro por segurança.
	return "", fmt.Errorf("provedor Groq no Lumaestro suporta apenas Texto/RAG no momento")
}

// Stop não faz nada para provedores Cloud.
func (g *GroqProvider) Stop() {}
