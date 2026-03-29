package agents

import (
	"context"
	"hash/fnv"

	"Lumaestro/internal/provider"
)

// Skillbook gerencia o armazenamento de estratégias de aprendizado.
type Skillbook struct {
	Qdrant   *provider.QdrantClient
	Embedder *provider.EmbeddingService
}

// NewSkillbook inicializa o repositório de habilidades.
func NewSkillbook(qdrant *provider.QdrantClient, embedder *provider.EmbeddingService) *Skillbook {
	return &Skillbook{Qdrant: qdrant, Embedder: embedder}
}

// SaveSkill salva uma nova estratégia no Qdrant.
func (s *Skillbook) SaveSkill(ctx context.Context, description string) error {
	// 1. Gerar vetor da estratégia
	vector, err := s.Embedder.GenerateEmbedding(ctx, description)
	if err != nil {
		return err
	}

	// 2. Gerar ID único estável
	h := fnv.New64a()
	h.Write([]byte(description))
	id := h.Sum64()

	// 3. Salvar na coleção "ace_skills"
	payload := map[string]interface{}{
		"description": description,
		"type":        "STRATEGY",
	}

	return s.Qdrant.UpsertPoint("ace_skills", id, vector, payload)
}

// RetrieveRelevantSkills busca estratégias que possam ajudar na pergunta atual.
func (s *Skillbook) RetrieveRelevantSkills(ctx context.Context, query string) ([]string, error) {
	return nil, nil
}
