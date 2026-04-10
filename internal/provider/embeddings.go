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

// EmbeddingService gerencia a geração de vetores e conteúdo via Gemini com suporte a pool de chaves e cascata de modelos.
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
		return nil, fmt.Errorf("falha ao criar cliente GenAI: %w", err)
	}

	return &EmbeddingService{
		Client:        client,
		ctx:           ctx,
		keys:          keys,
		CurrentKeyIdx: 0,
	}, nil
}

// rotateAndRetry tenta rotacionar a chave e recriar o client.
// Retorna true se conseguiu rotacionar, false se não há mais chaves.
func (s *EmbeddingService) rotateAndRetry() bool {
	cfg, err := config.Load()
	if err != nil || cfg == nil || cfg.GeminiKeyCount() <= 1 {
		return false
	}

	newKey := cfg.RotateGeminiKey()
	if newKey == "" {
		return false
	}

	// Recria o client com a nova chave
	newClient, err := genai.NewClient(s.ctx, &genai.ClientConfig{
		APIKey:  newKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Printf("[KeyPool] ❌ Falha ao criar client com nova chave: %v\n", err)
		return false
	}

	s.Client = newClient
	fmt.Printf("[KeyPool] ✅ Client recriado com nova chave.\n")
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

// GenerateMultimodalEmbedding transforma um binário (imagem, PDF, etc) em um vetor []float32.
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

// embedWithRetry é o motor central que realiza a chamada e gerencia a rotação de chaves com Throttle defensivo.
func (s *EmbeddingService) embedWithRetry(ctx context.Context, contents []*genai.Content, fastTrack bool) ([]float32, error) {
	model := "gemini-embedding-2-preview"

	for {
		res, err := s.Client.Models.EmbedContent(ctx, model, contents, nil)
		if err == nil {
			if len(res.Embeddings) > 0 && res.Embeddings[0] != nil && len(res.Embeddings[0].Values) > 0 {
				return res.Embeddings[0].Values, nil
			}
			return nil, fmt.Errorf("vetor de embedding vazio na resposta")
		}

		if !utils.IsQuotaError(err) && !utils.IsSuspendedError(err) {
			return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
		}

		if utils.IsSuspendedError(err) {
			fmt.Printf("[KeyPool] 🚫 Chave atual SUSPENSA detectada. Tentando rotacionar...\n")
		} else {
			fmt.Printf("[KeyPool] ⚠️ Chave atual exausta (quota). Tentando rotacionar...\n")
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
			fmt.Printf("[KeyPool] ⚠️ Chave #%d também exausta: %s\n", i+2, utils.FormatGenAIError(err))
		}

		if rotatedSuccess {
			if len(res.Embeddings) > 0 && res.Embeddings[0] != nil && len(res.Embeddings[0].Values) > 0 {
				return res.Embeddings[0].Values, nil
			}
			return nil, fmt.Errorf("vetor de embedding vazio na resposta")
		}

		// Fast-Track: Se todas as chaves falharam e estamos em modo fastTrack (Chat), abortamos imediatamente sem hibernar.
		if fastTrack {
			return nil, fmt.Errorf("quota_exhausted: todas as chaves exaustas (fast-track)")
		}

		// Throttle Defensivo: Quando todas as N chaves falham de cota, em vez de pular o arquivo e abortar,
		// O Backend bloqueia/hiberna por 30s. Isso perfeitamente alinha com a janela RPM de reset da API do Gemini.
		fmt.Println("⏳ [KeyPool] 🚨 Todas as chaves bateram o limite! Entrando em hibernação forçada de 30s para não perder arquivos... 😴")
		time.Sleep(30 * time.Second)
		fmt.Println("⚡ [KeyPool] Acordando da hibernação. Retomando embeddings do ponto em que parou...")
	}
}

// GenerateContentWithRetry é o motor generativo unificado para textos e multimídia com Cascata de Modelos (Gemini -> Gemma) e Rotação de Chaves.
func (s *EmbeddingService) GenerateContentWithRetry(ctx context.Context, contents []*genai.Content) (*genai.GenerateContentResponse, error) {
	// Super Frota Atualizada (Padrão Junho/2026): Resiliência Extrema
	models := []string{
		"gemini-3.1-flash-lite-preview", // 🚀 Velocidade de Triplas (Lite 3.1)
		"gemini-2.5-flash",              // 🏆 Capitão Confirmado (Flash 2.5)
		"gemini-3-flash-preview",        // ⚖️ Moderno (Flash 3)
		"gemini-2.5-flash-lite",         // 📦 Escala de Volume
		"gemma-4-31b-it",                // 🛡️ O Tanque (Resiliência)
		"gemma-4-26b-a4b-it",            // 🐘 Reserva Tática
	}

	if len(s.keys) == 0 {
		return nil, fmt.Errorf("nenhuma chave Gemini configurada para geração de conteúdo")
	}

	maxFleetCycles := 3
	cycles := 0

	for {
		cycles++
		for _, model := range models {
			// Tenta todas as chaves disponíveis para o modelo atual
			for i, key := range s.keys {
				// Garantir que o cliente usa a chave correta para esta tentativa
				client, _ := genai.NewClient(ctx, &genai.ClientConfig{APIKey: key, Backend: genai.BackendGeminiAPI})

				fmt.Printf("[ResilienceFleet] 🚀 Tentando modelo: %s (Chave %d/%d)\n", model, i+1, len(s.keys))

				resp, err := client.Models.GenerateContent(ctx, model, contents, nil)
				if err == nil {
					// Sincroniza a chave de sucesso no estado do serviço para próximas chamadas rápidas
					s.Mu.Lock()
					s.CurrentKeyIdx = i
					s.Client = client
					s.Mu.Unlock()
					return resp, nil
				}

				// Se for erro de cota (429), tenta a próxima CHAVE para o MESMO modelo
				if utils.IsQuotaError(err) {
					fmt.Printf("[ResilienceFleet] ⚠️ Cota exaurida no modelo %s (Chave %d). Rotacionando chave...\n", model, i+1)
					continue
				}

				// 🧠 Se a chave foi suspensa (403), pula para o próximo MODELO e avisa
				if err != nil && (err.Error() == "PERMISSION_DENIED" || utils.IsSuspendedError(err)) {
					fmt.Printf("[ResilienceFleet] 🚫 Chave SUSPENSA detectada (%d). Pulando para o próximo modelo...\n", i+1)
					break 
				}

				// 🚩 Erro genérico (404, 500, etc), PULA para o próximo MODELO na frota
				fmt.Printf("[ResilienceFleet] 🚩 Erro no modelo %s: %v. Pulando para o próximo modelo na cascata...\n", model, err)
				break 
			}
		}

		// Hibernação Defensiva: Se todos os modelos em todas as chaves falharem
		if cycles >= maxFleetCycles {
			return nil, fmt.Errorf("falha persistente: todos os modelos/chaves Gemini falharam após %d ciclos", maxFleetCycles)
		}
		fmt.Println("⏳ [ResilienceFleet] 🚨 Todos os modelos e chaves falharam! Hibernação de 30s... 😴")
		time.Sleep(30 * time.Second)
		fmt.Println("⚡ [ResilienceFleet] Acordando. Reiniciando ciclo de cascata...")
	}
}

// GenerateText satisfaz a interface ContentGenerator para geração de texto simples.
func (s *EmbeddingService) GenerateText(ctx context.Context, prompt string) (string, error) {
	resp, err := s.GenerateContentWithRetry(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if resp == nil {
		return "", fmt.Errorf("resposta nula do motor generativo")
	}
	return resp.Text(), nil
}

// GenerateMultimodalText satisfaz a interface ContentGenerator para geração com dados binários (imagens, PDFs).
func (s *EmbeddingService) GenerateMultimodalText(ctx context.Context, prompt string, data []byte, mimeType string) (string, error) {
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

	resp, err := s.GenerateContentWithRetry(ctx, contents)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.Text() == "" {
		return "", fmt.Errorf("resposta vazia no GenerateMultimodalText")
	}
	return resp.Text(), nil
}
