package provider

import (
	"Lumaestro/internal/config"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GroqProvider implementa ContentGenerator usando a API do Groq com Frota de Resiliência Multi-Modelo.
type GroqProvider struct {
	model      string
	httpClient *http.Client
}

func NewGroqProvider(apiKey, model string) *GroqProvider {
	if model == "" {
		model = "llama-3.3-70b-versatile" // Default de elite
	}
	return &GroqProvider{
		model: model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// GenerateText realiza a geração com a Frota de Resiliência (Cascata de Modelos e Chaves).
func (g *GroqProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	// 🚀 FROTA GROQ DINÂMICA (Lê os modelos ativos da configuração do Maestro)
	cfg, _ := config.Load()
	fleet := cfg.ActiveGroqModels

	// Fallback de Segurança caso a lista esteja vazia
	if len(fleet) == 0 {
		fleet = []string{
			"llama-3.3-70b-versatile",
			"openai/gpt-oss-120b",
			"qwen/qwen3-32b",
			"moonshotai/kimi-k2-instruct",
			"moonshotai/kimi-k2-instruct-0905",
			"meta-llama/llama-4-scout-17b-16e-instruct",
			"openai/gpt-oss-20b",
			"allam-2-7b",
			"llama-3.1-8b-instant",
			"groq/compound",
			"groq/compound-mini",
		}
	}

	maxFleetCycles := 3
	cycles := 0

	for {
		cycles++
		for _, modelName := range fleet {
			keyCount := cfg.GroqKeyCount()
			if keyCount == 0 {
				return "", fmt.Errorf("nenhuma chave Groq no pool")
			}

			for i := 0; i < keyCount; i++ {
				activeKey := cfg.GetActiveGroqKey()
				fmt.Printf("[GroqResilience] 🚀 Tentando %s (Chave %d/%d - Ciclo %d)\n", modelName, cfg.GroqKeyIndex+1, keyCount, cycles)

				// Lógica de Prompt Específica: Qwen precisa de diretiva /no_think
				finalPrompt := prompt
				if strings.Contains(modelName, "qwen") {
					finalPrompt = "/no_think\n" + prompt
				}

				payload := map[string]interface{}{
					"model": modelName,
					"messages": []map[string]string{
						{"role": "user", "content": finalPrompt},
					},
					"temperature": 0.0,
					"max_tokens":  4096,
				}

				body, _ := json.Marshal(payload)
				req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+activeKey)

				resp, err := g.httpClient.Do(req)
				if err != nil {
					fmt.Printf("[GroqResilience] 🚩 Erro de rede: %v. Pulando para próxima chave...\n", err)
					cfg.RotateGroqKey()
					continue
				}
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)

				// 🔄 Rotação por Rate Limit (429) ou Erro de Provedor (5xx)
				if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
					fmt.Printf("[GroqResilience] ⚠️ Modelo %s exausto ou instável (Status %d). Rotacionando chave...\n", modelName, resp.StatusCode)
					cfg.RotateGroqKey()
					continue
				}

				// 🚫 Chave Suspensa ou Inválida (401/403)
				if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
					fmt.Printf("[GroqResilience] 🚫 Chave #%d SUSPENSA. Pulando modelo...\n", cfg.GroqKeyIndex+1)
					cfg.RotateGroqKey()
					break // Pula para o próximo MODELO para economizar tempo
				}

				if resp.StatusCode != http.StatusOK {
					fmt.Printf("[GroqResilience] ❌ Erro Crítico (%d): %s. Pulando modelo...\n", resp.StatusCode, string(respBody))
					break
				}

				var chatResp struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if err := json.Unmarshal(respBody, &chatResp); err != nil {
					fmt.Printf("[GroqResilience] ⚠️ Resposta malformada do modelo %s. Pulando...\n", modelName)
					break
				}

				if len(chatResp.Choices) > 0 {
					return chatResp.Choices[0].Message.Content, nil
				}

				fmt.Printf("[GroqResilience] ⚠️ Modelo %s retornou vazio. Pulando...\n", modelName)
				break
			}
		}

		if cycles >= maxFleetCycles {
			return "", fmt.Errorf("falha catastrófica: Frota Groq (%d modelos em cascata) exausta após %d ciclos", len(fleet), maxFleetCycles)
		}

		fmt.Println("⏳ [GroqResilience] 🚨 Toda a frota Groq falhou! Hibernação tática de 30s... 😴")
		time.Sleep(30 * time.Second)
		fmt.Println("⚡ [GroqResilience] Acordando. Reiniciando cascata de elite...")
	}
}

// GenerateMultimodalText fallback para Groq (focado em texto LPU).
func (g *GroqProvider) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	return "", fmt.Errorf("provedor Groq focado em RAG de alta velocidade (Texto). Use GoogleProvider para Multimodal.")
}

func (g *GroqProvider) Stop() {}
