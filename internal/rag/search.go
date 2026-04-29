package rag

import (
	"context"
	"sort"
	"strings"

	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag/neural"
)

// SearchService gerencia a recuperação de conhecimento inteligente (Vítreo/Gráfico).
type SearchService struct {
	Qdrant *provider.QdrantClient
	Ranker *neural.Ranker
}

// NewSearchService inicializa o buscador avançado com suporte a Re-Ranking Neural.
func NewSearchService(qdrant *provider.QdrantClient, ranker *neural.Ranker) *SearchService {
	return &SearchService{
		Qdrant: qdrant,
		Ranker: ranker,
	}
}

// SearchNote realiza uma busca híbrida em múltiplas coleções (Arquivos + Memórias) e aplica Re-Ranking, restrita às órbitas autorizadas.
func (s *SearchService) SearchNote(ctx context.Context, vector []float32, allowedPaths []string, limit int) ([]map[string]interface{}, error) {
	// 1. Busca Paralela em Coleções Distintas (Arquivos vs Memórias)
	const oversampleFactor = 4 // Aumentado para compensar filtros de órbita
	searchLimit := limit * oversampleFactor

	// Busca 1: Obsidian Vault (Código/Notas)
	obsidianResults, _ := s.Qdrant.SearchWithScores("obsidian_knowledge", vector, searchLimit)
	
	// Busca 2: Memórias de Chat (Sinapses)
	memoryResults, _ := s.Qdrant.SearchWithScores("knowledge_graph", vector, searchLimit)

	// 2. Unificação e Normalização de Payloads
	type RankedNode struct {
		Payload map[string]interface{}
		Score   float64
	}
	var ranked []RankedNode

	processResults := func(results []map[string]interface{}, isMemory bool) {
		for _, res := range results {
			vecScore, _ := res["_score"].(float64)

			// FILTRO DE SOBERANIA MULTI-ÓRBITA (Arquivos E Memórias)
			if len(allowedPaths) > 0 {
				nodePath, _ := res["path"].(string)      // Para arquivos
				wsPath, _ := res["workspace_path"].(string) // Para memórias/sinapses
				
				targetPath := nodePath
				if isMemory && wsPath != "" {
					targetPath = wsPath
				}

				if targetPath != "" {
					found := false
					for _, p := range allowedPaths {
						if strings.HasPrefix(targetPath, p) {
							found = true
							break
						}
					}
					if !found {
						continue
					}
				} else if isMemory {
					// Se a memória não tem workspace_path, tratamos como global (opcional)
					// Por segurança estrita, poderíamos dar 'continue' aqui também.
				}
			} else {
				// MODO CEGO: Bloqueia TUDO se não houver órbitas autorizadas
				continue
			}
			
			// Normalização de campos para o Grafo (Memórias usam Subject, Notas usam Name)
			if isMemory {
				if subj, ok := res["subject"].(string); ok {
					res["name"] = subj // Mapeia para name para consistência na UI
				}
				res["document-type"] = "memory"
			} else {
				// Garante que o tipo do documento esteja presente se falhar no payload
				if res["document-type"] == nil {
					res["document-type"] = "chunk"
				}
			}

			// Boost de Centralidade (Centrality Bias)
			var graphScore float64 = 0.0
			if linksRaw, ok := res["links"].([]interface{}); ok {
				rawBoost := float64(len(linksRaw)) * 0.03
				if rawBoost > 0.20 { rawBoost = 0.20 }
				graphScore = rawBoost
			}

			finalScore := vecScore + graphScore
			
			// 🧠 RE-RANKING NEURAL: O clique do usuário agora decide a vitória!
			nodeName, _ := res["name"].(string)
			neuralScore := float64(s.Ranker.AdjustScore(nodeName, float32(finalScore)))

			res["_hybrid_score"] = neuralScore // Atualiza para o score final (Vetor + Grafo + Neural)
			ranked = append(ranked, RankedNode{Payload: res, Score: neuralScore})
		}
	}

	processResults(obsidianResults, false)
	processResults(memoryResults, true)

	// 3. Sorting Global por relevância híbrida
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score
	})

	// 4. Trimming Final (Top N resultados unificados)
	finalResults := make([]map[string]interface{}, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		finalResults = append(finalResults, r.Payload)
	}

	return finalResults, nil
}

// ExpandContext toma os nós principais e busca seus vizinhos imediatos (1-Hop) para enriquecimento de conhecimento.
func (s *SearchService) ExpandContext(ctx context.Context, nodes []map[string]interface{}) ([]map[string]interface{}, error) {
	seenIds := make(map[uint64]bool)
	var neighborIds []uint64

	// 1. Mapear quem já temos
	for _, n := range nodes {
		if idVal, ok := n["id"].(float64); ok {
			seenIds[uint64(idVal)] = true
		}
	}

	// 2. Extrair IDs dos vizinhos
	for _, n := range nodes {
		if links, ok := n["links"].([]interface{}); ok {
			for _, l := range links {
				var id uint64
				switch v := l.(type) {
				case float64:
					id = uint64(v)
				case uint64:
					id = v
				}
				
				if id > 0 && !seenIds[id] {
					neighborIds = append(neighborIds, id)
					seenIds[id] = true
				}
			}
		}
	}

	if len(neighborIds) == 0 {
		return nodes, nil
	}

	// 3. Busca em lote no Qdrant
	neighbors, err := s.Qdrant.GetPoints("obsidian_knowledge", neighborIds)
	if err != nil {
		return nodes, nil 
	}

	for _, nb := range neighbors {
		nb["_context_type"] = "related"
	}

	return append(nodes, neighbors...), nil
}
