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
	"sync/atomic"
	"time"

	"Lumaestro/internal/config"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/utils"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type IndexCache map[string]int64

// Crawler gerencia a descoberta e indexação de notas.
type Crawler struct {
	ctx       context.Context // Contexto persistente do Wails (Lifecycle)
	VaultPath string
	Embedder  *provider.EmbeddingService
	Qdrant    *provider.QdrantClient
	Ontology  *provider.OntologyService
	cachePath string
	cache     IndexCache
	mu        sync.Mutex
	workerCount int // 👷 Número de workers paralelos
}

type crawlTask struct {
	path          string
	info          os.FileInfo
	docType       string
	implicitLinks []string
}

// SetContext injeta o contexto oficial do Wails para emissão de eventos assíncronos.
func (c *Crawler) SetContext(ctx context.Context) {
	c.ctx = ctx
}

// NewCrawler inicializa o crawler com suporte a cache de indexação.
func NewCrawler(vaultPath string, embedder *provider.EmbeddingService, qdrant *provider.QdrantClient, ontology *provider.OntologyService) *Crawler {
	c := &Crawler{
		VaultPath:   vaultPath,
		Embedder:    embedder,
		Qdrant:      qdrant,
		Ontology:    ontology,
		cachePath:   ".context/index_cache.json",
		cache:       make(IndexCache),
		workerCount: 8, // ⚙️ Valor balanceado para cota Gemini vs Velocidade
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

// IndexVault percorre e indexa notas do Obsidian (Cofre do Usuário) em paralelo.
func (c *Crawler) IndexVault(ctx context.Context) error {
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	tasks := make(chan crawlTask, 100)
	var wg sync.WaitGroup
	var totalSkipped int32 = 0
	var totalIndexed int32 = 0

	// 👷 Iniciar Workers
	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				indexed, err := c.processFile(ctx, task.path, task.info, task.docType, task.implicitLinks)
				if err == nil {
					if indexed {
						atomic.AddInt32(&totalIndexed, 1)
					} else {
						atomic.AddInt32(&totalSkipped, 1)
					}
				}
			}
		}()
	}

	err := filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		docType := "chunk"
		if ext == ".pdf" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			docType = "source"
		}

		// Enviar tarefa para o pool
		tasks <- crawlTask{path: path, info: info, docType: docType}
		return nil
	})

	close(tasks)
	wg.Wait()

	c.saveCache()
	runtime.EventsEmit(ctx, "agent:log", map[string]string{
		"source":  "CRAWLER",
		"content": fmt.Sprintf("✅ Indexação completa. Novos: %d. Cache: %d.", totalIndexed, totalSkipped),
	})
	return err
}

// IndexSystemDocs varre a raiz do projeto em busca de documentação técnica interna (Paralelo).
func (c *Crawler) IndexSystemDocs(ctx context.Context, rootPath string) error {
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	tasks := make(chan crawlTask, 100)
	var wg sync.WaitGroup
	var totalIndexed int32 = 0

	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				indexed, err := c.processFile(ctx, task.path, task.info, task.docType, nil)
				if err == nil && indexed {
					atomic.AddInt32(&totalIndexed, 1)
				}
			}
		}()
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

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

		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}

		tasks <- crawlTask{path: path, info: info, docType: "system"}
		return nil
	})

	close(tasks)
	wg.Wait()

	if totalIndexed > 0 {
		runtime.EventsEmit(c.ctx, "agent:log", map[string]string{
			"source":  "SYSTEM",
			"content": fmt.Sprintf("⚙️ Documentação do projeto integrada ao RAG (%d arquivos).", totalIndexed),
		})
	}
	return err
}

// IndexRepositories engloba a lógica radial paralela.
func (c *Crawler) IndexRepositories(ctx context.Context, repositories []config.ProjectScan) error {
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	for _, repo := range repositories {
		if repo.Path == "" { continue }

		tasks := make(chan crawlTask, 100)
		var wg sync.WaitGroup
		var totalIndexed int32 = 0

		fmt.Printf("[Crawler] 🪐 Acionando RAG Radial no repousitório PARALELO: %s\n", repo.Path)

		for i := 0; i < c.workerCount; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					indexed, err := c.processFile(ctx, task.path, task.info, task.docType, task.implicitLinks)
					if err == nil && indexed {
						atomic.AddInt32(&totalIndexed, 1)
					}
				}
			}()
		}
		
		filepath.Walk(repo.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() { return nil }

			pathLower := strings.ToLower(path)
			if strings.Contains(pathLower, "node_modules") || 
			   strings.Contains(pathLower, ".git") || 
			   strings.Contains(pathLower, "build") ||
			   strings.Contains(pathLower, "dist") {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			isCode := ext == ".go" || ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".html" || ext == ".css"
			isMD := ext == ".md"
			
			if !isMD && !(isCode && repo.IncludeCode) {
				return nil
			}

			docType := "project-file"
			if isCode { docType = "code-file" }

			tasks <- crawlTask{
				path: path, 
				info: info, 
				docType: docType, 
				implicitLinks: []string{repo.CoreNode},
			}
			return nil
		})

		close(tasks)
		wg.Wait()

		if totalIndexed > 0 {
			runtime.EventsEmit(c.ctx, "agent:log", map[string]string{
				"source":  "RADIAL",
				"content": fmt.Sprintf("🌌 %s orbitado! %d fragmentos amarrados ao núcleo RAG.", repo.CoreNode, totalIndexed),
			})
		}
	}
	return nil
}

