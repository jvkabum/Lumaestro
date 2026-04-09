package obsidian

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// IndexCache armazena o hash SHA-256 do conteúdo de cada arquivo indexado.
// Isso é mais preciso do que a data de modificação, pois detecta mudanças reais de conteúdo.
type IndexCache map[string]string

// Crawler gerencia a descoberta e indexação de notas.
type Crawler struct {
	ctx       context.Context // Contexto persistente do Wails (Lifecycle)
	VaultPath string
	Embedder  provider.Embedder
	Qdrant    *provider.QdrantClient
	Ontology  *provider.OntologyService
	cachePath string
	cache     IndexCache
	mu        sync.Mutex
	workerCount int // 👷 Número de workers paralelos (reduzido para evitar burst de cota)
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
func NewCrawler(vaultPath string, embedder provider.Embedder, qdrant *provider.QdrantClient, ontology *provider.OntologyService) *Crawler {
	c := &Crawler{
		VaultPath:   vaultPath,
		Embedder:    embedder,
		Qdrant:      qdrant,
		Ontology:    ontology,
		cachePath:   ".context/index_cache.json",
		cache:       make(IndexCache),
		workerCount: 2, // ⚙️ Reduzido para 2 — evita burst de cota em chaves gratuitas
	}
	c.loadCache()
	return c
}

// contentHash gera um SHA-256 do conteúdo do arquivo para cache inteligente.
func contentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
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

// IndexVault percorre e indexa notas do Obsidian em DUAS FASES para máxima eficiência.
// FASA 1 (Offline): Extrai links e monta o grafo visual hierárquico (0 chamadas de API).
func (c *Crawler) IndexVault(ctx context.Context) error {
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	// ══════════════════════════════════════════════════════════
	// FASE 1: GRAFO VISUAL HIERÁRQUICO (Cosmos Cognitivo)
	// ══════════════════════════════════════════════════════════
	fmt.Println("[Crawler] ⚡ FASE 1: Montando Cosmos Cognitivo (Galáxia, Planetas e Luas)...")
	var pendingFiles []crawlTask 
	var totalCached int32 = 0
	processedFolders := make(map[string]bool)

	// Nome da Galáxia (Raiz do Vault)
	galaxyName := filepath.Base(c.VaultPath)
	galaxyID := "galaxy:" + strings.ToLower(galaxyName)

	// Emite o Sol Central (Core da Galáxia)
	runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
		"id":            galaxyID,
		"name":          galaxyName,
		"document-type": "galaxy-core",
		"celestial-type": "sun",
		"mass":          100.0,
		"summary":       "Nó central do vault; organiza pastas e documentos orbitais.",
		"what-it-does":  "Funciona como raiz estrutural da base de conhecimento no grafo 3D.",
	})

	err := filepath.Walk(c.VaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }

		relPath, _ := filepath.Rel(c.VaultPath, path)
		if relPath == "." { return nil }

		// 📁 Se for diretório, emite como um Planeta
		if info.IsDir() {
			folderID := "planet:" + strings.ToLower(relPath)
			folderName := info.Name()
			
			// Determina o Pai (Parent) para criar aresta de órbita
			parentDir := filepath.Dir(relPath)
			var parentID string
			if parentDir == "." {
				parentID = galaxyID
			} else {
				parentID = "planet:" + strings.ToLower(parentDir)
			}

			if !processedFolders[folderID] {
				runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
					"id":            folderID,
					"name":          folderName,
					"document-type": "folder",
					"celestial-type": "planet",
					"mass":          20.0,
					"summary":       fmt.Sprintf("Pasta '%s' no vault, contendo notas relacionadas.", folderName),
					"what-it-does":  "Agrupa notas por tema e cria contexto estrutural para navegação semântica.",
				})
				// Aresta de Órbita Física (Parentesco)
				runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
					"source": parentID,
					"target": folderID,
					"weight": 5, // Aresta forte de gravidade
				})
				processedFolders[folderID] = true
			}
			return nil
		}

		// 📄 Se for arquivo, emite como uma Lua
		ext := strings.ToLower(filepath.Ext(path))
		isMD := ext == ".md"
		isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
		isPDF := ext == ".pdf"
		isCode := ext == ".go" || ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".html" || ext == ".css"

		if !isMD && !isImage && !isPDF && !isCode {
			return nil
		}

		nodeName := strings.TrimSuffix(info.Name(), ext)
		nodeID := strings.ToLower(nodeName)
		docType := "chunk"
		if isImage || isPDF { docType = "source" }

		// Emite a Lua
		runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
			"id":            nodeID,
			"name":          nodeName,
			"document-type": docType,
			"celestial-type": "moon",
			"mass":          5.0,
			"summary":       fmt.Sprintf("Arquivo '%s' detectado e preparado para indexação semântica.", nodeName),
			"what-it-does":  "Será processado pelo RAG para responder perguntas com contexto real do conteúdo.",
		})

		// Aresta de Órbita da Lua ao seu Planeta (Pasta)
		parentDir := filepath.Dir(relPath)
		var parentID string
		if parentDir == "." {
			parentID = galaxyID
		} else {
			parentID = "planet:" + strings.ToLower(parentDir)
		}

		runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
			"source": parentID,
			"target": nodeID,
			"weight": 3, // Gravidade local
		})

		// Extrai links [[wiki-links]] (Relacionamentos Cruzados)
		if isMD || isCode {
			rawContent, readErr := os.ReadFile(path)
			if readErr == nil {
				links := extractLinks(string(rawContent))
				for _, target := range links {
					runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
						"source": nodeID,
						"target": strings.ToLower(target),
						"weight": 1, // Link semântico (mais fraco que órbita)
					})
				}

				// Cache inteligente
				hash := contentHash(rawContent)
				c.mu.Lock()
				cachedHash, exists := c.cache[path]
				c.mu.Unlock()

				if exists && cachedHash == hash {
					atomic.AddInt32(&totalCached, 1)
					return nil 
				}
			}
		}

		pendingFiles = append(pendingFiles, crawlTask{path: path, info: info, docType: docType})
		return nil
	})

	fmt.Printf("[Crawler] ⚡ Cosmos montado: %d objetos celestiais. %d arquivos pendentes para IA.\n", totalCached+int32(len(pendingFiles)), len(pendingFiles))

	// ══════════════════════════════════════════════════════════
	// FASE 2: PROCESSAMENTO SEMÂNTICO (API — Workers Limitados)
	// ══════════════════════════════════════════════════════════
	if len(pendingFiles) == 0 {
		fmt.Println("[Crawler] ✅ Nenhum arquivo novo ou modificado. Scan completo sem chamadas de API!")
		runtime.EventsEmit(ctx, "agent:log", map[string]string{
			"source":  "CRAWLER",
			"content": fmt.Sprintf("✅ Grafo montado. Todos os %d arquivos estão em cache.", totalCached),
		})
		return err
	}

	fmt.Printf("[Crawler] 🧠 FASE 2: Processando %d arquivos pendentes com %d workers...\n", len(pendingFiles), c.workerCount)

	tasks := make(chan crawlTask, 50)
	var wg sync.WaitGroup
	var totalIndexed int32 = 0

	for i := 0; i < c.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range tasks {
				indexed, processErr := c.processFile(ctx, task.path, task.info, task.docType, task.implicitLinks)
				if processErr == nil && indexed {
					atomic.AddInt32(&totalIndexed, 1)
				}
			}
		}()
	}

	for _, task := range pendingFiles {
		tasks <- task
	}
	close(tasks)
	wg.Wait()

	c.saveCache()
	runtime.EventsEmit(ctx, "agent:log", map[string]string{
		"source":  "CRAWLER",
		"content": fmt.Sprintf("✅ Indexação completa. Novos/Atualizados: %d. Cache: %d.", totalIndexed, totalCached),
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

// IndexRepositories engloba a lógica radial paralela com hierarquia celestial.
func (c *Crawler) IndexRepositories(ctx context.Context, repositories []config.ProjectScan) error {
	if err := c.EnsureCollections(ctx); err != nil {
		return err
	}

	for _, repo := range repositories {
		if repo.Path == "" { continue }

		tasks := make(chan crawlTask, 100)
		var wg sync.WaitGroup
		var totalIndexed int32 = 0
		processedFolders := make(map[string]bool)

		// O CoreNode é o Sol da Galáxia do Projeto
		galaxyID := "galaxy:" + strings.ToLower(repo.CoreNode)
		runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
			"id":            galaxyID,
			"name":          repo.CoreNode,
			"document-type": "galaxy-core",
			"celestial-type": "sun",
			"mass":          80.0,
			"summary":       fmt.Sprintf("Núcleo do repositório satélite '%s'.", repo.CoreNode),
			"what-it-does":  "Conecta código/projetos externos ao RAG radial sem misturar domínios.",
		})

		fmt.Printf("[Crawler] 🪐 Expandindo Galáxia Radial: %s\n", repo.CoreNode)

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
			if err != nil { return nil }

			relPath, _ := filepath.Rel(repo.Path, path)
			if relPath == "." { return nil }

			pathLower := strings.ToLower(path)
			if strings.Contains(pathLower, "node_modules") || 
			   strings.Contains(pathLower, ".git") || 
			   strings.Contains(pathLower, "build") ||
			   strings.Contains(pathLower, "dist") {
				return nil
			}

			// 📁 Emitir Pasta (Planeta)
			if info.IsDir() {
				folderID := "planet:" + strings.ToLower(repo.CoreNode+":"+relPath)
				folderName := info.Name()

				parentDir := filepath.Dir(relPath)
				var parentID string
				if parentDir == "." {
					parentID = galaxyID
				} else {
					parentID = "planet:" + strings.ToLower(repo.CoreNode+":"+parentDir)
				}

				if !processedFolders[folderID] {
					runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
						"id":            folderID,
						"name":          folderName,
						"document-type": "folder",
						"celestial-type": "planet",
						"mass":          15.0,
						"summary":       fmt.Sprintf("Pasta '%s' dentro do repositório satélite.", folderName),
						"what-it-does":  "Agrupa módulos relacionados para facilitar contexto técnico no grafo.",
					})
					runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
						"source": parentID,
						"target": folderID,
						"weight": 5,
					})
					processedFolders[folderID] = true
				}
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

			nodeName := strings.TrimSuffix(info.Name(), ext)
			nodeID := strings.ToLower(nodeName)

			// Emite a Lua do Projeto
			runtime.EventsEmit(c.ctx, "graph:node", map[string]interface{}{
				"id":            nodeID,
				"name":          nodeName,
				"document-type": docType,
				"celestial-type": "moon",
				"mass":          4.0,
				"summary":       fmt.Sprintf("Arquivo '%s' importado para análise semântica.", nodeName),
				"what-it-does":  "Alimenta o RAG com contexto de documentação/código do repositório satélite.",
			})

			// Aresta de órbita para a pasta
			parentDir := filepath.Dir(relPath)
			var parentID string
			if parentDir == "." {
				parentID = galaxyID
			} else {
				parentID = "planet:" + strings.ToLower(repo.CoreNode+":"+parentDir)
			}

			runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
				"source": parentID,
				"target": nodeID,
				"weight": 3,
			})

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
				"content": fmt.Sprintf("🌌 Galáxia %s estabilizada com %d planetas e luas.", repo.CoreNode, totalIndexed),
			})
		}
	}
	return nil
}

