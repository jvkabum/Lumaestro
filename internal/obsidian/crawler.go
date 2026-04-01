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

// PurgeCache remove o arquivo físico e limpa a memória para forçar reindexação total.
func (c *Crawler) PurgeCache() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	fmt.Printf("[Crawler] 🔥 Iniciando PurgeCache em %s\n", c.cachePath)
	c.cache = make(IndexCache)
	err := os.Remove(c.cachePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("falha ao remover arquivo de cache: %w", err)
	}
	fmt.Println("[Crawler] ✅ Cache local removido com sucesso. Próximo SCAN será completo.")
	return nil
}

// IndexVault percorre e indexa notas do Obsidian (Cofre do Usuário).
func (c *Crawler) IndexVault(ctx context.Context) error {
	// 🏗️ Garante que as 'gavetas' (coleções) existam no Qdrant antes de começar
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	var totalSkipped int = 0
	var totalIndexed int = 0

	err := filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Detecta tipo base (Chunk para MD, Source para Mídia)
		ext := strings.ToLower(filepath.Ext(path))
		docType := "chunk"
		if ext == ".pdf" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			docType = "source"
		}

		indexed, err := c.processFile(ctx, path, info, docType)
		if err == nil {
			if indexed {
				totalIndexed++
			} else {
				totalSkipped++
			}
		}
		return nil
	})

	c.saveCache()
	runtime.EventsEmit(ctx, "agent:log", map[string]string{
		"source":  "CRAWLER",
		"content": fmt.Sprintf("✅ Indexação do Vault completa. Novos: %d. Cache: %d.", totalIndexed, totalSkipped),
	})
	return err
}

// IndexSystemDocs varre a raiz do projeto em busca de documentação técnica interna.
func (c *Crawler) IndexSystemDocs(ctx context.Context, rootPath string) error {
	// 🏗️ Garante que as 'gavetas' (coleções) existam no Qdrant antes de começar
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	var totalIndexed int = 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Filtros de Segurança: Ignora dependências e arquivos de build
		pathLower := strings.ToLower(path)
		if strings.Contains(pathLower, "node_modules") || 
		   strings.Contains(pathLower, ".git") || 
		   strings.Contains(pathLower, "wailsjs") || 
		   strings.Contains(pathLower, "build") ||
		   strings.Contains(pathLower, "bin") ||
		   strings.Contains(pathLower, ".next") ||
		   strings.Contains(pathLower, "frontend/dist") {
			return nil
		}

		// Apenas documentos Markdown do próprio projeto
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".md" {
			return nil
		}

		// Indexa como tipo 'system' para diferenciação no 3D
		indexed, err := c.processFile(ctx, path, info, "system")
		if err == nil && indexed {
			totalIndexed++
		}
		return nil
	})

	if totalIndexed > 0 {
		runtime.EventsEmit(ctx, "agent:log", map[string]string{
			"source":  "SYSTEM",
			"content": fmt.Sprintf("⚙️ Documentação do projeto integrada ao RAG (%d arquivos).", totalIndexed),
		})
	}
	return err
}

// processFile é o núcleo de inteligência que processa, extrai triplas e salva no Qdrant
func (c *Crawler) processFile(ctx context.Context, path string, info os.FileInfo, forcedDocType string) (bool, error) {
	// 🔍 LOG DE DIAGNÓSTICO: Vermos exatamente o que o crawler está percorrendo
	fmt.Printf("[Crawler] Auditando: %s\n", path)

	ext := strings.ToLower(filepath.Ext(path))
	isMD := ext == ".md"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
	isPDF := ext == ".pdf"

	if !isMD && !isImage && !isPDF {
		return false, nil
	}

	c.mu.Lock()
	lastMod, exists := c.cache[path]
	c.mu.Unlock()

	nodeName := strings.TrimSuffix(info.Name(), ext)

	// Lógica de Cache (Aumenta performance do Boot)
	if exists && lastMod == info.ModTime().Unix() {
		fmt.Printf("[Crawler] 💨 Pulando (Cache Válido): %s\n", nodeName)
		runtime.EventsEmit(ctx, "graph:node", map[string]string{
			"id":            nodeName,
			"name":          nodeName,
			"document-type": forcedDocType,
		})
		content, err := os.ReadFile(path)
		if err == nil {
			links := extractLinks(string(content))
			for _, link := range links {
				runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": nodeName, "target": link})
			}
		}
		return false, nil
	}

	fmt.Printf("[Crawler] 🚀 REINDEXANDO (Cache Vazio/Novo): %s (Type: %s)\n", nodeName, forcedDocType)

	rawContent, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	var textContent string
	var triples []provider.Triple
	var links []string

	if isMD {
		textContent = string(rawContent)
		
		// Desambiguação (Context Graphs): Extrai o título e o início para resolver pronomes
		contextHint := fmt.Sprintf("Arquivo: %s. Contexto inicial: %s", nodeName, firstLines(textContent, 500))
		triples, _ = c.Ontology.ExtractTriples(ctx, textContent, contextHint)
		links = extractLinks(textContent)
	} else {
		// Visão Computacional / OCR
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

	// 1. Geração de Embedding
	vector, err := c.Embedder.GenerateEmbedding(ctx, textContent)
	if err != nil {
		return false, err
	}

	// 2. Persistência no Qdrant com Verdade Situacional
	c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
		"path": path, 
		"name": nodeName, 
		"content": textContent, 
		"triples": triples, 
		"links": links, 
		"type": ext, 
		"document-type": forcedDocType,
		"status": "active", // Marca o fato como ativo (Truth Strategy)
		"observed_at": time.Now().Format(time.RFC3339),
	})

	// 3. Update Cache
	c.mu.Lock()
	c.cache[path] = info.ModTime().Unix()
	c.mu.Unlock()

	// 4. Feedback Visual em Tempo Real
	runtime.EventsEmit(ctx, "graph:node", map[string]string{
		"id":            nodeName,
		"name":          nodeName,
		"document-type": forcedDocType,
	})
	for _, link := range links {
		runtime.EventsEmit(ctx, "graph:edge", map[string]string{"source": nodeName, "target": link})
	}

	return true, nil
}

var linkRegex = regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)

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

// EnsureCollections verifica e cria as coleções necessárias no Qdrant.
func (c *Crawler) EnsureCollections(ctx context.Context) error {
	collections := []string{"obsidian_knowledge", "knowledge_graph"}
	dimension := 3072 // Gemini Embedding v2 Dimension (768 era v1)

	for _, name := range collections {
		exists, err := c.Qdrant.CheckCollectionExists(name)
		if err != nil {
			return fmt.Errorf("erro ao verificar coleção %s: %w", name, err)
		}

		if !exists {
			fmt.Printf("[Crawler] 🏗️ Criando coleção inexistente: %s (Dim: %d)\n", name, dimension)
			runtime.EventsEmit(ctx, "agent:log", map[string]string{
				"source":  "CRAWLER",
				"content": fmt.Sprintf("🏗️ Preparando infraestrutura: Criando coleção '%s' (3072 dim)...", name),
			})
			if err := c.Qdrant.CreateCollection(name, dimension); err != nil {
				return fmt.Errorf("falha ao criar coleção %s: %w", name, err)
			}
		}
	}
	return nil
}

func firstLines(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "..."
}

