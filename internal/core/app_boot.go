package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"fmt"
	"time"
)

// bootSequence executa a inicialização dos motores em background. (DNA 1:1)
func (a *App) bootSequence() {
	time.Sleep(1 * time.Second)
	a.emitBoot("config", "⚙️", "Carregando configurações...")

	if err := a.initServices(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no initServices: %v\n", err)
		a.emitBoot("error", "🔴", "Falha na inicialização: "+err.Error())
		return
	}

	a.injectContexts()

	if a.crawler != nil && a.ctx != nil {
		go func() {
			_ = a.crawler.EnsureCollections(a.ctx)
		}()
	}

	if a.config != nil {
		if len(a.config.AutoStartAgents) > 0 {
			for _, agent := range a.config.AutoStartAgents {
				a.emitBoot("agent", "🤖", "Iniciando agente "+agent+"...")
				go func(agentName string) {
					if err := a.StartAgentSession(agentName); err == nil {
						time.Sleep(1 * time.Second)
						a.emitEvent("agent:log", map[string]string{
							"source": "SYSTEM", "type": "system",
							"content": "🟢 Sessão '" + agentName + "' pronta.",
						})
					}
				}(agent)
			}
		}

		if a.crawler != nil && a.config.ObsidianVaultPath != "" {
			a.emitBoot("scan", "🚀", "Sincronizando conhecimento...")
			go func() {
				a.ScanVault()
				a.emitBoot("complete", "✅", "Sincronização concluída.")
			}()
		}
		go a.startOrchestration()
	}
}

// initServices orquestra a inicialização fragmentada de todos os serviços.
func (a *App) initServices() error {
	if a.isBooted {
		return nil
	}

	a.muInit.Lock()
	defer a.muInit.Unlock()

	// Dupla verificação após o lock para evitar race condition
	if a.isBooted {
		return nil
	}

	// Limpeza Pesada: Apenas se ainda não estivermos "bootados"
	a.installer.KillOrphans()

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		return nil
	}
	a.config = cfg

	// 1. LM Studio
	if cfg.LMStudioEnabled && cfg.LMStudioURL != "" {
		a.lmStudio = provider.NewLMStudioClient(cfg.LMStudioURL)
	}

	if a.crawler != nil {
		return nil
	}

	// 2. Banco Vetorial
	a.emitBoot("qdrant", "📡", "Conectando ao Qdrant...")
	a.qdrant = provider.NewQdrantClient(cfg.QdrantURL, cfg.QdrantAPIKey)

	// 3. NLP & Motores de Geração (app_init_nlp.go)
	_, err = a.initNLPEngine(cfg)
	if err != nil {
		return err
	}

	// 4. Infraestrutura RAG & Grafo (app_init_rag.go)
	a.initRAGInfrastructure(cfg)

	// 5. Analytics & Lightning (app_init_lightning.go)
	a.initLightningAnalytics(cfg)

	a.emitBoot("ready", "✅", "Maestro pronto.")
	a.isBooted = true
	return nil
}

// resetServicesForReload anula serviços para forçar reinicialização.
func (a *App) resetServicesForReload() {
	a.muInit.Lock()
	defer a.muInit.Unlock()
	a.isBooted = false
	a.crawler = nil
	a.qdrant = nil
	a.embedder = nil
	a.chat = nil
	a.weaver = nil
	a.navigator = nil
	a.lmStudio = nil
}
