package obsidian

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"Lumaestro/internal/provider"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type IndexCache map[string]int64

// Crawler gerencia a descoberta e indexação de notas.
type Crawler struct {
	VaultPath string
	Embedder  *provider.EmbeddingService
	Qdrant    *provider.QdrantClient
	Ontology  *provider.OntologyService
	cachePath string
	cache     IndexCache
	mu        sync.Mutex
}

// NewCrawler inicializa o crawler com suporte a cache de indexação.
func NewCrawler(vaultPath string, embedder *provider.EmbeddingService, qdrant *provider.QdrantClient, ontology *provider.OntologyService) *Crawler {
	c := &Crawler{
		VaultPath: vaultPath,
		Embedder:  embedder,
		Qdrant:    qdrant,
		Ontology:  ontology,
		cachePath: ".context/index_cache.json",
		cache:     make(IndexCache),
	}
	c.loadCache()
	return c
}

func (c *Crawler) loadCache() {
	data, err := os.ReadFile(c.cachePath)
	if err == nil {
		json.Unmarshal(data, &c.cache)
	}
}

func (c *Crawler) saveCache() {
	os.MkdirAll(".context", 0755)
	data, _ := json.MarshalIndent(c.cache, "", "  ")
	os.WriteFile(c.cachePath, data, 0644)
}

// IndexVault percorre e indexa notas somente se tiverem sido modificadas.
func (c *Crawler) IndexVault(ctx context.Context) error {
	var totalSkipped int = 0
	var totalIndexed int = 0

	err := filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		// Checa se o arquivo mudou antes de chamar a API
		c.mu.Lock()
		lastMod, exists := c.cache[path]
		c.mu.Unlock()

		if exists && lastMod == info.ModTime().Unix() {
			totalSkipped++
			return nil // JÁ INDEXADO E INTEGRAL
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// 1. Gerar Embedding
		vector, err := c.Embedder.GenerateEmbedding(ctx, string(content))
		if err != nil {
			return nil // Silencia erros individuais
		}

		// 2. Extrair Triplas
		triples, _ := c.Ontology.ExtractTriples(ctx, string(content))

		// 2.5 Extrair Integridade do Grafo (Links)
		links := extractLinks(string(content))

		// 3. Salvar no Qdrant
		nodeName := strings.TrimSuffix(info.Name(), ".md")
		c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
			"path": path, "name": nodeName, "content": string(content), "triples": triples, "links": links,
		})

		// 4. Update Cache
		c.mu.Lock()
		c.cache[path] = info.ModTime().Unix()
		c.mu.Unlock()
		totalIndexed++

		// 5. Atualiza UI
		runtime.EventsEmit(ctx, "graph:node", map[string]string{"id": nodeName, "name": nodeName})
		for _, t := range triples {
			runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": nodeName, "target": t.Object})
		}

		return nil
	})

	c.saveCache()
	runtime.EventsEmit(ctx, "agent:log", map[string]string{
		"source":  "CRAWLER",
		"content": fmt.Sprintf("✅ Indexação completa. Arquivos novos/alterados: %d. Ignorados por cache: %d.", totalIndexed, totalSkipped),
	})
	return err
}

var linkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

// extractLinks extrai os links bidirecionais do conteúdo da nota
func extractLinks(content string) []string {
	matches := linkRegex.FindAllStringSubmatch(content, -1)
	var links []string
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) > 1 {
			link := strings.TrimSpace(match[1])
			if link != "" && !seen[link] {
				seen[link] = true
				links = append(links, link)
			}
		}
	}
	return links
}
