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

// IndexVault percorre e indexa notas independentemente de terem Cache (Forçar Reboot Visual e DB).
func (c *Crawler) IndexVault(ctx context.Context) error {
	// O cache é preservado para evitar re-indexação inútil.


	var totalSkipped int = 0
	var totalIndexed int = 0

	err := filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		isMD := ext == ".md"
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		isPDF := ext == ".pdf"

		if !isMD && !isImage && !isPDF {
			return nil
		}

		c.mu.Lock()
		lastMod, exists := c.cache[path]
		c.mu.Unlock()

		nodeName := strings.TrimSuffix(info.Name(), ".md")
		if exists && lastMod == info.ModTime().Unix() {
			totalSkipped++
			
			// ✅ RESTAURAÇÃO VISUAL: Mesmo em cache, precisamos avisar a UI sobre a existência do nó e suas conexões
			runtime.EventsEmit(ctx, "graph:node", map[string]string{"id": nodeName, "name": nodeName})
			content, err := os.ReadFile(path)
			if err == nil {
				links := extractLinks(string(content))
				for _, link := range links {
					runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": nodeName, "target": link})
				}
			}
			return nil // JÁ INDEXADO E INTEGRAL
		}

		rawContent, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		var textContent string
		var triples []provider.Triple
		var links []string

		if isMD {
			textContent = string(rawContent)
			triples, _ = c.Ontology.ExtractTriples(ctx, textContent)
			links = extractLinks(textContent)
		} else {
			// Lógica Multimodal (OCR / Visão)
			mimeType := "image/png"
			if isPDF {
				mimeType = "application/pdf"
			} else if ext == ".jpg" || ext == ".jpeg" {
				mimeType = "image/jpeg"
			}

			runtime.EventsEmit(ctx, "agent:log", map[string]string{
				"source":  "CRAWLER",
				"content": fmt.Sprintf("👁️ Analisando mídia: %s...", info.Name()),
			})

			desc, tri, err := c.Ontology.ProcessMedia(ctx, rawContent, mimeType)
			if err == nil {
				textContent = desc
				triples = tri
			}
		}

		// 1. Gerar Embedding do conhecimento extraído (Seja texto puro ou descrição da imagem)
		vector, err := c.Embedder.GenerateEmbedding(ctx, textContent)
		if err != nil {
			return nil 
		}

		// 3. Salvar no Qdrant
		nodeName = strings.TrimSuffix(info.Name(), ext)
		c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
			"path": path, "name": nodeName, "content": textContent, "triples": triples, "links": links, "type": ext,
		})

		// 3.5 Feedback de Aprendizado
		if len(triples) > 0 || !isMD {
			msg := fmt.Sprintf("🧠 [%s] Conhecimento extraído com sucesso.\n", info.Name())
			if len(triples) > 0 {
				msg = fmt.Sprintf("🧠 [%s] Aprendi %d novos fatos estruturados.\n", info.Name(), len(triples))
			}
			runtime.EventsEmit(ctx, "agent:log", map[string]string{"source": "CRAWLER", "content": msg})
		}

		// 4. Update Cache
		c.mu.Lock()
		c.cache[path] = info.ModTime().Unix()
		c.mu.Unlock()
		totalIndexed++

		// 5. Atualiza UI ao terminar o Parser
		runtime.EventsEmit(ctx, "graph:node", map[string]string{"id": nodeName, "name": nodeName})
		for _, link := range links {
			runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": nodeName, "target": link})
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
