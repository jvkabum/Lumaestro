package rag

import (
	"context"
	"sort"

	"Lumaestro/internal/provider"
)

// SearchService gerencia a recuperação de conhecimento inteligente (Vítreo/Gráfico).
type SearchService struct {
	Qdrant *provider.QdrantClient
}

// NewSearchService inicializa o buscador avançado.
func NewSearchService(qdrant *provider.QdrantClient) *SearchService {
	return &SearchService{Qdrant: qdrant}
}

// SearchNote não apenas busca o vetor, mas aplica Multi-Re-Ranking para destacar "Hubs".
func (s *SearchService) SearchNote(ctx context.Context, vector []float32, limit int) ([]map[string]interface{}, error) {
	// 1. Oversampling Sútil (Pegar mais para ter uma boa margem de re-ranking)
	const oversampleFactor = 3
	rawResults, err := s.Qdrant.SearchWithScores("obsidian_knowledge", vector, limit*oversampleFactor)
	if err != nil {
		return nil, err
	}

	// 2. Score Híbrido (Vector: 80% / Graph Centrality Boost: 20%)
	// Assumimos que a busca retorna _score
	type RankedNode struct {
		Payload map[string]interface{}
		Score   float64
	}

	var ranked []RankedNode
	for _, res := range rawResults {
		vecScore, _ := res["_score"].(float64) // Similaridade bruta (ex: 0.85)

		var graphScore float64 = 0.0
		if linksRaw, ok := res["links"].([]interface{}); ok {
			// Um empurrão tático para nós centrais (+0.03 de boost de similaridade extra a cada link real originado, limit. máximo de 0.20)
			rawBoost := float64(len(linksRaw)) * 0.03
			if rawBoost > 0.20 {
				rawBoost = 0.20
			}
			graphScore = rawBoost
		}

		// Re-Ranking Final
		finalScore := vecScore + graphScore
		
		// Guardar internamente para debug/log eventuais
		res["_hybrid_score"] = finalScore
		
		ranked = append(ranked, RankedNode{Payload: res, Score: finalScore})
	}

	// 3. Sorting Inteligente Descendente (Maior Score Ouro para Menor)
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].Score > ranked[j].Score // Sort reverso (do Top 1 para trás)
	})

	// 4. Trimming Final Retornando os Elites Exatos Limitados
	finalResults := make([]map[string]interface{}, 0, limit)
	for i, r := range ranked {
		if i >= limit {
			break
		}
		finalResults = append(finalResults, r.Payload)
	}

	return finalResults, nil
}