// processFile é o núcleo de inteligência que processa, extrai triplas e salva no Qdrant.
// Otimizado: pula extração de triplas para notas pequenas e adiciona throttle entre chamadas.
func (c *Crawler) processFile(ctx context.Context, path string, info os.FileInfo, forcedDocType string, implicitLinks []string) (bool, error) {
	ext := strings.ToLower(filepath.Ext(path))
	isMD := ext == ".md"
	isImage := ext == ".png" || ext == ".jpg" || ext == ".jpeg"
	isPDF := ext == ".pdf"
	isCode := ext == ".go" || ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx" || ext == ".py" || ext == ".html" || ext == ".css"

	if !isMD && !isImage && !isPDF && !isCode {
		return false, nil
	}

	nodeName := strings.TrimSuffix(info.Name(), ext)
	nodeID := strings.ToLower(nodeName)

	rawContent, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	// Cache por Hash de Conteúdo: Verifica se o conteúdo realmente mudou
	hash := contentHash(rawContent)
	c.mu.Lock()
	cachedHash, exists := c.cache[path]
	c.mu.Unlock()

	if exists && cachedHash == hash {
		return false, nil // Conteúdo idêntico — pula processamento semântico
	}

	fmt.Printf("[Crawler] 🚀 REINDEXANDO: %s (Type: %s)\n", nodeName, forcedDocType)

	var textContent string
	var triples []provider.Triple
	var links []string

	if len(implicitLinks) > 0 {
		links = append(links, implicitLinks...)
	}

	if isMD || isCode {
		textContent = string(rawContent)
		links = extractLinks(textContent)

		// 🧠 Extração de Triplas: Pula para notas muito pequenas (< 100 chars)
		// Notas curtas como lembretes não produzem triplas úteis e gastam cota.
		if len(textContent) >= 100 {
			contextHint := fmt.Sprintf("Arquivo: %s. Contexto inicial: %s", nodeName, firstLines(textContent, 500))
			triples, err = c.Ontology.ExtractTriples(ctx, textContent, contextHint)
			if err != nil {
				fmt.Printf("[Crawler] ⚠️ Erro ao extrair triplas de %s: %s\n", nodeName, utils.FormatGenAIError(err))
			} else {
				fmt.Printf("[Crawler] 🧠 %d Triplas extraídas de %s\n", len(triples), nodeName)
			}
		} else {
			fmt.Printf("[Crawler] 💨 Nota curta (%d chars), pulando extração de triplas para %s\n", len(textContent), nodeName)
		}
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

		desc, tri, errMedia := c.Ontology.ProcessMedia(ctx, rawContent, mimeType)
		if errMedia == nil {
			textContent = desc
			triples = tri
		}
	}

	// Emite arestas das triplas extraídas para o grafo visual
	for _, t := range triples {
		if t.Object != "" && len(t.Object) < 50 {
			runtime.EventsEmit(c.ctx, "graph:edge", map[string]interface{}{
				"source": nodeID,
				"target": strings.ToLower(t.Object),
				"weight": 1,
			})
		}
	}

	// ══════════════════════════════════════════════════════════
	// PERSISTÊNCIA VETORIAL (Depende da API Gemini)
	// ══════════════════════════════════════════════════════════
	var vector []float32
	if isImage || isPDF {
		mimeType := "image/png"
		if isPDF { mimeType = "application/pdf" }
		vector, err = c.Embedder.GenerateMultimodalEmbedding(ctx, rawContent, mimeType, false)
	} else {
		vector, err = c.Embedder.GenerateEmbedding(ctx, textContent, false)
	}

	if err != nil {
		fmt.Printf("[Crawler] ⚠️ Embedding falhou para %s: %s\n", nodeName, utils.FormatGenAIError(err))
		return true, nil
	}

	// Persistência no Qdrant
	c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
		"path": path, "name": nodeName, "content": textContent,
		"triples": triples, "links": links, "type": ext,
		"document-type": forcedDocType, "status": "active",
		"observed_at": time.Now().Format(time.RFC3339),
	})

	// Atualiza o cache com o hash do conteúdo
	c.mu.Lock()
	c.cache[path] = hash
	c.mu.Unlock()

	// ⏱️ Throttle suave: Respira 200ms entre cada arquivo para distribuir as chamadas
	time.Sleep(200 * time.Millisecond)

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

	// Dimensão configurável: lê do config (3072=Gemini, 768=LM Studio nomic, etc.)
	cfg, _ := config.Load()
	dimension := 3072
	if cfg != nil {
		cfg.NormalizeProviders()
		if cfg.EmbeddingDimension > 0 {
			dimension = cfg.EmbeddingDimension
		}
	}

	for _, name := range collections {
		exists, err := c.Qdrant.CheckCollectionExists(name)
		if err != nil {
			return fmt.Errorf("erro ao verificar coleção %s: %w", name, err)
		}

		if !exists {
			fmt.Printf("[Crawler] 🏗️ Criando coleção inexistente: %s (Dim: %d)\n", name, dimension)
			runtime.EventsEmit(c.ctx, "agent:log", map[string]string{
				"source":  "CRAWLER",
				"content": fmt.Sprintf("🏗️ Preparando infraestrutura: Criando coleção '%s' (%d dim)...", name, dimension),
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