// processFile é o núcleo de inteligência que processa, extrai triplas e salva no Qdrant
func (c *Crawler) processFile(ctx context.Context, path string, info os.FileInfo, forcedDocType string, implicitLinks []string) (bool, error) {
	// 🔍 LOG DE DIAGNÓSTICO: Vermos exatamente o que o crawler está percorrendo
	fmt.Printf("[Crawler] Auditando: %s\n", path)

	ext := strings.ToLower(filepath.Ext(path))
	isMD := ext == ".md"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
	isPDF := ext == ".pdf"
	isCode := ext == ".go" || ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".html" || ext == ".css"

	if !isMD && !isImage && !isPDF && !isCode {
		return false, nil
	}

	c.mu.Lock()
	lastMod, exists := c.cache[path]
	c.mu.Unlock()

	nodeName := strings.TrimSuffix(info.Name(), ext)
	nodeID := strings.ToLower(nodeName) // Normalização para compatibilidade de links

	// Lógica de Cache (Aumenta performance do Boot)
	if exists && lastMod == info.ModTime().Unix() {
		fmt.Printf("[Crawler] 💨 Pulando (Cache Válido): %s\n", nodeName)
		runtime.EventsEmit(ctx, "graph:node", map[string]string{
			"id":            nodeID,
			"name":          nodeName,
			"document-type": forcedDocType,
		})
		content, err := os.ReadFile(path)
		if err == nil {
			linkCounts := make(map[string]int)
			links := extractLinks(string(content))
			for _, l := range links {
				targetID := strings.ToLower(l)
				linkCounts[targetID]++
			}
			for target, weight := range linkCounts {
				runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
					"source": nodeID, 
					"target": target,
					"weight": weight,
				})
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
	
	// Adiciona os links orbitais (radiais implícitos)
	if len(implicitLinks) > 0 {
		links = append(links, implicitLinks...)
	}

	if isMD || isCode {
		textContent = string(rawContent)
		
		// Desambiguação (Context Graphs): Extrai o título e o início para resolver pronomes
		contextHint := fmt.Sprintf("Arquivo: %s. Contexto inicial: %s", nodeName, firstLines(textContent, 500))
		triples, err = c.Ontology.ExtractTriples(ctx, textContent, contextHint)
		if err != nil {
			fmt.Printf("[Crawler] ⚠️ Erro ao extrair triplas de %s: %s\n", nodeName, utils.FormatGenAIError(err))
		} else {
			fmt.Printf("[Crawler] 🧠 %d Triplas extraídas de %s\n", len(triples), nodeName)
		}
		links = extractLinks(textContent)
	} else {
		// Visão Computacional / OCR
		mimeType := "image/png"
		if isPDF {
			mimeType = "application/pdf"
		} else if ext == ".jpg" || ext == ".jpeg" {
			mimeType = "image/jpeg"
		}

		runtime.EventsEmit(c.ctx, "agent:log", map[string]string{
			"source":  "CRAWLER",
			"content": fmt.Sprintf("👁️ Analisando mídia: %s...", info.Name()),
		})

		desc, tri, err := c.Ontology.ProcessMedia(ctx, rawContent, mimeType)
		if err == nil {
			textContent = desc
			triples = tri
		}
	}

	// ══════════════════════════════════════════════════════════
	// PASSO 1: FEEDBACK VISUAL IMEDIATO (Independente de API)
	// ══════════════════════════════════════════════════════════
	runtime.EventsEmit(c.ctx, "graph:node", map[string]string{
		"id":            nodeID,
		"name":          nodeName,
		"document-type": forcedDocType,
	})

	// Conta links Obsidian [[wiki-links]] (100% offline, sem API)
	linkCounts := make(map[string]int)
	for _, l := range links {
		targetID := strings.ToLower(l)
		linkCounts[targetID]++
	}

	// Adiciona peso das triplas semânticas extraídas pela IA (se houver)
	for _, t := range triples {
		if t.Object != "" && len(t.Object) < 50 { 
			targetID := strings.ToLower(t.Object)
			linkCounts[targetID]++
		}
	}

	// Emite TODAS as arestas imediatamente para o frontend
	for target, weight := range linkCounts {
		runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
			"source": nodeID, 
			"target": target,
			"weight": weight,
		})
	}

	fmt.Printf("[Crawler] 🔗 %s: %d links emitidos para o grafo visual\n", nodeName, len(linkCounts))

	// ══════════════════════════════════════════════════════════
	// PASSO 2: PERSISTÊNCIA VETORIAL (Depende da API Gemini)
	// ══════════════════════════════════════════════════════════
	var vector []float32
	if isImage || isPDF {
		// 🌟 MULTIMODALIDADE NATIVA (Gemini Embedding 2)
		// Transforma a mídia diretamente em um vetor sem precisar de descrição prévia.
		mimeType := "image/png"
		if isPDF { mimeType = "application/pdf" }
		
		vector, err = c.Embedder.GenerateMultimodalEmbedding(ctx, rawContent, mimeType)
	} else {
		vector, err = c.Embedder.GenerateEmbedding(ctx, textContent)
	}

	if err != nil {
		fmt.Printf("[Crawler] ⚠️ Embedding falhou para %s: %s\n", nodeName, utils.FormatGenAIError(err))
		return true, nil
	}

	// Persistência no Qdrant com Verdade Situacional
	c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
		"path": path, 
		"name": nodeName, 
		"content": textContent, 
		"triples": triples, 
		"links": links, 
		"type": ext, 
		"document-type": forcedDocType,
		"status": "active",
		"observed_at": time.Now().Format(time.RFC3339),
	})

	// Update Cache (só marca como "completo" se o embedding teve sucesso)
	c.mu.Lock()
	c.cache[path] = info.ModTime().Unix()
	c.mu.Unlock()

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
			runtime.EventsEmit(c.ctx, "agent:log", map[string]string{
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

