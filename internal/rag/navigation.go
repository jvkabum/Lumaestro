package rag

import (
	"context"
	"Lumaestro/internal/provider"
)

// GraphNavigator gerencia a expansão de contexto baseada em links.
type GraphNavigator struct {
	Qdrant *provider.QdrantClient
}

// NewGraphNavigator inicializa o navegador com acesso à memória semântica.
func NewGraphNavigator(qdrant *provider.QdrantClient) *GraphNavigator {
	return &GraphNavigator{Qdrant: qdrant}
}

// ExpandContext busca as notas vizinhas de forma recursiva (profundidade 2).
func (n *GraphNavigator) ExpandContext(ctx context.Context, initialNotes []map[string]interface{}) []string {
	var fullContext []string
	visited := make(map[string]bool)

	for _, note := range initialNotes {
		content, _ := note["content"].(string)
		title, _ := note["name"].(string)

		if visited[title] {
			continue
		}
		visited[title] = true
		fullContext = append(fullContext, content)

		// Nota: A expansão avançada por links será implementada aqui 
		// usando o Graph db ou o Qdrant Payload.
	}

	return fullContext
}
