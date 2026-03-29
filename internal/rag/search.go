package rag

import (
	"context"
	"Lumaestro/internal/provider"
)

// SearchService gerencia a recuperação de conhecimento do Qdrant.
type SearchService struct {
	Qdrant *provider.QdrantClient
}

// NewSearchService inicializa o buscador.
func NewSearchService(qdrant *provider.QdrantClient) *SearchService {
	return &SearchService{Qdrant: qdrant}
}

// SearchNote busca as notas mais similares a uma query vetorial.
func (s *SearchService) SearchNote(ctx context.Context, vector []float32, limit int) ([]map[string]interface{}, error) {
	return s.Qdrant.Search("obsidian_knowledge", vector, limit)
}
