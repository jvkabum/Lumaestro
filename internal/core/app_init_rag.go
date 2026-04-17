package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/rag"
)

// initRAGInfrastructure inicializa search, navigator, weaver, crawler e sincroniza o grafo neural.
func (a *App) initRAGInfrastructure(cfg *config.Config) {
	a.emitBoot("neon", "🧠", "Sincronizando Córtex Neural...")

	search := rag.NewSearchService(a.qdrant, a.ranker)
	a.navigator = rag.NewGraphNavigator(a.qdrant, a.ranker)

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

	if a.embedder != nil && a.ontology != nil {
		a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology)
		
		// 🏗️ [CRÍTICO] Garante que as coleções existem no banco vetorial imediatamente
		go func() {
			if a.crawler != nil && a.ctx != nil {
				_ = a.crawler.EnsureCollections(a.ctx)
			}
		}()
	} else {
		a.crawler = nil
		a.emitBoot("crawler", "⚠️", "Crawler pausado: configure um provedor de embeddings na aba MODELOS.")
	}

	// Sincronização do Grafo a partir do LStore (DuckDB)
	if a.LStore != nil {
		nodes, edges, err := a.LStore.GetFullGraph()
		if err == nil {
			for _, n := range nodes {
				a.GEngine.AddNode(n["id"].(string), n["name"].(string), n["type"].(string))
			}
			for _, e := range edges {
				a.GEngine.AddEdge(e["source"].(string), e["target"].(string), e["weight"].(float64), e["relation_type"].(string))
			}
			a.GEngine.ComputePageRank()
		}
	}

	a.executor.Tools.Indexer = a.crawler
}
