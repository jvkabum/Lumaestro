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

	// 🛑 PROTOCOLO DE LIMPEZA: Mata motores nativos anteriores para liberar VRAM/RAM
	if a.nativeExtraction != nil {
		fmt.Println("[NLP] 🧹 Liberando memória: Encerrando motor nativo anterior...")
		
		// O método Stop() agora aguarda ativamente a morte do processo (cmd.Wait())
		if err := a.nativeExtraction.Stop(); err != nil {
			fmt.Printf("[NLP] ❌ Erro ao parar motor nativo: %v\n", err)
		} else {
			fmt.Println("[NLP] ✅ Memória VRAM liberada com sucesso.")
		}
		
		a.nativeExtraction = nil
	}

	fmt.Printf("[NLP] 🔍 Inicializando motores... RAG Alvo: %s | Embeddings Alvo: %s\n", ragProvider, embProvider)

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
					a.emitBoot("embeddings", "✅", "Instalação concluída. O motor nativo será ativado na próxima reinicialização ou via HMR.")
					a.installer.SyncPath()
					time.Sleep(2 * time.Second)
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
		fmt.Printf("[NLP] 🛡️ Verificando Modo Sobrevivência (Failover)... Alvo: %s\n", ragProvider)
		if cfg.HybridFailoverEnabled && len(cfg.FailoverPriority) > 0 {
			a.emitBoot("expert", "🛡️", "Modo Sobrevivência ATIVO: Inicializando Cascata de Resiliência...")
			
			cascade := provider.NewCascadeProvider(func(from, to, reason string) {
				fmt.Printf("[RAG] ⚠️ FAILOVER: %s ➡ %s (Motivo: %s)\n", from, to, reason)
				a.emitBoot("expert", "⚠️", fmt.Sprintf("Failover disparado: %s ➤ %s (Motivo: %s)", from, to, reason))
			})

			// 🎯 Respeita a escolha do usuário: rag_provider é SEMPRE o primeiro da cascata
			priority := cfg.FailoverPriority
			if ragProvider != "" {
				// Garante que o provedor selecionado está no topo
				reordered := []string{ragProvider}
				for _, p := range priority {
					if strings.ToLower(strings.TrimSpace(p)) != ragProvider {
						reordered = append(reordered, p)
					}
				}
				priority = reordered
				a.emitBoot("expert", "🎯", fmt.Sprintf("Cascata reordenada: %s é o motor principal (seleção do usuário)", strings.ToUpper(ragProvider)))
			}

			for _, provName := range priority {
				pName := strings.ToLower(strings.TrimSpace(provName))
				switch pName {
				case "groq":
					activeKey := cfg.GetActiveGroqKey()
					if activeKey != "" {
						groq := provider.NewGroqProvider(activeKey, cfg.GroqModel)
						cascade.Add("GROQ", groq)
						a.emitBoot("expert", "🏎️", "Cascata: Groq LPU adicionada ao pool.")
					}
				case "gemini":
					gemProv, err := provider.NewGoogleProvider(a.ctx, cfg.GetActiveGeminiKey())
					if err == nil {
						cascade.Add("GEMINI", gemProv)
						a.emitBoot("expert", "⚡", "Cascata: Google Resilience Fleet adicionada.")
					} else {
						a.emitBoot("expert", "⚠️", "Falha ao iniciar Google Fleet: "+err.Error())
					}
				case "native":
					qwenModel := "mradermacher/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic-GGUF:Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic.Q4_K_M.gguf"
					a.emitBoot("expert", "🧩", "Cascata: Preparando motor Local (Standby)...")
					nativeExtraction := provider.NewNativeGenerator(qwenModel, 8086, "QWEN-LOCAL")
					nativeExtraction.OnLog = func(line string) {
						a.emitBoot("expert", "⏳", "Especialista: "+line)
					}
					if err := nativeExtraction.Start(); err == nil {
						a.nativeExtraction = nativeExtraction
						cascade.Add("NATIVE", nativeExtraction)
						a.emitBoot("expert", "✅", "Cascata: Motor Local ONLINE (Especialista pronto).")
					}
				case "lmstudio":
					if cfg.LMStudioEnabled && cfg.LMStudioURL != "" {
						ragModel := cfg.RAGModel
						if ragModel == "" { ragModel = cfg.LMStudioModel }
						lms := provider.NewLMStudioEmbedder(cfg.LMStudioURL, "", ragModel)
						cascade.Add("LMSTUDIO", lms)
						a.emitBoot("expert", "🤖", "Cascata: LM Studio adicionado ao pool.")
					}
				}
			}
			contentGen = cascade
			fmt.Printf("[RAG] 🎯 Cascata final: %v (Motor principal: %s)\n", cascade.Names, strings.ToUpper(ragProvider))
		} else if ragProvider == "lmstudio" && cfg.LMStudioEnabled && cfg.LMStudioURL != "" {
			ragModel := cfg.RAGModel
			if ragModel == "" {
				ragModel = cfg.LMStudioModel
			}
			contentGen = provider.NewLMStudioEmbedder(cfg.LMStudioURL, "", ragModel)
			a.emitBoot("rag", "✅", "Motor RAG/Ontologia: LM Studio ("+ragModel+")")
		} else if ragProvider == "native" {
			a.emitBoot("expert", "🧩", "Iniciando Especialista Claude-Distilled (Lógica Elite)...")

			// --- MOTOR NATIVO DINÂMICO (Escolha do Usuário) ---
			modelToUse := cfg.RAGModel
			if modelToUse == "" {
				modelToUse = "ozgurpolat/gemma-4-E2B-it-text-only-GGUF:Q4_K_M" // Default se estiver vazio
			}

			// --- TIME DE ELITE 2026 (Modelos Especialistas em Extração RAG) ---
			
			// [ OPÇÕES OTIMIZADAS PARA PLACAS ANTIGAS / RX 580 ]
			// qwenModel := "mradermacher/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic-GGUF:Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic.IQ4_XS.gguf" // (Matriz de Importância - Ultra Leve 2.5GB)
			// qwenModel := "mradermacher/Qwen3-4B-Qwen3.6-plus-Reasoning-Slerp-i1-GGUF:Qwen3-4B-Qwen3.6-plus-Reasoning-Slerp.i1-Q4_K_M.gguf"
			// qwenModel := "khazarai/Qwen3-4B-Qwen3.6-plus-Reasoning-Distilled-GGUF:Q4_1"
			
			// [ OPÇÕES PESADAS / ORIGINAIS (Alta Resolução Lógica) ]
			// qwenModel := "Jackrong/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-GGUF:Qwen3.5-4B.Q5_K_M.gguf"

			// --- [ ATIVO ] Padrão de Ouro RX 580 (Equilíbrio Inteligência e Velocidade Vulkan) ---
			// qwenModel := "mradermacher/Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic-GGUF:Qwen3.5-4B-Claude-4.6-Opus-Reasoning-Distilled-v2-heretic.Q4_K_M.gguf"

			a.emitBoot("expert", "🧪", "Lançando Especialista de Lógica (Motor: "+modelToUse+")...")
			nativeExtraction := provider.NewNativeGenerator(modelToUse, 8086, "NATIVE-EXPERT")
			
			progressRe := regexp.MustCompile(`(\d+\.?\d*)%`)
			
			nativeExtraction.OnLog = func(line string) {
				a.emitBoot("expert", "⏳", "Motor Local: "+line)
				
				// 📡 Envia progresso estruturado apenas se o regex capturar uma porcentagem válida
				if match := progressRe.FindStringSubmatch(line); len(match) > 1 && a.ctx != nil {
					a.emitEvent("native:progress", map[string]interface{}{
						"model":    modelToUse,
						"progress": match[0], // "45.5%"
						"value":    match[1], // "45.5"
						"stage":    "downloading",
					})
				}
			}

			// --- CHAT & ORQUESTRAÇÃO ---
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
				a.emitBoot("expert", "✅", "Especialista Nativo (RAG Engine) ONLINE")
				a.nativeExtraction = nativeExtraction
				contentGen = nativeExtraction
			}
		} else if ragProvider == "groq" {
			activeKey := cfg.GetActiveGroqKey()
			if activeKey == "" {
				a.emitBoot("expert", "⚠️", "Chave API Groq ausente ou vazia. Por favor, insira nas configurações.")
			} else {
				groqModel := cfg.GroqModel
				if groqModel == "" {
					groqModel = "llama-3.3-70b-versatile"
				}
				contentGen = provider.NewGroqProvider(activeKey, groqModel)
				a.emitBoot("expert", "✅", fmt.Sprintf("Especialista Groq LPU (%s) ONLINE (Key #%d)", groqModel, cfg.GroqKeyIndex+1))
			}
		} else if ragProvider == "gemini" || ragProvider == "" {
			gemProv, err := provider.NewGoogleProvider(a.ctx, cfg.GetActiveGeminiKey())
			if err == nil {
				contentGen = gemProv
				a.emitBoot("rag", "✅", "Motor RAG/Ontologia: Google Resilience Fleet (Gemini/Gemma)")
			} else {
				a.emitBoot("rag", "⚠️", "Falha ao iniciar motor Google: "+err.Error())
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
