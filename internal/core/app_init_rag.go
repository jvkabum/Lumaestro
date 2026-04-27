package core

import (
	"context"
	"fmt"
	"Lumaestro/internal/config"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/rag"
)

// initRAGInfrastructure inicializa search, navigator, weaver, crawler e sincroniza o grafo neural.
func (a *App) initRAGInfrastructure(cfg *config.Config) {
	a.emitBoot("neon", "🧠", "Sincronizando Córtex Neural...")

	search := rag.NewSearchService(a.qdrant, a.ranker)
	a.navigator = rag.NewGraphNavigatorV2(a.qdrant, a.ranker, a.LStore)

	if a.embedder != nil && a.ontology != nil {
		a.weaver = rag.NewKnowledgeWeaver(a.ontology, a.qdrant, a.embedder)
	} else {
		a.weaver = nil
	}

	a.emitBoot("chat", "🎭", "Orquestrando serviços de Chat e RAG...")
	a.chat = rag.NewChatService(a.legacyExec, a.orchestrator, search, a.navigator, a.embedder, a.installer)

	a.emitBoot("crawler", "🕸️", "Tecendo o Crawler do Obsidian...")
	a.Validator = rag.NewAgentValidator(a.LStore, a.GEngine)
	a.Recon = rag.NewAgentRecon(a.LStore, a.GEngine, a.qdrant)

	// 🕸️ Inicializa o Crawler (Pode rodar em modo degradado sem IA para Fase 1)
	a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology, a.LStore)
	
	if a.embedder == nil || a.ontology == nil {
		a.emitBoot("crawler", "⚠️", "Crawler em modo degradado: Somente estrutura de arquivos (IA offline).")
	}

	// 🏗️ [CRÍTICO] Garante que as coleções existem no banco vetorial de forma assíncrona
	go func(ctx context.Context) {
		if a.crawler != nil && ctx != nil {
			if err := a.crawler.EnsureCollections(ctx); err != nil {
				fmt.Printf("[RAG-INIT] ❌ Erro crítico ao preparar coleções Qdrant: %v\n", err)
			} else {
				fmt.Println("[RAG-INIT] ✅ Coleções vetoriais prontas.")
			}
		}
	}(a.ctx)

	// Sincronização do Grafo a partir do LStore (DuckDB)
	if a.LStore != nil {
		// 🧠 Lógica de Path: Prioriza o Workspace ativo, fallback para o Vault principal
		workspacePath := a.executor.Workspace
		if workspacePath == "" {
			workspacePath = cfg.ObsidianVaultPath
		}

		nodes, edges, err := a.LStore.GetFullGraph(workspacePath)
		if err == nil {
			fmt.Printf("[RAG-INIT] 🧬 Sincronizando %d nós do DuckDB (Path: %s)\n", len(nodes), workspacePath)
			
			for _, n := range nodes {
				// 🔒 Proteger type assertions (CRÍTICO)
				id, ok1 := n["id"].(string)
				name, ok2 := n["name"].(string)
				typ, ok3 := n["type"].(string)
				
				if ok1 && ok2 && ok3 {
					a.GEngine.AddNode(id, name, typ)
				}
			}
			
			for _, e := range edges {
				src, ok1 := e["source"].(string)
				tgt, ok2 := e["target"].(string)
				w, ok3 := e["weight"].(float64)
				rel, ok4 := e["relation_type"].(string)
				
				if ok1 && ok2 && ok3 && ok4 {
					a.GEngine.AddEdge(src, tgt, w, rel)
				}
			}
			a.GEngine.ComputePageRank()
		} else {
			fmt.Printf("[RAG-INIT] ⚠️ Falha ao carregar grafo do LStore: %v\n", err)
		}
	}

	a.executor.Tools.Indexer = a.crawler
}
