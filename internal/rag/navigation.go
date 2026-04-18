package rag

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag/neural"
)

// GraphNavigator gerencia a expansão de contexto baseada em links com suporte a Destaque Visual.
type GraphNavigator struct {
	ctx    context.Context // Contexto persistente do Wails
	Qdrant *provider.QdrantClient
	Ranker *neural.Ranker
}

// SetContext injeta o contexto oficial do Wails.
func (n *GraphNavigator) SetContext(ctx context.Context) {
	n.ctx = ctx
}

// NewGraphNavigator inicializa o navegador com foco em Trajetória Semântica e Aprendizado Ativo.
func NewGraphNavigator(qdrant *provider.QdrantClient, ranker *neural.Ranker) *GraphNavigator {
	return &GraphNavigator{
		Qdrant: qdrant,
		Ranker: ranker,
	}
}

// ExpandContext realiza uma travessia inteligente e emite a "Trilha de Raciocínio" para o frontend.
func (n *GraphNavigator) ExpandContext(ctx context.Context, initialNotes []map[string]interface{}) []string {
	var fullContext []string
	visited := make(map[string]bool)
	visitedIds := make(map[uint64]bool)

	cfg, _ := config.Load()
	depthLimit := 1
	neighborLimit := 5
	contextLimit := 4000
	if cfg != nil {
		if cfg.GraphDepth > 0 { depthLimit = cfg.GraphDepth }
		if cfg.GraphNeighborLimit > 0 { neighborLimit = cfg.GraphNeighborLimit }
		if cfg.GraphContextLimit > 0 { contextLimit = cfg.GraphContextLimit }
	}

	totalChars := 0
	
	// 🚀 FASE 1: Processar Núcleos (Depth 0)
	for _, note := range initialNotes {
		name, _ := note["name"].(string)
		if id, ok := note["id"].(float64); ok {
			visitedIds[uint64(id)] = true
		}
		
		if name != "" {
			visited[name] = true
			content, _ := note["content"].(string)
			fullContext = append(fullContext, fmt.Sprintf("=== [NÚCLEO]: %s ===\n%s", name, content))
			totalChars += len(content)
			
			// Efeito: Acende o nó mestre
			runtime.EventsEmit(n.ctx, "node:active", name)
		}
	}

	// 🚀 FASE 2: Expansão de Vizinhança (N-Hop) guiada pelo Grafo Visual
	if depthLimit > 0 {
		type TrailHop struct {
			From   string  `json:"from"`
			To     string  `json:"to"`
			Weight float32 `json:"weight"` // Peso aprendido para visualização visual
		}
		var trail []TrailHop
		
		for _, note := range initialNotes {
			parentName, _ := note["name"].(string)
			
			if links, ok := note["links"].([]interface{}); ok {
				for _, linkIntf := range links {
					if len(trail) >= neighborLimit {
						break
					}
					
					// 🛡️ Prevenção de Panic: Os links são indexados como Nomes (String), não Qdrant IDs!
					linkName, ok := linkIntf.(string)
					if !ok {
						continue // Pula chaves antigas que fossem IDs numéricos caso base v1
					}

					if visited[linkName] {
						continue
					}
					visited[linkName] = true

					// 🔍 Busca no motor Relacional por nome
					nb, err := n.Qdrant.SearchByField("obsidian_knowledge", "name", linkName)
					if err == nil && nb != nil {
						name, _ := nb["name"].(string)
						content, _ := nb["content"].(string)

						if totalChars+len(content) > contextLimit {
							continue
						}

						fullContext = append(fullContext, fmt.Sprintf("=== [CONTEXTO_RELACIONADO]: %s ===\n%s", name, content))
						totalChars += len(content)

						// ✨ VISUAL: destaca link individual E coleta trilha com peso neural
						neuralWeight := n.Ranker.GetWeight(name)

						runtime.EventsEmit(n.ctx, "graph:highlight", map[string]interface{}{
							"source": parentName,
							"target": name,
							"weight": neuralWeight,
						})
						
						trail = append(trail, TrailHop{
							From:   parentName, 
							To:     name, 
							Weight: neuralWeight,
						})
					}
				}
			}
		}

		// 🚀 Emite o percurso completo como uma única mensagem animável no frontend
		if len(trail) > 0 {
			runtime.EventsEmit(n.ctx, "graph:traverse", map[string]interface{}{
				"hops":  trail,
				"total": len(trail),
			})
		}
	}

	// 🚀 FASE 3: Sinapses de Chat (Memória Longa)
	// (Busca os últimos fatos relevantes da sessão ou conhecimentos consolidados)
	synapses, err := n.Qdrant.Search("knowledge_graph", nil, 5) 
	if err == nil && synapses != nil {
		for _, syn := range synapses {
			fact, _ := syn["content"].(string)
			if fact != "" && totalChars + len(fact) < contextLimit {
				fullContext = append(fullContext, fmt.Sprintf("[SINAPSE]: %s", fact))
				totalChars += len(fact)
			}
		}
	} else if err != nil {
		fmt.Printf("[DEBUG-RAG] ⚠️ Falha ao buscar sinapses (ignorando): %v\n", err)
	}

	return fullContext
}
