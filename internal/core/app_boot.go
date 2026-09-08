package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/agents/acp"
	"context"
	"fmt"
	"time"
)

// bootSequence executa a inicialização dos motores em background. (DNA 1:1)
func (a *App) bootSequence() {
	// 🔌 Injeção imediata de contexto para habilitar comunicações seguras
	a.injectContexts()
	
	// ⚡ Início a Frio: Carrega o mapa instantaneamente do cache enquanto o resto inicializa
	go a.LoadFastGraph()

	time.Sleep(500 * time.Millisecond)
	a.emitBoot("config", "⚙️", "Carregando configurações...")

	if err := a.initServices(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no initServices: %v\n", err)
		a.emitBoot("error", "🔴", "Falha na inicialização: "+err.Error())
		return
	}

	// 🧠 Os serviços estão prontos. Agora podemos iniciar os agentes e o RAG.
	a.NLPReady = true 

	c, cx := a.crawler, a.ctx
	if c != nil && cx != nil {
		go func(cr *obsidian.Crawler, ct context.Context) {
			_ = cr.EnsureCollections(ct)
		}(c, cx)
	}

	if a.config != nil {
		fmt.Printf("[Boot] 🔍 Verificando Início Automático (%d agentes configurados)...\n", len(a.config.AutoStartAgents))
		if len(a.config.AutoStartAgents) > 0 {
			// ⏳ Pequeno delay para garantir que o Frontend recebeu o sinal de ONLINE
			time.Sleep(2 * time.Second)

			for _, agent := range a.config.AutoStartAgents {
				agentName := agent
				go func(name string) {
					fmt.Printf("[Boot] 🤖 Disparando início automático: %s (Workspace: %s)\n", name, a.config.ActiveWorkspace)
					if err := a.StartAgentSession(name); err == nil {
						fmt.Printf("[Boot] ✅ Agente %s iniciado e restaurado com sucesso.\n", name)
						a.emitBoot("agent", "✅", "Agente "+name+" pronto!")
					} else {
						fmt.Printf("[Boot] ❌ Falha no início automático de %s: %v\n", name, err)
					}
				}(agentName)
			}
		}

		a.emitBoot("complete", "✅", "Sistema pronto.")
	} else {
		fmt.Println("[Boot] ⚠️ Erro: Configuração nula ao tentar iniciar agentes.")
	}

	go a.startOrchestration()
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
		fmt.Printf("⚠️ Erro ao carregar configuraçao. Usando estado base para permitir recuperaçao: %v\n", err)
		cfg = &config.Config{}
	}
	a.config = cfg

	// 🛡️ RE-ARMAMENTO TARDIO: Agora que temos a config, armamos o CPI com as órbitas reais
	a.executor.CPI = acp.NewCPIValidator(cfg.ActiveWorkspace, cfg.ObsidianVaultPath)
	a.executor.Workspace = a.executor.CPI.ActiveOrbit

	if a.executor.CPI.IsArmed() {
		fmt.Printf("[Boot] 🛡️ Segurança: CPI ARMADO. Órbita: %s | Vault: %s\n", a.executor.CPI.ActiveOrbit, a.executor.CPI.VaultOrbit)
	} else {
		fmt.Println("[Boot] 🛡️ Segurança: Sistema operando em modo de amnésia total (sem órbitas definidas).")
	}

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

	// 6. Injeção Final de Contexto (Garante que todos os novos serviços tenham o "telefone" do Wails)
	a.injectContexts()

	a.emitBoot("ready", "🏛️", "Orquestrador Soberano online. Observando o Universo.")
	a.isBooted = true
	a.NLPReady = true
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
