package provider

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/utils"
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/genai"
)

// GoogleProvider implementa ContentGenerator usando a infraestrutura do Google (Gemini/Gemma).
// Focado exclusivamente em geração de texto e multimídia com alta resiliência (Frota).
type GoogleProvider struct {
	Client        *genai.Client
	ctx           context.Context
	Mu            sync.Mutex
	keys          []string
	CurrentKeyIdx int
}

// NewGoogleProvider inicializa o provedor Google com o pool de chaves configurado.
func NewGoogleProvider(ctx context.Context, apiKey string) (*GoogleProvider, error) {
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
		return nil, fmt.Errorf("falha ao criar cliente Google GenAI: %w", err)
	}

	return &GoogleProvider{
		Client:        client,
		ctx:           ctx,
		keys:          keys,
		CurrentKeyIdx: 0,
	}, nil
}

// GenerateContentWithRetry é o motor generativo unificado com Cascata de Modelos (Gemini -> Gemma) e Rotação de Chaves.
func (p *GoogleProvider) GenerateContentWithRetry(ctx context.Context, contents []*genai.Content) (*genai.GenerateContentResponse, error) {
	// Super Frota Dinâmica (Lê os modelos ativos da configuração do Maestro)
	cfg, _ := config.Load()
	models := cfg.ActiveGoogleModels

	// Fallback de Segurança caso a lista esteja vazia
	if len(models) == 0 {
		models = []string{
			"gemini-3.1-flash-lite-preview", // 🚀 Velocidade de Triplas (Lite 3.1)
			"gemini-2.5-flash",              // 🏆 Capitão Confirmado (Flash 2.5)
			"gemini-3-flash-preview",        // ⚖️ Moderno (Flash 3)
			"gemini-2.5-flash-lite",         // 📦 Escala de Volume
			"gemma-4-31b-it",                // 🛡️ O Tanque (Resiliência)
			"gemma-4-26b-a4b-it",            // 🐘 Reserva Tática
		}
	}

	if len(p.keys) == 0 {
		return nil, fmt.Errorf("nenhuma chave Google configurada para geração de conteúdo")
	}

	maxFleetCycles := 3
	cycles := 0

	for {
		cycles++
		for _, model := range models {
			// Tenta todas as chaves disponíveis para o modelo atual
			for i, key := range p.keys {
				// Garantir que o cliente usa a chave correta para esta tentativa
				client, _ := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})

				fmt.Printf("[ResilienceFleet] 🚀 Tentando modelo: %s (Chave %d/%d)\n", model, i+1, len(p.keys))

				temp := float32(0.0)
				config := &genai.GenerateContentConfig{
					Temperature: &temp,
				}

				resp, err := client.Models.GenerateContent(ctx, model, contents, config)
				if err == nil {
					// Sincroniza a chave de sucesso no estado do provedor
					p.Mu.Lock()
					p.CurrentKeyIdx = i
					p.Client = client
					p.Mu.Unlock()
					return resp, nil
				}

				if utils.IsQuotaError(err) {
					fmt.Printf("[ResilienceFleet] ⚠️ Cota exaurida no modelo %s (Chave %d). Rotacionando chave...\n", model, i+1)
					continue
				}

				if err != nil && (err.Error() == "PERMISSION_DENIED" || utils.IsSuspendedError(err)) {
					fmt.Printf("[ResilienceFleet] 🚫 Chave SUSPENSA detectada (%d). Pulando para o próximo modelo...\n", i+1)
					break 
				}

				fmt.Printf("[ResilienceFleet] 🚩 Erro no modelo %s: %v. Pulando para o próximo...\n", model, err)
				break 
			}
		}

		if cycles >= maxFleetCycles {
			return nil, fmt.Errorf("falha persistente: frotas Google/Gemma falharam após %d ciclos", maxFleetCycles)
		}
		fmt.Println("⏳ [ResilienceFleet] 🚨 Todos os modelos e chaves falharam! Hibernação de 30s... 😴")
		time.Sleep(30 * time.Second)
	}
}

// GenerateText satisfaz a interface ContentGenerator para geração de texto simples.
func (p *GoogleProvider) GenerateText(ctx context.Context, prompt string) (string, error) {
	resp, err := p.GenerateContentWithRetry(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("resposta nula do motor Google")
	}
	return resp.Text(), nil
}

// GenerateMultimodalText satisfaz a interface ContentGenerator para geração com dados binários.
func (p *GoogleProvider) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
	contents := []*genai.Content{
		{
			Parts: []*genai.Part{
				genai.NewPartFromText(prompt),
				{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     data,
					},
				},
			},
		},
	}

	resp, err := p.GenerateContentWithRetry(ctx, contents)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Text() == "" {
		return "", fmt.Errorf("resposta vazia no Google Content Gen")
	}
	return resp.Text(), nil
}
