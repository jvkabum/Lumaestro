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
	QuotaManager  *QuotaManager
}

// NewGoogleProvider inicializa o provedor Google com o pool de chaves configurado e QuotaManager integrado.
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

	qm := GetGlobalQuotaManager(keys)

	return &GoogleProvider{
		Client:        client,
		ctx:           ctx,
		keys:          keys,
		CurrentKeyIdx: 0,
		QuotaManager:  qm,
	}, nil
}

// GenerateContentWithRetry é o motor generativo unificado com Cascata de Modelos (Gemini -> Gemma), Rotação de Chaves e QuotaManager (Gesttik Engine).
func (p *GoogleProvider) GenerateContentWithRetry(ctx context.Context, contents []*genai.Content) (*genai.GenerateContentResponse, error) {
	// Super Frota Dinâmica (Lê os modelos ativos da configuração do Maestro)
	cfg, _ := config.Load()
	models := cfg.ActiveGoogleModels

	// Fallback de Segurança caso a lista esteja vazia
	// ATENÇÃO: Use apenas IDs oficiais da API. Consulte: https://ai.google.dev/gemini-api/docs/models
	if len(models) == 0 {
		models = []string{
			"gemini-3.8-flash",      // 🚀 Topo da frota — Flash 3.8
			"gemini-3.7-flash",      // ⚡ Flash 3.7
			"gemini-3.6-flash",      // ⚡ Flash 3.6
			"gemini-3.5-flash",      // 🏆 Flash 3.5 (confirmado ativo)
			"gemini-3.1-flash-lite", // 🏎️ Alta velocidade — Lite 3.1 (maior RPM)
			"gemini-3.5-flash-lite", // 🚀 Lite 3.5 (volume/escala)
			"gemini-3-flash",        // ⚖️ Flash 3 estável
			"gemini-2.5-flash",      // 🥈 Flash 2.5 (fallback testado)
			"gemini-2.5-flash-lite", // 📦 Flash Lite 2.5 (volume)
			"gemma-4-31b-it",        // 🛡️ Gemma 4 31B (resiliência open-weight)
			"gemma-4-26b-it",        // 🐘 Gemma 4 26B (reserva tática)
		}
	}

	if len(p.keys) == 0 {
		return nil, fmt.Errorf("nenhuma chave Google configurada para geração de conteúdo")
	}

	qm := p.QuotaManager
	if qm == nil {
		qm = GetGlobalQuotaManager(p.keys)
		p.QuotaManager = qm
	}

	maxFleetCycles := 3
	cycles := 0

	for {
		cycles++
		for _, model := range models {
			// Tenta as chaves disponíveis para o modelo atual com balanceamento round-robin
			for i := 0; i < len(p.keys); i++ {
				p.Mu.Lock()
				actualKeyIdx := (p.CurrentKeyIdx + i) % len(p.keys)
				p.Mu.Unlock()
				key := p.keys[actualKeyIdx]

				// Verifica se a chave+modelo está exaurida (RPD, RPM exponencial ou Circuit Breaker)
				if qm.IsExhausted(key, model) {
					continue
				}

				// Cliente usando a chave da tentativa
				client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})
				if err != nil {
					qm.HandleError(key, actualKeyIdx, model, err)
					qm.LogRequest(model, actualKeyIdx, key, "FAILED", err.Error())
					continue
				}

				masked := qm.MaskAPIKey(key)
				fmt.Printf("[ResilienceFleet] 🚀 Tentando modelo: %s (Chave [%d] %s)\n", model, actualKeyIdx+1, masked)

				temp := float32(0.0)
				genConfig := &genai.GenerateContentConfig{
					Temperature: &temp,
				}

				resp, err := client.Models.GenerateContent(ctx, model, contents, genConfig)
				if err == nil && resp != nil {
					// Sucesso: reseta cooldowns e registra log positivo
					qm.MarkSuccess(key, model)
					qm.LogRequest(model, actualKeyIdx, key, "SUCCESS", "")

					// Sincroniza o cliente de sucesso no estado do provedor
					p.Mu.Lock()
					p.CurrentKeyIdx = (actualKeyIdx + 1) % len(p.keys)
					p.Client = client
					p.Mu.Unlock()
					return resp, nil
				}

				// Registra e processa erro via QuotaManager
				qm.HandleError(key, actualKeyIdx, model, err)
				errMsg := ""
				if err != nil {
					errMsg = err.Error()
				}
				qm.LogRequest(model, actualKeyIdx, key, "FAILED", errMsg)

				if utils.IsQuotaError(err) {
					fmt.Printf("[ResilienceFleet] ⚠️ Cota exaurida no modelo %s (Chave [%d] %s). Rotacionando chave...\n", model, actualKeyIdx+1, masked)
					continue
				}

				if utils.IsSuspendedError(err) {
					fmt.Printf("[ResilienceFleet] 🚫 Chave SUSPENSA detectada ([%d] %s). Quarentena ativada.\n", actualKeyIdx+1, masked)
					continue
				}

				fmt.Printf("[ResilienceFleet] 🚩 Erro no modelo %s: %v. Pulando para o próximo modelo...\n", model, err)
				break
			}
		}

		if cycles >= maxFleetCycles {
			return nil, fmt.Errorf("falha persistente: frotas Google/Gemma falharam após %d ciclos", maxFleetCycles)
		}

		// Calcula tempo exato de cooldown pendente com o QuotaManager
		shortestWait := qm.GetShortestCooldown(models)
		if shortestWait > 60*time.Second {
			shortestWait = 60 * time.Second
		}
		if shortestWait < 5*time.Second {
			shortestWait = 5 * time.Second
		}

		fmt.Printf("⏳ [ResilienceFleet] 🚨 Todos os modelos e chaves em cooldown! Aguardando %v para liberação do próximo slot... 😴\n", shortestWait.Round(time.Second))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(shortestWait):
		}
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
