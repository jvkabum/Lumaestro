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
	utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
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

		// 📁 Se for diretório, emite como um Planeta ou Sistema Solar
		if info.IsDir() {
			folderID := "planet:" + strings.ToLower(relPath)
			folderName := info.Name()
			
			// Determina o Pai (Parent) para criar aresta de órbita
			parentDir := filepath.Dir(relPath)
			var parentID string
			celestialType := "planet"
			mass := 20.0

			if parentDir == "." {
				parentID = galaxyID
				celestialType = "solar-system-core" // Pastas raiz do vault são sistemas solares
				mass = 50.0
			} else {
				parentID = "planet:" + strings.ToLower(parentDir)
			}

			if !processedFolders[folderID] {
				utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
					"id":            folderID,
					"name":          folderName,
					"document-type": "folder",
					"celestial-type": celestialType,
					"mass":          mass,
					"parent_gravity_id": parentID,
					"summary":       fmt.Sprintf("Entidade astronômica '%s' (Tipo: %s).", folderName, celestialType),
					"what-it-does":  "Atua como centro de gravidade local para documentos e subpastas orbitais.",
				})
				// Aresta de Órbita Física (Parentesco)
				utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
					"source": parentID,
					"target": folderID,
					"weight": 5, // Aresta forte de gravidade
					"edge-type": "orbital",
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

		// Aresta de Órbita da Lua ao seu Planeta (Pasta)
		parentDir := filepath.Dir(relPath)
		var parentID string
		if parentDir == "." {
			parentID = galaxyID
		} else {
			parentID = "planet:" + strings.ToLower(parentDir)
		}

		utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
			"source": parentID,
			"target": nodeID,
			"weight": 3, // Gravidade local
			"edge-type": "orbital",
		})

		// Lê conteúdo (md/código) para gerar resumo real e extrair links
		var fileSummary, fileWhatItDoes string
		if isMD || isCode {
			rawContent, readErr := os.ReadFile(path)
			if readErr == nil {
				content := string(rawContent)
				fileSummary, fileWhatItDoes = extractFileSummary(nodeName, ext, content)

				links := extractLinks(content)
				for _, target := range links {
					utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
						"source": nodeID,
						"target": strings.ToLower(target),
						"weight": 1, // Link semântico
					})
				}

				// Cache inteligente — ignora arquivos não modificados
				hash := contentHash(rawContent)
				c.mu.Lock()
				cachedHash, exists := c.cache[path]
				c.mu.Unlock()

				if exists && cachedHash == hash {
					atomic.AddInt32(&totalCached, 1)
					// Emite nó com resumo real mesmo para arquivos em cache
					utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
						"id":             nodeID,
						"name":           nodeName,
						"document-type":  docType,
						"celestial-type": "moon",
						"mass":           5.0,
						"parent_gravity_id": parentID,
						"summary":        fileSummary,
						"what-it-does":   fileWhatItDoes,
					})
					return nil
				}
			}
		}

		if fileSummary == "" {
			ext2 := strings.ToUpper(strings.TrimPrefix(ext, "."))
			fileSummary = fmt.Sprintf("Arquivo %s: %s.", ext2, nodeName)
			fileWhatItDoes = fmt.Sprintf("Mídia '%s' indexada para análise visual e extração de conteúdo.", nodeName)
		}

		// Emite a Lua com resumo gerado a partir do conteúdo real
		utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
			"id":             nodeID,
			"name":           nodeName,
			"document-type":  docType,
			"celestial-type": "moon",
			"mass":           5.0,
			"parent_gravity_id": parentID,
			"summary":        fileSummary,
			"what-it-does":   fileWhatItDoes,
		})

		pendingFiles = append(pendingFiles, crawlTask{path: path, info: info, docType: docType})
		return nil
	})

	fmt.Printf("[Crawler] ⚡ Cosmos montado: %d objetos celestiais. %d arquivos pendentes para IA.\n", totalCached+int32(len(pendingFiles)), len(pendingFiles))

	// ══════════════════════════════════════════════════════════
	// FASE 2: PROCESSAMENTO SEMÂNTICO (API — Workers Limitados)
	// ══════════════════════════════════════════════════════════
	if len(pendingFiles) == 0 {
		fmt.Println("[Crawler] ✅ Nenhum arquivo novo ou modificado. Scan completo sem chamadas de API!")
		utils.SafeEmit(c.ctx, "agent:log", map[string]string{
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
	utils.SafeEmit(c.ctx, "agent:log", map[string]string{
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
		utils.SafeEmit(c.ctx, "agent:log", map[string]string{
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
		utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
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

			// 📁 Emitir Pasta (Planeta ou Sistema Solar)
			if info.IsDir() {
				folderID := "planet:" + strings.ToLower(repo.CoreNode+":"+relPath)
				folderName := info.Name()

				parentDir := filepath.Dir(relPath)
				var parentID string
				celestialType := "planet"
				mass := 15.0

				if parentDir == "." {
					parentID = galaxyID
					celestialType = "solar-system-core"
					mass = 40.0
				} else {
					parentID = "planet:" + strings.ToLower(repo.CoreNode+":"+parentDir)
				}

				if !processedFolders[folderID] {
					utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
						"id":            folderID,
						"name":          folderName,
						"document-type": "folder",
						"celestial-type": celestialType,
						"mass":          mass,
						"parent_gravity_id": parentID,
						"summary":       fmt.Sprintf("Entidade astronômica '%s' do repositório satélite.", folderName),
						"what-it-does":  "Atua como centro de gravidade local em uma galáxia externa.",
					})
					utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
						"source": parentID,
						"target": folderID,
						"weight": 5,
						"edge-type": "orbital",
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

			// Gera resumo real a partir do conteúdo do arquivo
			fileSummary, fileWhatItDoes := func() (string, string) {
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					return fmt.Sprintf("Arquivo '%s' do repositório satélite.", nodeName),
						"Sem conteúdo legível disponível."
				}
				return extractFileSummary(nodeName, ext, string(raw))
			}()

			// Determina o Pai (Parent) para criar aresta de órbita
			parentDir := filepath.Dir(relPath)
			var parentID string
			if parentDir == "." {
				parentID = galaxyID
			} else {
				parentID = "planet:" + strings.ToLower(repo.CoreNode+":"+parentDir)
			}

			// Emite a Lua do Projeto com resumo individual
			utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
				"id":             nodeID,
				"name":           nodeName,
				"document-type":  docType,
				"celestial-type": "moon",
				"mass":           4.0,
				"parent_gravity_id": parentID,
				"summary":        fileSummary,
				"what-it-does":   fileWhatItDoes,
			})

			utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
				"source": parentID,
				"target": nodeID,
				"weight": 3,
				"edge-type": "orbital",
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
			utils.SafeEmit(c.ctx, "agent:log", map[string]string{
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
		// Para notas gigantes, trunca para evitar estouro de contexto (Erro 500)
		if len(textContent) >= 100 {
			// Truncamento de Segurança para Lógica (Max 6k chars no Qwen)
			safeText := textContent
			if len(safeText) > 6000 {
				safeText = safeText[:6000]
			}
			
			contextHint := fmt.Sprintf("Arquivo: %s. Contexto inicial: %s", nodeName, firstLines(safeText, 500))
			triples, err = c.Ontology.ExtractTriples(ctx, safeText, contextHint)
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

		utils.SafeEmit(c.ctx, "agent:log", map[string]string{
			"source":  "CRAWLER",
			"content": fmt.Sprintf("👁️ Analisando mídia: %s...", info.Name()),
		})

		desc, tri, errMedia := c.Ontology.ProcessMedia(ctx, rawContent, mimeType)
		if errMedia == nil {
			textContent = desc
			triples = tri
		}
	}

	// Emite arestas das triplas extraídas para o grafo visual (Asteroides)
	for _, t := range triples {
		if t.Object != "" && len(t.Object) < 50 {
			utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
				"id":            strings.ToLower(t.Object),
				"name":          t.Object,
				"document-type": "keyword",
				"celestial-type": "asteroid",
				"mass":          1.0,
				"parent_gravity_id": nodeID,
			})
			utils.SafeEmit(c.ctx, "graph:edge", map[string]interface{}{
				"source": nodeID,
				"target": strings.ToLower(t.Object),
				"weight": 1,
				"edge-type": "semantic",
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
		// Truncamento de Segurança para Embeddings (Max 2k chars)
		// Nota: Idealmente faríamos chunk-averaging, mas truncar previne o Erro 500 no llama-server.
		safeEmbedText := textContent
		if len(safeEmbedText) > 2000 {
			safeEmbedText = safeEmbedText[:2000]
		}
		vector, err = c.Embedder.GenerateEmbedding(ctx, safeEmbedText, false)
	}

	if err != nil {
		fmt.Printf("[Crawler] ⚠️ Embedding falhou para %s: %s\n", nodeName, utils.FormatGenAIError(err))
		return true, nil
	}

	// Gera resumo individual a partir do conteúdo processado
	var nodeSummary, nodeWhatItDoes string
	if isImage || isPDF {
		if textContent != "" {
			nodeSummary = clampStr(textContent, 220)
			ext2 := strings.ToUpper(strings.TrimPrefix(ext, "."))
			nodeWhatItDoes = fmt.Sprintf("Mídia %s com conteúdo extraído via visão computacional.", ext2)
		} else {
			nodeSummary = fmt.Sprintf("Arquivo %s: %s.", strings.ToUpper(strings.TrimPrefix(ext, ".")), nodeName)
			nodeWhatItDoes = "Arquivo de mídia sem descrição extraída."
		}
	} else {
		nodeSummary, nodeWhatItDoes = extractFileSummary(nodeName, ext, textContent)
	}

	// Persistência no Qdrant (inclui campos de resumo para SyncAllNodes futuro)
	c.Qdrant.UpsertPoint("obsidian_knowledge", uint64(time.Now().UnixNano()), vector, map[string]interface{}{
		"path": path, "name": nodeName, "content": textContent,
		"triples": triples, "links": links, "type": ext,
		"document-type": forcedDocType, "status": "active",
		"observed_at":   time.Now().Format(time.RFC3339),
		"summary":       nodeSummary,
		"what-it-does":  nodeWhatItDoes,
	})

	// Emite nó atualizado com resumo real (garante que o grafo exibe conteúdo indexado)
	utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
		"id":            nodeID,
		"document-type": forcedDocType,
		"summary":       nodeSummary,
		"what-it-does":  nodeWhatItDoes,
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
			utils.SafeEmit(c.ctx, "agent:log", map[string]string{
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

// clampStr trunca texto em limit caracteres adicionando "..." se necessário.
func clampStr(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return strings.TrimSpace(s[:limit-3]) + "..."
}

// extractFileSummary lê o conteúdo real do arquivo e extrai um resumo individual por tipo.
// Zero chamadas de API — extração puramente baseada no conteúdo do arquivo.
func extractFileSummary(name, ext, content string) (summary, whatItDoes string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Sprintf("Arquivo '%s'.", name), "Conteúdo vazio ou não textual."
	}
	switch strings.ToLower(ext) {
	case ".md":
		return extractMarkdownSummary(name, content)
	case ".go":
		return extractGoSummary(name, content)
	case ".js", ".jsx", ".ts", ".tsx":
		return extractJSSummary(name, content)
	case ".py":
		return extractPySummary(name, content)
	case ".html":
		return extractHTMLSummary(name, content)
	case ".css":
		return extractCSSSummary(name, content)
	default:
		return extractGenericSummary(name, content)
	}
}

func extractMarkdownSummary(name, content string) (string, string) {
	lines := strings.Split(content, "\n")
	var heading, firstPara string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") && heading == "" {
			heading = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}
		if !strings.HasPrefix(line, "#") && firstPara == "" {
			firstPara = line
			break
		}
	}
	if heading == "" {
		heading = name
	}
	if firstPara == "" {
		firstPara = heading
	}
	sum := heading
	if firstPara != heading {
		sum = heading + ": " + firstPara
	}
	return clampStr(sum, 220), clampStr(firstPara, 180)
}

func extractGoSummary(name, content string) (string, string) {
	lines := strings.Split(content, "\n")
	var pkgDoc []string
	var exports []string
	var pkgName string
	inPkgComment := true

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "package ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				pkgName = parts[1]
			}
			inPkgComment = false
			continue
		}
		if inPkgComment && strings.HasPrefix(trimmed, "//") {
			doc := strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
			if doc != "" {
				pkgDoc = append(pkgDoc, doc)
			}
			continue
		}
		if trimmed == "" {
			inPkgComment = false
		}
		if len(exports) < 5 {
			for _, kw := range []string{"func ", "type ", "var ", "const "} {
				if strings.HasPrefix(trimmed, kw) {
					rest := strings.TrimPrefix(trimmed, kw)
					parts := strings.Fields(rest)
					if len(parts) > 0 && len(parts[0]) > 0 && parts[0][0] >= 'A' && parts[0][0] <= 'Z' {
						id := parts[0]
						if idx := strings.IndexAny(id, "([{"); idx > 0 {
							id = id[:idx]
						}
						exports = append(exports, id)
					}
					break
				}
			}
		}
	}

	var sum string
	if len(pkgDoc) > 0 {
		sum = strings.Join(pkgDoc, " ")
	} else if pkgName != "" {
		sum = fmt.Sprintf("Pacote Go '%s'.", pkgName)
	} else {
		sum = fmt.Sprintf("Arquivo Go: %s.", name)
	}

	var what string
	if len(exports) > 0 {
		what = fmt.Sprintf("Define: %s.", strings.Join(exports, ", "))
	} else if pkgName != "" {
		what = fmt.Sprintf("Implementa lógica do pacote '%s'.", pkgName)
	} else {
		what = "Código Go do backend da aplicação."
	}
	return clampStr(sum, 220), clampStr(what, 180)
}

func extractJSSummary(name, content string) (string, string) {
	lines := strings.Split(content, "\n")
	var commentLines []string
	var exports []string
	inBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "/*") {
			inBlock = true
			continue
		}
		if inBlock {
			if strings.Contains(trimmed, "*/") {
				inBlock = false
				continue
			}
			clean := strings.TrimLeft(trimmed, "* ")
			if clean != "" && !strings.HasPrefix(clean, "@") {
				commentLines = append(commentLines, clean)
			}
			continue
		}
		if len(commentLines) == 0 && strings.HasPrefix(trimmed, "// ") {
			commentLines = append(commentLines, strings.TrimPrefix(trimmed, "// "))
			continue
		}
		if len(exports) < 5 && strings.HasPrefix(trimmed, "export ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				id := parts[2]
				if idx := strings.IndexAny(id, "({<"); idx > 0 {
					id = id[:idx]
				}
				if id != "" && id != "from" && id != "default" {
					exports = append(exports, id)
				}
			}
		}
	}

	var sum string
	n := min(3, len(commentLines))
	if n > 0 {
		sum = strings.Join(commentLines[:n], " ")
	} else {
		sum = fmt.Sprintf("Módulo JS/TS: %s.", name)
	}

	var what string
	if len(exports) > 0 {
		what = fmt.Sprintf("Exporta: %s.", strings.Join(exports, ", "))
	} else {
		what = fmt.Sprintf("Módulo '%s' — componente ou utilitário da interface.", name)
	}
	return clampStr(sum, 220), clampStr(what, 180)
}

