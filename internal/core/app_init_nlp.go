package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// initNLPEngine centraliza a inicialização dos motores de Embeddings e Extração/Geração.
func (a *App) initNLPEngine(cfg *config.Config) (provider.ContentGenerator, error) {
	a.emitBoot("embeddings", "🧪", "Inicializando motor de Embeddings...")
	a.embedder = nil
	a.ontology = nil

	embProvider := strings.ToLower(strings.TrimSpace(cfg.EmbeddingsProvider))
	ragProvider := strings.ToLower(strings.TrimSpace(cfg.RAGProvider))
	if embProvider == "" {
		embProvider = "gemini"
	}
	if ragProvider == "" {
		ragProvider = "gemini"
	}

	// ─── Motor de Embeddings ──────────────────────────────────────────────────
	if embProvider == "lmstudio" && cfg.LMStudioEnabled && cfg.LMStudioURL != "" {
		embedModel := strings.TrimSpace(cfg.EmbeddingsModel)
		baseCtx := a.ctx
		if baseCtx == nil {
			baseCtx = context.Background()
		}

		if embedModel == "" {
			client := provider.NewLMStudioClient(cfg.LMStudioURL)
			ctxModels, cancelModels := context.WithTimeout(baseCtx, 8*time.Second)
			models, err := client.ListModels(ctxModels)
			cancelModels()
			if err == nil {
				re := regexp.MustCompile(`(?i)(embed|embedding|nomic|bge|e5|gte)`)
				for _, m := range models {
					if re.MatchString(m) {
						embedModel = m
						break
					}
				}
			}
		}

		if embedModel == "" {
			a.emitBoot("embeddings", "⚠️", "Embeddings LM Studio sem modelo válido. Configure um modelo de embedding dedicado.")
		} else {
			client := provider.NewLMStudioClient(cfg.LMStudioURL)
			ctxDim, cancelDim := context.WithTimeout(baseCtx, 12*time.Second)
			dim, err := client.DetectEmbeddingDimension(ctxDim, embedModel)
			cancelDim()
			if err != nil || dim <= 0 {
				a.emitBoot("embeddings", "⚠️", "Modelo de embeddings LM Studio inválido: "+embedModel+". Use um modelo de embedding (ex: text-embedding-nomic-embed-text-v1.5).")
			} else {
				cfg.EmbeddingsModel = embedModel
				cfg.EmbeddingDimension = dim
				a.config = cfg
				_ = config.Save(*cfg)

				lmEmb := provider.NewLMStudioEmbedder(cfg.LMStudioURL, embedModel, cfg.LMStudioModel)
				a.embedder = lmEmb
				a.emitBoot("embeddings", "✅", fmt.Sprintf("Motor de Embeddings: LM Studio (%s · %d dim)", embedModel, dim))
			}
		}
	} else if embProvider == "native" {
		if !a.installer.CheckStatus("llama-server") {
			a.emitBoot("embeddings", "🛠️", "Motor local não encontrado. Iniciando instalação via winget...")
			go func() {
				if err := a.installer.InstallLlamaCPP(); err == nil {
					a.emitBoot("embeddings", "✅", "Instalação concluída. Atualizando ambiente...")
					a.installer.SyncPath()
					time.Sleep(2 * time.Second)
					a.initServices()
				}
			}()
			return nil, fmt.Errorf("aguardando instalação do llama.cpp")
		}

		a.emitBoot("embeddings", "🧩", "Iniciando motor nativo (llama.cpp)...")
		native := provider.NewNativeEmbedder("")
		native.OnLog = func(line string) {
			a.emitBoot("embeddings", "⏳", "Baixando Memória: "+line)
		}
		if err := native.Start(); err != nil {
			a.emitBoot("embeddings", "⚠️", "Falha ao iniciar motor nativo: "+err.Error())
		} else {
			a.nativeEmbedder = native
			a.embedder = native
			a.emitBoot("embeddings", "✅", "Motor Nativo (Qwen3 0.6B) Online.")
		}
	} else {
		emb, err := provider.NewEmbeddingService(a.ctx, cfg.GetActiveGeminiKey())
		if err != nil {
			a.emitBoot("embeddings", "⚠️", "Embeddings Gemini indisponível (modo degradado): "+err.Error())
		} else {
			a.embedder = emb
			a.emitBoot("embeddings", "✅", "Motor de Embeddings: Gemini (gemini-embedding-2-preview)")
		}
	}

	// ─── Motor de RAG/Ontologia (Geração de Conteúdo) ─────────────────────────
	var contentGen provider.ContentGenerator
	if a.embedder != nil {
		if ragProvider == "lmstudio" && cfg.LMStudioEnabled && cfg.LMStudioURL != "" {
			ragModel := cfg.RAGModel
			if ragModel == "" {
				ragModel = cfg.LMStudioModel
			}
			contentGen = provider.NewLMStudioEmbedder(cfg.LMStudioURL, "", ragModel)
			a.emitBoot("rag", "✅", "Motor RAG/Ontologia: LM Studio ("+ragModel+")")
		} else if ragProvider == "native" {
			a.emitBoot("expert", "🧩", "Iniciando Especialista Claude-Distilled (Lógica Elite)...")

			// --- TIME DE ELITE 2026 (Modelos Especialistas em Extração RAG) ---
			
			// [ OPÇÕES OTIMIZADAS PARA PLACAS ANTIGAS / RX 580 ]
			// qwenModel := "mradermacher/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic-GGUF:Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic.IQ4_XS.gguf" // (Matriz de Importância - Ultra Leve 2.5GB)
			// qwenModel := "mradermacher/Qwen3-4B-Qwen3.6-plus-Reasoning-Slerp-i1-GGUF:Qwen3-4B-Qwen3.6-plus-Reasoning-Slerp.i1-Q4_K_M.gguf"
			// qwenModel := "khazarai/Qwen3-4B-Qwen3.6-plus-Reasoning-Distilled-GGUF:Q4_1"
			
			// [ OPÇÕES PESADAS / ORIGINAIS (Alta Resolução Lógica) ]
			// qwenModel := "Jackrong/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-GGUF:Qwen3.5-4B.Q5_K_M.gguf"

			// --- [ ATIVO ] Padrão de Ouro RX 580 (Equilíbrio Inteligência e Velocidade Vulkan) ---
			qwenModel := "mradermacher/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic-GGUF:Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic.Q4_K_M.gguf"

			a.emitBoot("expert", "🧪", "Lançando Especialista de Lógica Slerp (Alta Velocidade na 8086)...")
			nativeExtraction := provider.NewNativeGenerator(qwenModel, 8086, "QWEN-SLERP")
			nativeExtraction.OnLog = func(line string) {
				a.emitBoot("expert", "⏳", "Baixando Especialista: "+line)
			}

			// --- CHAT & ORQUESTRAÇÃO ---
			// Agora focado 100% no Modo ACP Cloud (Gemini/Claude via CLI) para economizar RAM.
			// Se quiser reativar o motor local de chat, descomente as linhas abaixo:
			/*
				gemmaModel := "unsloth/gemma-4-E4B-it-GGUF:gemma-4-E4B-it-Q4_K_M.gguf"
				a.emitBoot("rag", "🧪", "Lançando Revisor Linguístico (Gemma 4 na 8087)...")
				nativeGeneral := provider.NewNativeGenerator(gemmaModel, 8087, "GEMMA-4")
				nativeGeneral.OnLog = func(line string) {
					a.emitBoot("rag", "⏳", "Baixando Linguística: "+line)
					runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
						"source":  "GEMMA-4",
						"content": "📥 " + line,
					})
				}
			*/

			if err := nativeExtraction.Start(); err == nil {
				a.emitBoot("expert", "✅", "Especialista Claude-Distilled (Qwen 3.5 Q5) ONLINE")
				a.nativeExtraction = nativeExtraction
				contentGen = nativeExtraction
			}
		} else if ragProvider == "gemini" || ragProvider == "" {
			if gemEmb, ok := a.embedder.(*provider.EmbeddingService); ok {
				contentGen = gemEmb
				a.emitBoot("rag", "✅", "Motor RAG/Ontologia: Gemini (cascata)")
			} else if ragProvider == "gemini" {
				gemSvc, err := provider.NewEmbeddingService(a.ctx, cfg.GetActiveGeminiKey())
				if err == nil {
					contentGen = gemSvc
					a.emitBoot("rag", "✅", "Motor RAG/Ontologia: Gemini (serviço dedicado)")
				}
			}
		}
	}

	if contentGen != nil {
		a.ontology = provider.NewOntologyService(a.ctx, contentGen)
	} else {
		a.emitBoot("rag", "⚠️", "Motor RAG/Ontologia indisponível — sem motor generativo configurado")
	}

	return contentGen, nil
}
