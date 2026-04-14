package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
)

func (a *App) ensureLMStudioClient() (*provider.LMStudioClient, *config.Config, error) {
	cfg := a.config
	if cfg == nil {
		loaded, err := config.Load()
		if err != nil {
			return nil, nil, fmt.Errorf("falha ao carregar configuração: %v", err)
		}
		cfg = loaded
		a.config = loaded
	}

	if cfg == nil {
		return nil, nil, fmt.Errorf("configuração ausente")
	}

	url := strings.TrimSpace(cfg.LMStudioURL)
	if url == "" {
		return nil, cfg, fmt.Errorf("URL do LM Studio vazia")
	}

	// Se a URL mudou ou o cliente ainda não existe, recria na hora para evitar estado stale.
	targetBase := strings.TrimRight(url, "/")
	if a.lmStudio == nil || a.lmStudio.BaseURL != targetBase {
		a.lmStudio = provider.NewLMStudioClient(url)
	}

	// Mantém o estado habilitado coerente quando há uso explícito do LM Studio.
	if !cfg.LMStudioEnabled {
		cfg.LMStudioEnabled = true
		a.config = cfg
		_ = config.Save(*cfg)
	}

	return a.lmStudio, cfg, nil
}

// ListLMStudioModels retorna os modelos disponíveis no servidor LM Studio.
func (a *App) ListLMStudioModels() []string {
	client, _, err := a.ensureLMStudioClient()
	if err != nil {
		fmt.Printf("[LMStudio] Model list indisponível: %v\n", err)
		return []string{}
	}

	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	models, err := client.ListModels(ctx)
	if err != nil {
		fmt.Printf("[LMStudio] Erro ao listar modelos: %v\n", err)
		return []string{}
	}
	return models
}

// DetectLMStudioEmbeddingDimension testa o modelo no endpoint de embeddings e retorna a dimensão detectada.
// Retorna 0 quando falha (modelo inválido para embeddings, endpoint indisponível, etc).
func (a *App) DetectLMStudioEmbeddingDimension(model string) int {
	model = strings.TrimSpace(model)
	if model == "" {
		return 0
	}

	client, _, err := a.ensureLMStudioClient()
	if err != nil {
		fmt.Printf("[LMStudio] Detect dimension indisponível: %v\n", err)
		return 0
	}

	ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()

	dim, err := client.DetectEmbeddingDimension(ctx, model)
	if err != nil {
		fmt.Printf("[LMStudio] Falha ao detectar dimensão para %s: %v\n", model, err)
		return 0
	}

	return dim
}

// TestLMStudioModel executa o conjunto de testes de capacidade no modelo indicado.
// Retorna um mapa com: success, model_id, latency_ms, capabilities, warnings, error.
func (a *App) TestLMStudioModel(url string, model string) map[string]interface{} {
	if url == "" {
		url = "http://localhost:1234"
	}
	client := provider.NewLMStudioClient(url)

	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()

	result := client.TestModel(ctx, model)

	return map[string]interface{}{
		"success":      result.Success,
		"model_id":     result.ModelID,
		"latency_ms":   result.LatencyMs,
		"capabilities": result.Capabilities,
		"warnings":     result.Warnings,
		"error":        result.ErrorMsg,
	}
}

// LMStudioChat envia uma mensagem ao LM Studio e emite a resposta via evento de streaming.
// Chamado ex-interno por SendAgentInput quando o agente é "lmstudio".
func (a *App) lmStudioChat(sessionID string, prompt string) {
	client, cfg, err := a.ensureLMStudioClient()
	if err != nil {
		a.emitEvent("agent:log", map[string]string{
			"source":  "LMSTUDIO",
			"content": "❌ LM Studio indisponível: " + err.Error(),
		})
		return
	}

	sysprompt := fmt.Sprintf(
		"You are a powerful AI assistant integrated into Lumaestro. Answer in %s. Be concise and helpful.",
		func() string {
			if cfg != nil && cfg.AgentLanguage != "" {
				return cfg.AgentLanguage
			}
			return "Português do Brasil"
		}(),
	)

	model := ""
	if cfg != nil {
		model = cfg.LMStudioModel
	}

	ctx, cancel := context.WithTimeout(a.ctx, 120*time.Second)
	defer cancel()

	a.emitEvent("agent:profile", map[string]string{
		"name":   "LM Studio",
		"engine": "lmstudio",
	})

	response, err := client.Chat(ctx, model, sysprompt, prompt)
	if err != nil {
		a.emitEvent("agent:log", map[string]string{
			"source":  "LMSTUDIO",
			"content": "❌ Erro no LM Studio: " + err.Error(),
		})
		return
	}

	// Emite a resposta completa (sem streaming nativo, pois LM Studio não requer SSE aqui)
	a.emitEvent("agent:log", map[string]string{
		"source":  "LMSTUDIO",
		"content": response,
	})
	a.emitEvent("agent:turn_complete", "lmstudio")
	a.emitEvent("agent:done", map[string]string{
		"session": sessionID,
	})
}