func extractPySummary(name, content string) (string, string) {
	lines := strings.Split(content, "\n")
	var docLines []string
	var defs []string
	inDoc := false
	docQuote := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inDoc {
			if strings.HasPrefix(trimmed, `"""`) || strings.HasPrefix(trimmed, `'''`) {
				docQuote = trimmed[:3]
				inDoc = true
				rest := strings.TrimPrefix(trimmed, docQuote)
				rest = strings.TrimSuffix(rest, docQuote)
				if rest = strings.TrimSpace(rest); rest != "" {
					docLines = append(docLines, rest)
				}
				if strings.Count(trimmed, docQuote) >= 2 {
					inDoc = false
				}
				continue
			}
		} else {
			if strings.Contains(trimmed, docQuote) {
				part := strings.Split(trimmed, docQuote)[0]
				if part = strings.TrimSpace(part); part != "" {
					docLines = append(docLines, part)
				}
				inDoc = false
				continue
			}
			if trimmed != "" {
				docLines = append(docLines, trimmed)
			}
			if len(docLines) >= 3 {
				inDoc = false
			}
			continue
		}
		if len(defs) < 5 {
			for _, kw := range []string{"async def ", "def ", "class "} {
				if strings.HasPrefix(trimmed, kw) {
					rest := strings.TrimPrefix(trimmed, kw)
					parts := strings.Fields(rest)
					if len(parts) > 0 {
						id := parts[0]
						if idx := strings.IndexAny(id, "(:{"); idx > 0 {
							id = id[:idx]
						}
						defs = append(defs, id)
					}
					break
				}
			}
		}
	}

	var sum string
	if len(docLines) > 0 {
		sum = strings.Join(docLines, " ")
	} else {
		sum = fmt.Sprintf("Módulo Python: %s.", name)
	}

	var what string
	if len(defs) > 0 {
		what = fmt.Sprintf("Define: %s.", strings.Join(defs, ", "))
	} else {
		what = fmt.Sprintf("Script Python '%s'.", name)
	}
	return clampStr(sum, 220), clampStr(what, 180)
}

func extractHTMLSummary(name, content string) (string, string) {
	titleRe := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	h1Re := regexp.MustCompile(`(?i)<h1[^>]*>([^<]+)</h1>`)
	title := name
	if m := titleRe.FindStringSubmatch(content); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	} else if m := h1Re.FindStringSubmatch(content); len(m) > 1 {
		title = strings.TrimSpace(m[1])
	}
	return fmt.Sprintf("Página HTML: %s.", title),
		fmt.Sprintf("Template de interface que renderiza '%s'.", title)
}

func extractCSSSummary(name, content string) (string, string) {
	ruleCount := strings.Count(content, "{")
	return fmt.Sprintf("Folha de estilos CSS: %s (%d regras).", name, ruleCount),
		fmt.Sprintf("Define %d regras de estilos visuais para a interface.", ruleCount)
}

func extractGenericSummary(name, content string) (string, string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#!") {
			return clampStr(line, 220), fmt.Sprintf("Arquivo de configuração ou dados: %s.", name)
		}
	}
	return fmt.Sprintf("Arquivo: %s.", name), "Arquivo de suporte do projeto."
}

