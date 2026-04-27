package rag

import (
	"Lumaestro/internal/lightning"
	"Lumaestro/internal/provider"
	"context"
	"fmt"
	"strings"
)

// AgentRecon é o sentinela pró-ativo que encontra conexões perdidas.
type AgentRecon struct {
	Store  *lightning.DuckDBStore
	Graph  *GraphEngine
	Qdrant *provider.QdrantClient
}

// NewAgentRecon cria uma nova instância do sentinela.
func NewAgentRecon(store *lightning.DuckDBStore, graph *GraphEngine, qdrant *provider.QdrantClient) *AgentRecon {
	return &AgentRecon{
		Store:  store,
		Graph:  graph,
		Qdrant: qdrant,
	}
}

// ReconProposal representa uma sugestão de sinapse.
type ReconProposal struct {
	SourceID   string  `json:"source_id"`
	TargetID   string  `json:"target_id"`
	Similarity float64 `json:"similarity"`
	Auto       bool    `json:"auto"` // Se true, foi conectada automaticamente
}

// ScanMissingLinks procura por notas similares que não possuem conexão no grafo dentro de um workspace.
func (r *AgentRecon) ScanMissingLinks(ctx context.Context, workspacePath string) ([]ReconProposal, error) {
	if r.Qdrant == nil || r.Graph == nil {
		return nil, fmt.Errorf("infraestrutura incompleta")
	}

	// 1. Pegar alguns nós aleatórios ou de alto rank para auditar
	// [SIMPLIFICADO]: Audita os últimos 50 pontos indexados no Qdrant
	points, err := r.Qdrant.Search("obsidian_knowledge", nil, 50)
	if err != nil {
		return nil, err
	}

	var proposals []ReconProposal

	for _, p := range points {
		// 🛡️ Ajuste Mixer: Usar o ID real (moon:hash:nome) em vez de apenas o nome
		sourceID, _ := p["id"].(string)
		if sourceID == "" {
			sourceID = strings.ToLower(p["name"].(string))
		}
		
		// 2. Procurar vizinhos semânticos no Qdrant (K=5)
		// NOTA: Passamos nil aqui para listar outros pontos relevantes da coleção enquanto
		// a extração do vetor original de 'p' não é implementada.
		neighbors, err := r.Qdrant.Search("obsidian_knowledge", nil, 5)
		if err != nil { continue }

		for _, n := range neighbors {
			targetID, _ := n["id"].(string)
			if targetID == "" {
				targetID = strings.ToLower(n["name"].(string))
			}
			if sourceID == targetID { continue }

			// 3. Verificar se já existe conexão no Cérebro Relacional (Go-Engine)
			// Se a distância for > 1 (ou seja, não há link direto), é um candidato.
			if !r.hasDirectEdge(sourceID, targetID) {
				sim := 0.85 // Exemplo: Pegaríamos o score real do Qdrant aqui
				
				// Lógica Mística (Misto: Auto/Manual)
				auto := sim > 0.95
				
				proposals = append(proposals, ReconProposal{
					SourceID:   sourceID,
					TargetID:   targetID,
					Similarity: sim,
					Auto:       auto,
				})

				// Se for auto, já injeta no grafo imediatamente!
				if auto {
					r.Graph.AddEdge(sourceID, targetID, sim, "recon_auto")
					if r.Store != nil {
						r.Store.InsertGraphEdge(workspacePath, sourceID, targetID, sim, "recon_auto")
					}
				}
			}
		}
	}

	return proposals, nil
}

func (r *AgentRecon) hasDirectEdge(s, t string) bool {
    // [LOGICA SIMPLIFICADA]: Checa a lista de adjacência do GraphEngine
	// Precisamos expor um método no GraphEngine para isso ou acessar internamente
	return false // Fallback
}
