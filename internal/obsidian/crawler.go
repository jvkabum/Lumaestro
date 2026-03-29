package obsidian

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"Lumaestro/internal/provider"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Crawler gerencia a descoberta e indexação de notas.
type Crawler struct {
	VaultPath string
	Embedder  *provider.EmbeddingService
	Qdrant    *provider.QdrantClient
	Ontology  *provider.OntologyService
}

// NewCrawler inicializa o crawler com as dependências de inteligência completa.
func NewCrawler(vaultPath string, embedder *provider.EmbeddingService, qdrant *provider.QdrantClient, ontology *provider.OntologyService) *Crawler {
	return &Crawler{
		VaultPath: vaultPath,
		Embedder:  embedder,
		Qdrant:    qdrant,
		Ontology:  ontology,
	}
}

// IndexVault percorre e indexa semânticamente cada nota .md no Qdrant.
func (c *Crawler) IndexVault(ctx context.Context) error {
	var fileCount uint64 = 1

	return filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// 1. Gerar Embedding via Gemini
			vector, err := c.Embedder.GenerateEmbedding(ctx, string(content))
			if err != nil {
				return fmt.Errorf("erro gerando embedding para %s: %w", path, err)
			}

			// 2. Extrair Fatos (Ontologia TrustGraph)
			triples, _ := c.Ontology.ExtractTriples(ctx, string(content))

			// 3. Preparar Payload (Metadados da nota + Fatos)
			payload := map[string]interface{}{
				"path":    path,
				"name":    info.Name(),
				"content": string(content),
				"triples": triples,
			}

			// 4. Upsert no Qdrant Remoto (Coolify)
			err = c.Qdrant.UpsertPoint("obsidian_knowledge", fileCount, vector, payload)
			if err != nil {
				return fmt.Errorf("erro no upload p/ Qdrant: %w", err)
			}

			// 5. Notificar Interface (Atualiza o Grafo Starfield)
			nodeName := strings.TrimSuffix(info.Name(), ".md")
			runtime.EventsEmit(ctx, "graph:node", map[string]string{
				"id":   nodeName,
				"name": nodeName,
			})

			// Emite as conexões (edges) se houver fatos extraídos
			for _, triple := range triples {
				runtime.EventsEmit(ctx, "graph:edge", map[string]string{
					"source": nodeName,
					"target": triple.Object, // Assume que o objeto da tripla é outra nota vinculada
				})
			}

			fileCount++
		}
		return nil
	})
}
