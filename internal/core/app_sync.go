package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/utils"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ScanVault percorre o Obsidian e indexa no Qdrant com Embeddings
func (a *App) ScanVault() string {
	fmt.Println("[BACKEND] 🚀 Comando ScanVault recebido.")
	if a.ctx == nil {
		return "⚠️ Sincronização indisponível: contexto do app ainda não inicializado."
	}

	if a.crawler == nil {
		fmt.Println("[BACKEND] ⚠️ Crawler é nil. Tentando inicializar...")
		_ = a.initServices()
	}

	if a.crawler == nil {
		return "⚠️ Sync Obsidian 3D bloqueado: crawler não pôde ser inicializado."
	}

	if a.ctx != nil {
		a.crawler.SetContext(a.ctx)
	}

	if a.IsScanning {
		return "⚠️ Scan já em progresso."
	}

	a.IsScanning = true
	// 🕵️⚡ RAG em Segundo Plano
	go func() {
		defer func() { a.IsScanning = false }()
		fmt.Println("[BACKEND] 🕵️ Iniciando Scan em segundo plano...")

		// Captura local
		crawler := a.crawler
		ctx := a.ctx
		qdrant := a.qdrant

		// 1. Verificação Crítica de Motor e Contexto
		if crawler == nil || ctx == nil || qdrant == nil {
			fmt.Println("[BACKEND] ⏳ Scan ABORTADO: Motores em transição ou offline.")
			return
		}

		crawler.SetContext(ctx)

		err := crawler.IndexVault(ctx)
		if err != nil {
			fmt.Printf("[BACKEND] Erro na Indexação do Vault: %v\n", err)
			a.emitEvent("agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Erro na Indexação do Obsidian: " + err.Error(),
			})
			return
		}

		// ⚡ Sync intermediário: mostra as notas do Obsidian imediatamente na UI
		a.SyncAllNodes()

		// 2. Indexar a documentação do projeto (Lumaestro Core)
		// 🛡️ SEGURANÇA: Só indexa o sistema se o CPI estiver armado e autorizado (Prevenção de vazamento de código)
		if a.executor.CPI.IsArmed() {
			fmt.Println("[BACKEND] Indexando documentos internos do sistema...")
			err = crawler.IndexSystemDocs(ctx, "./")
			if err != nil {
				fmt.Printf("[BACKEND] Aviso: Erro ao indexar docs do sistema: %v\n", err)
			}
		} else {
			fmt.Println("[BACKEND] 🛡️ Segurança: Indexação de documentos do sistema bloqueada (CPI Desarmado).")
		}

		// 3. Indexar Repositórios Dinâmicos e o Workspace Ativo (Devorador de Código)
		projectsToScan := append([]config.ProjectScan{}, a.config.ExternalProjects...)

		// 🚀 INTEGRAÇÃO WORKSPACE: Se houver um workspace ativo e o CPI estiver armado, ele entra como prioridade no Devorador
		if a.executor.Workspace != "" && a.executor.CPI.IsArmed() {
			projectName := filepath.Base(a.executor.Workspace)
			projectsToScan = append(projectsToScan, config.ProjectScan{
				Path:        a.executor.Workspace,
				CoreNode:    projectName,
				IncludeCode: true, // Força o "Devorador de Código" no workspace
			})
			fmt.Printf("[Sync] 📂 Workspace '%s' adicionado à fila do Devorador de Código.\n", projectName)
		}

		if len(projectsToScan) > 0 {
			fmt.Println("[BACKEND] Iniciando expansão radial (Projetos satélites e Workspace)...")
			err = a.crawler.IndexRepositories(a.ctx, projectsToScan)
			if err != nil {
				fmt.Printf("[BACKEND] Erro ao sincronizar projetos: %v\n", err)
			}
		}

		// 3. Força a atualização visual de todos os nós (isolados e conectados)
		a.SyncAllNodes()
	}()

	return "Indexação iniciada em segundo plano. O Maestro agora está integrando seu Obsidian e as memórias do sistema."
}

// FullSync limpa o cache e inicia uma indexação completa atômica (Alias para compatibilidade).
func (a *App) FullSync() string {
	return a.ExecuteFullSync()
}

// ExecuteFullSync limpa o cache e inicia uma indexação completa atômica.
func (a *App) ExecuteFullSync() string {
	if a.crawler == nil {
		_ = a.initServices()
	}
	if a.crawler == nil {
		return "⚠️ Motor de indexação indisponível: sem provedor de embeddings ativo."
	}
	fmt.Println("[BACKEND] 🔄 Solicitado FullSync Atômico. Limpando cache...")
	a.crawler.PurgeCache()
	return a.ScanVault()
}

// AddExternalProject vincula um repositório inteiro e o expande via Crawler Radial
func (a *App) AddExternalProject(path string, coreNode string, includeCode bool) map[string]interface{} {
	cfg, err := config.Load()
	if err != nil {
		return map[string]interface{}{"success": false, "error": "Erro de config interno"}
	}

	for _, p := range cfg.ExternalProjects {
		if p.Path == path {
			return map[string]interface{}{"success": false, "error": "Repositório já mapeado!"}
		}
	}

	cfg.ExternalProjects = append(cfg.ExternalProjects, config.ProjectScan{
		Path:        path,
		CoreNode:    coreNode,
		IncludeCode: includeCode,
	})

	config.Save(*cfg)
	a.config = cfg

	// Dispara a sincronização imediatamente e de forma limpa (Sincronizando Nodes via EventsEmit com ScanVault)
	_ = a.ScanVault()

	return map[string]interface{}{"success": true, "message": "Projetos satélite vinculados e auto-scan de gravidade acionado."}
}

// ToggleProjectCodeRAG alterna entre modo Documentação e Código Fonte para um repositório existente
func (a *App) ToggleProjectCodeRAG(path string) map[string]interface{} {
	cfg, err := config.Load()
	if err != nil {
		return map[string]interface{}{"success": false, "error": "Erro de config interno"}
	}

	found := false
	for i, p := range cfg.ExternalProjects {
		if p.Path == path {
			cfg.ExternalProjects[i].IncludeCode = !p.IncludeCode
			found = true
			break
		}
	}

	if !found {
		return map[string]interface{}{"success": false, "error": "Projeto não encontrado"}
	}

	config.Save(*cfg)
	a.config = cfg

	// Re-sincroniza o grafo para refletir a nova profundidade semântica
	_ = a.ScanVault()

	return map[string]interface{}{"success": true, "message": "Modo de análise do projeto atualizado."}
}

// GetExternalProjects retorna os repositórios em formato JSON para Renderização no frontend (Settings)
func (a *App) GetExternalProjects() []config.ProjectScan {
	if a.config != nil {
		return a.config.ExternalProjects
	}
	return []config.ProjectScan{}
}

// SelectDirectory abre o explorador de arquivos nativo do S.O. para escolher uma pasta
func (a *App) SelectDirectory() string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Selecione o Repositório do Projeto",
	})
	if err != nil {
		return ""
	}
	return dir
}

// ResetQdrantDB apaga permanentemente o banco de dados remoto e limpa o cache local.
func (a *App) ResetQdrantDB() string {
	if a.qdrant == nil || a.ctx == nil {
		return "⚠️ Erro: Cliente Qdrant não inicializado."
	}

	fmt.Println("[RESET] 🚨 Iniciando Reset do Banco de Dados Qdrant...")

	collections := []string{"obsidian_knowledge", "knowledge_graph"}
	for _, name := range collections {
		err := a.qdrant.DeleteCollection(name)
		if err != nil {
			fmt.Printf("[RESET] Erro ao excluir %s: %v\n", name, err)
			continue
		}
		fmt.Printf("[RESET] ✅ Coleção %s excluída.\n", name)
	}

	// 2. Limpa Cache Local e Analítico
	if a.crawler != nil {
		fmt.Println("[RESET] 🧹 Limpando cache do Crawler...")
		a.crawler.PurgeCache()
	}
	
	// 🔥 RESET PROFUNDO: DuckDB e Motor em RAM
	if a.LStore != nil {
		fmt.Println("[RESET] 🧹 Limpando Grafos Analíticos (DuckDB)...")
		_ = a.LStore.ClearGraph()
	}
	if a.GEngine != nil {
		fmt.Println("[RESET] 🧠 Zerando Motor de Grafos (RAM)...")
		a.GEngine.Clear()
	}

	os.Remove(".lumaestro/cache/topology.json") // Expurga cache visual 3D

	// 3. Recria Infraestrutura do zero
	dim := 3072
	if a.config != nil && a.config.EmbeddingDimension > 0 {
		dim = a.config.EmbeddingDimension
	}
	fmt.Printf("[RESET] 🏗️ Recriando infraestrutura (%d dim)...\n", dim)
	if a.crawler != nil {
		a.crawler.EnsureCollections(a.ctx)
	}

	// 4. Notifica o Frontend (Log + Limpeza de Tela)
	a.emitEvent("agent:log", map[string]string{
		"source":  "SYSTEM",
		"content": "☢️ RESET COMPLETO: Banco de dados, DuckDB e cache local foram expurgados.",
	})
	a.emitEvent("graph:clear", nil)

	return "✅ O banco de dados foi resetado com sucesso! Inicie um novo SCAN para repovoar."
}

// PurgeCache limpa todo o histórico de indexação local.
func (a *App) PurgeCache() string {
	os.Remove(".lumaestro/cache/topology.json") // Invalida Topology Cache
	if a.crawler == nil {
		return "⚠️ Motor de indexação indisponível."
	}
	err := a.crawler.PurgeCache()
	if err != nil {
		return fmt.Sprintf("Erro ao limpar cache: %v", err)
	}
	return "Cache de indexação limpo com sucesso!"
}

// TopologyCache representa o snapshot completo do grafo para carregamento instantâneo.
type TopologyCache struct {
	Workspace string                   `json:"workspace"`
	Nodes     []map[string]interface{} `json:"nodes"`
	Edges     []map[string]interface{} `json:"edges"`
}

// Retorna o caminho do cache isolado por workspace
func (a *App) getTopologyCachePath() string {
	targetWs := a.getActiveWorkspace()
	if targetWs == "" && a.config != nil {
		targetWs = a.config.ObsidianVaultPath
	}
	if targetWs == "" {
		return ".lumaestro/cache/topology.json"
	}
	h := sha256.New()
	h.Write([]byte(filepath.Clean(targetWs)))
	hash := hex.EncodeToString(h.Sum(nil))[:8]
	os.MkdirAll(".lumaestro/cache", 0755)
	return fmt.Sprintf(".lumaestro/cache/topology_%s.json", hash)
}

// Sincronização e I/O Desacoplado do Motor Físico
func (a *App) saveTopologyCache(nodes []map[string]interface{}, edges []map[string]interface{}) {
	// 🛡️ Proteção contra gravação de cache vazio que apagaria dados anteriores
	if len(nodes) == 0 {
		fmt.Println("[Sync] ⚠️ Ignorando gravação de cache vazio (0 nós). Cache anterior preservado.")
		return
	}
	cachePath := a.getTopologyCachePath()
	cache := TopologyCache{
		Workspace: a.getActiveWorkspace(),
		Nodes:     nodes,
		Edges:     edges,
	}
	data, err := json.Marshal(cache)
	if err == nil {
		os.WriteFile(cachePath, data, 0644)
	}
}

func (a *App) loadTopologyCache() *TopologyCache {
	targetWs := a.getActiveWorkspace()
	if targetWs == "" && a.config != nil {
		targetWs = a.config.ObsidianVaultPath
	}

	cachePath := a.getTopologyCachePath()
	data, err := os.ReadFile(cachePath)
	if err != nil {
		// 🔎 Varredura proativa: busca qualquer topology_*.json cujo workspace seja compatível
		files, _ := filepath.Glob(".lumaestro/cache/topology_*.json")
		for _, f := range files {
			d, readErr := os.ReadFile(f)
			if readErr == nil {
				var candidate TopologyCache
				if json.Unmarshal(d, &candidate) == nil {
					if candidate.Workspace != "" && targetWs != "" && strings.EqualFold(filepath.Clean(candidate.Workspace), filepath.Clean(targetWs)) {
						data = d
						err = nil
						break
					}
				}
			}
		}
	}
	if err != nil {
		data, err = os.ReadFile(".lumaestro/cache/topology.json")
		if err != nil {
			return nil
		}
	}
	var cache TopologyCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	// Se o cache for de outro workspace, descarta para evitar contaminação
	if cache.Workspace != "" && targetWs != "" && !strings.EqualFold(filepath.Clean(cache.Workspace), filepath.Clean(targetWs)) {
		return nil
	}
	return &cache
}

// LoadFastGraph realiza o "Início a Frio": emite o grafo do cache/DuckDB instantaneamente
func (a *App) LoadFastGraph() {
	// Aguarda um instante breve para o Wails Handshake estabilizar
	time.Sleep(200 * time.Millisecond)
	
	fmt.Println("[Sync] ⚡ Acionando Início a Frio (Fast-Track)...")

	// 1. Tenta carregar do Cache de Topologia (Nós + Arestas)
	cache := a.loadTopologyCache()
	if cache != nil && len(cache.Nodes) > 0 {
		fmt.Printf("[Sync] 🚀 Emitindo %d nós do cache para carregamento instantâneo.\n", len(cache.Nodes))
		a.emitEvent("graph:nodes:batch", cache.Nodes)

		// Delay curto para garantir que o Deck.gl montou a camada de nós antes das arestas
		time.Sleep(250 * time.Millisecond)
		fmt.Printf("[Sync] 🚀 Emitindo %d arestas do cache.\n", len(cache.Edges))
		a.emitEvent("graph:edges:batch", cache.Edges)
		return
	}

	// 2. Fallback: Se não houver cache, tenta ler do DuckDB (Nós + Arestas)
	if a.LStore != nil {
		targetWs := a.getActiveWorkspace()
		if targetWs == "" && a.config != nil {
			targetWs = a.config.ObsidianVaultPath
		}
		nodes, edges, err := a.LStore.GetFullGraph(targetWs)
		if err == nil && len(nodes) > 0 {
			fmt.Printf("[Sync] 💾 Fallback: Emitindo %d nós do DuckDB.\n", len(nodes))
			a.emitEvent("graph:nodes:batch", nodes)
			if len(edges) > 0 {
				time.Sleep(300 * time.Millisecond)
				fmt.Printf("[Sync] 💾 Fallback: Emitindo %d arestas do DuckDB.\n", len(edges))
				a.emitEvent("graph:edges:batch", edges)
			}
			return
		}
	}

	// 3. Auto-Scan se o workspace nunca foi indexado (0 nós no cache e no DuckDB)
	if a.crawler != nil && a.getActiveWorkspace() != "" && !a.IsScanning {
		fmt.Println("[Sync] 🚀 Primeiro acesso ao workspace: disparando auto-scan estrutural...")
		go a.ScanVault()
	}
}

// UpdateNodePositions recebe as coordenadas atuais do Frontend e persiste no DuckDB e Cache.
func (a *App) UpdateNodePositions(nodes []map[string]interface{}) string {
	// 🛡️ Proteção: Nunca gravar 0 nós (destruiria o cache existente)
	if len(nodes) == 0 {
		return "Nenhum nó para atualizar."
	}
	fmt.Printf("[Sync] 💾 Recebendo atualização de layout para %d nós...\n", len(nodes))

	// 1. Persistência Analítica (DuckDB)
	if a.LStore != nil {
		err := a.LStore.UpdateNodePositions(nodes)
		if err != nil {
			fmt.Printf("[Sync] ❌ Erro ao salvar posições no DuckDB: %v\n", err)
		}
	}

	// 2. Persistência de Carregamento Rápido (Topology Cache)
	cache := a.loadTopologyCache()
	targetWs := a.getActiveWorkspace()
	if targetWs == "" && a.config != nil {
		targetWs = a.config.ObsidianVaultPath
	}

	// Recupera arestas existentes do cache ou do DuckDB para NUNCA zerar as arestas
	var edgesToKeep []map[string]interface{}
	if cache != nil && len(cache.Edges) > 0 {
		edgesToKeep = cache.Edges
	} else if a.LStore != nil {
		_, edges, _ := a.LStore.GetFullGraph(targetWs)
		edgesToKeep = edges
	}

	if cache == nil {
		a.saveTopologyCache(nodes, edgesToKeep)
	} else {
		nodeMap := make(map[string]int)
		for i, n := range cache.Nodes {
			if id, ok := n["id"].(string); ok {
				nodeMap[id] = i
			}
		}

		for _, n := range nodes {
			id, _ := n["id"].(string)
			if idx, exists := nodeMap[id]; exists {
				// Atualiza posição de nó existente
				cache.Nodes[idx]["x"] = n["x"]
				cache.Nodes[idx]["y"] = n["y"]
				cache.Nodes[idx]["z"] = n["z"]
			} else {
				// Adiciona novo nó descoberto ao cache garantindo metadados mínimos
				if _, hasName := n["name"]; !hasName && id != "" {
					parts := strings.Split(id, ":")
					if len(parts) >= 3 {
						n["name"] = filepath.Base(parts[2])
						n["celestial-type"] = parts[0]
					}
				}
				cache.Nodes = append(cache.Nodes, n)
			}
		}
		if len(cache.Edges) == 0 && len(edgesToKeep) > 0 {
			cache.Edges = edgesToKeep
		}
		a.saveTopologyCache(cache.Nodes, cache.Edges)
	}

	return fmt.Sprintf("Layout sincronizado com sucesso (%d nós atualizados).", len(nodes))
}

// SyncAllNodes percorre o banco de dados e emite cada nota para o visualizador 3D.
func (a *App) SyncAllNodes() {
	// ⏳ Aguarda brevemente caso os motores vitais ainda estejam inicializando no boot
	for i := 0; i < 6; i++ {
		if a.qdrant != nil && a.ctx != nil && a.GEngine != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if a.qdrant == nil || a.ctx == nil || a.GEngine == nil {
		fmt.Println("[Sync] ⚠️ Sincronização cancelada: Motores vitais indisponíveis.")
		return
	}

	targetWs := a.getActiveWorkspace()
	if targetWs == "" && a.config != nil {
		targetWs = a.config.ObsidianVaultPath
	}

	// ⚡ Carrega posições salvas do DuckDB para merge
	savedPositions := make(map[string][]float64)
	if a.LStore != nil {
		nodes, _, _ := a.LStore.GetFullGraph(targetWs)
		for _, n := range nodes {
			id, _ := n["id"].(string)
			x, _ := n["x"].(float64)
			y, _ := n["y"].(float64)
			z, _ := n["z"].(float64)
			savedPositions[id] = []float64{x, y, z}
		}
	}

	nodesBatch := make([]map[string]interface{}, 0)
	edgesBatch := make([]map[string]interface{}, 0)
	batchIndex := map[string]struct{}{}
	batchIndexPos := map[string]int{}
	edgeIndex := map[string]struct{}{}
	nameToID := make(map[string]string)

	addNode := func(node map[string]interface{}) {
		id, _ := node["id"].(string)
		if id == "" {
			return
		}
		if _, exists := batchIndex[id]; exists {
			return
		}
		batchIndex[id] = struct{}{}
		batchIndexPos[id] = len(nodesBatch)
		nodesBatch = append(nodesBatch, node)
	}

	addEdge := func(source, target string, weight float64, relType string) {
		if source == "" || target == "" || source == target {
			return
		}
		pairID := fmt.Sprintf("%s->%s", source, target)
		if _, exists := edgeIndex[pairID]; exists {
			return
		}
		edgeIndex[pairID] = struct{}{}

		edge := map[string]interface{}{
			"source":    source,
			"target":    target,
			"weight":    weight,
			"edge-type": relType,
		}
		edgesBatch = append(edgesBatch, edge)
	}

	// 🪐 FASE 1: BASE ESTRUTURAL DO WORKSPACE (DuckDB e Cache Local)
	// Garante que todas as centenas de arquivos, pastas, texturas e luas do workspace (Fortress)
	// estejam SEMPRE presentes como alicerce, nunca sendo destruídos por contagens isoladas do Qdrant.
	var structuralNodes []map[string]interface{}
	var structuralEdges []map[string]interface{}

	if a.LStore != nil {
		dbNodes, dbEdges, err := a.LStore.GetFullGraph(targetWs)
		if err == nil && len(dbNodes) > 0 {
			structuralNodes = dbNodes
			structuralEdges = dbEdges
			fmt.Printf("[Sync] 📂 Carregados %d nós estruturais e %d arestas do DuckDB para '%s'.\n", len(dbNodes), len(dbEdges), targetWs)
		}
	}

	if len(structuralNodes) == 0 {
		cache := a.loadTopologyCache()
		if cache != nil && len(cache.Nodes) > 0 {
			structuralNodes = cache.Nodes
			structuralEdges = cache.Edges
			fmt.Printf("[Sync] ⚡ Carregados %d nós e %d arestas do Cache de Topologia Local.\n", len(cache.Nodes), len(cache.Edges))
		}
	}

	// Mapeador auxiliar de arquivos para pastas orbitais no workspace ativo
	fileToParentPlanet := make(map[string]string)
	if targetWs != "" {
		h := sha256.New()
		h.Write([]byte(filepath.Clean(targetWs)))
		pathHash := hex.EncodeToString(h.Sum(nil))[:6]
		galaxyID := "galaxy:" + pathHash + ":" + strings.ToLower(filepath.Base(targetWs))

		_ = filepath.Walk(targetWs, func(p string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return nil
			}
			if info.IsDir() {
				if info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == ".lumaestro" {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(targetWs, p)
			if rel == "." {
				return nil
			}
			ext := filepath.Ext(p)
			baseName := strings.ToLower(strings.TrimSuffix(info.Name(), ext))
			parentDir := filepath.Dir(rel)
			if parentDir == "." {
				fileToParentPlanet[baseName] = galaxyID
			} else {
				fileToParentPlanet[baseName] = fmt.Sprintf("planet:%s:%s", pathHash, strings.ToLower(parentDir))
			}
			return nil
		})
	}

	for _, n := range structuralNodes {
		id, _ := n["id"].(string)
		if id == "" {
			continue
		}
		name, _ := n["name"].(string)
		if name == "" {
			parts := strings.Split(id, ":")
			if len(parts) >= 3 {
				name = filepath.Base(parts[2])
			} else {
				name = id
			}
		}
		docType, _ := n["type"].(string)
		if docType == "" {
			docType, _ = n["document-type"].(string)
		}
		if docType == "" {
			docType = "source"
		}
		parent, _ := n["parent_gravity_id"].(string)
		if parent == "" {
			parent, _ = n["parent_id"].(string)
		}

		// Dedução inteligente de parentesco para planetas (estruturas de pastas)
		if parent == "" && strings.HasPrefix(id, "planet:") {
			parts := strings.SplitN(id, ":", 3)
			if len(parts) == 3 {
				hash := parts[1]
				relPath := parts[2]
				cleanRel := filepath.Clean(strings.ReplaceAll(relPath, "/", "\\"))
				dir := filepath.Dir(cleanRel)
				if dir == "." || dir == "" {
					for _, cand := range structuralNodes {
						candID, _ := cand["id"].(string)
						if strings.HasPrefix(candID, "galaxy:"+hash+":") {
							parent = candID
							break
						}
					}
					if parent == "" && targetWs != "" {
						parent = fmt.Sprintf("galaxy:%s:%s", hash, strings.ToLower(filepath.Base(targetWs)))
					}
				} else {
					parent = fmt.Sprintf("planet:%s:%s", hash, strings.ToLower(dir))
				}
			}
		}

		// Dedução inteligente de parentesco para luas (arquivos de código/mídia)
		if parent == "" && strings.HasPrefix(id, "moon:") {
			parts := strings.SplitN(id, ":", 3)
			if len(parts) == 3 {
				moonBase := strings.ToLower(parts[2])
				if pPlanet, found := fileToParentPlanet[moonBase]; found {
					parent = pPlanet
				}
			}
		}

		a.GEngine.AddNode(id, name, docType)
		nameLower := strings.ToLower(name)
		nameToID[nameLower] = id
		parts := strings.Split(id, ":")
		if len(parts) >= 3 {
			sub := strings.ToLower(parts[2])
			nameToID[sub] = id
			nameToID[strings.ReplaceAll(sub, "\\", "/")] = id
			nameToID[strings.ReplaceAll(sub, "/", "\\")] = id
			base := filepath.Base(sub)
			if _, exists := nameToID[base]; !exists {
				nameToID[base] = id
			}
		}
		if strings.HasPrefix(id, "asteroid:") {
			nameToID[strings.TrimPrefix(id, "asteroid:")] = id
		}

		nodeData := make(map[string]interface{})
		for k, v := range n {
			nodeData[k] = v
		}
		nodeData["id"] = id
		nodeData["name"] = name
		nodeData["document-type"] = docType
		if parent != "" {
			nodeData["parent_gravity_id"] = parent
		}
		if pos, exists := savedPositions[id]; exists {
			nodeData["x"] = pos[0]
			nodeData["y"] = pos[1]
			nodeData["z"] = pos[2]
		}
		addNode(nodeData)

		// Gera aresta física de órbita garantida
		if parent != "" && parent != id {
			a.GEngine.AddEdge(parent, id, 3.0, "orbital")
			addEdge(parent, id, 3.0, "orbital")
		}
	}

	for _, e := range structuralEdges {
		src, _ := e["source"].(string)
		if src == "" {
			src, _ = e["source_id"].(string)
		}
		tgt, _ := e["target"].(string)
		if tgt == "" {
			tgt, _ = e["target_id"].(string)
		}
		weight, _ := e["weight"].(float64)
		relType, _ := e["relation_type"].(string)
		if relType == "" {
			relType, _ = e["edge-type"].(string)
		}
		if relType == "" {
			relType = "orbital"
		}
		if src != "" && tgt != "" && src != tgt {
			a.GEngine.AddEdge(src, tgt, weight, relType)
			addEdge(src, tgt, weight, relType)
		}
	}

	// 🧠 FASE 2: CAMADA SEMÂNTICA (Qdrant - obsidian_knowledge)
	fmt.Println("[Sync] Consultando pontos semânticos no Qdrant...")
	points, err := a.qdrant.Search("obsidian_knowledge", nil, 10000)
	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar pontos semânticos do Qdrant: %v\n", err)
		if strings.Contains(err.Error(), "Status 404") || strings.Contains(err.Error(), "Not found") {
			if a.crawler != nil && a.ctx != nil {
				_ = a.crawler.EnsureCollections(a.ctx)
			}
		}
	} else {
		for _, p := range points {
			name, _ := p["name"].(string)
			id, _ := p["id"].(string)
			if name != "" && id != "" {
				nameToID[strings.ToLower(name)] = id
			}
		}

		for _, p := range points {
			name, _ := p["name"].(string)
			nodeID, _ := p["id"].(string)
			if nodeID == "" {
				if name == "" { continue }
				nodeID = strings.ToLower(name)
			}
			if name == "" { name = nodeID }

			summary := summarizeNodeContent(p)
			whatItDoes := inferNodePurpose(p, summary)
			docType, _ := p["document-type"].(string)
			if docType == "" { docType = "markdown" }

			if idx, exists := batchIndexPos[nodeID]; exists {
				// Enriquece o nó estrutural já existente com a inteligência do Qdrant
				nodesBatch[idx]["summary"] = summary
				nodesBatch[idx]["what-it-does"] = whatItDoes
				if dt, ok := nodesBatch[idx]["document-type"].(string); !ok || dt == "source" || dt == "" {
					nodesBatch[idx]["document-type"] = docType
				}
			} else {
				a.GEngine.AddNode(nodeID, name, docType)
				nodeData := map[string]interface{}{
					"id":            nodeID,
					"name":          name,
					"document-type": docType,
					"summary":       summary,
					"what-it-does":  whatItDoes,
				}
				if pos, exists := savedPositions[nodeID]; exists {
					nodeData["x"] = pos[0]
					nodeData["y"] = pos[1]
					nodeData["z"] = pos[2]
				}
				addNode(nodeData)
			}

			// Links Diretos [[Bracket]]
			if linksRaw, ok := p["links"].([]interface{}); ok {
				for _, target := range linksRaw {
					if t, ok := target.(string); ok && t != "" {
						targetNameLower := strings.ToLower(strings.TrimSpace(t))
						targetID := ""
						if realID, ok := nameToID[targetNameLower]; ok {
							targetID = realID
						} else if _, exists := batchIndex[targetNameLower]; exists {
							targetID = targetNameLower
						} else if realID, ok := nameToID[strings.ReplaceAll(targetNameLower, "/", "\\")]; ok {
							targetID = realID
						} else if realID, ok := nameToID[strings.ReplaceAll(targetNameLower, "\\", "/")]; ok {
							targetID = realID
						} else if _, exists := batchIndex["asteroid:"+targetNameLower]; exists {
							targetID = "asteroid:" + targetNameLower
						}

						if targetID != "" && targetID != nodeID {
							a.GEngine.AddEdge(nodeID, targetID, 1.0, "link")
							addEdge(nodeID, targetID, 1.0, "link")
						}
					}
				}
			}

			// Triplas Semânticas
			if triplesRaw, ok := p["triples"].([]interface{}); ok {
				for _, t := range triplesRaw {
					if tm, ok := t.(map[string]interface{}); ok {
						if obj, ok := tm["object"].(string); ok && obj != "" {
							targetNameLower := strings.ToLower(strings.TrimSpace(obj))
							targetID := ""
							if realID, ok := nameToID[targetNameLower]; ok {
								targetID = realID
							} else if _, exists := batchIndex[targetNameLower]; exists {
								targetID = targetNameLower
							} else if realID, ok := nameToID[strings.ReplaceAll(targetNameLower, "/", "\\")]; ok {
								targetID = realID
							} else if realID, ok := nameToID[strings.ReplaceAll(targetNameLower, "\\", "/")]; ok {
								targetID = realID
							} else if _, exists := batchIndex["asteroid:"+targetNameLower]; exists {
								targetID = "asteroid:" + targetNameLower
							}

							if targetID != "" && targetID != nodeID {
								a.GEngine.AddEdge(nodeID, targetID, 2.0, "semantic")
								addEdge(nodeID, targetID, 2.0, "semantic")
							}
						}
					}
				}
			}
		}
	}

	// 💬 FASE 3: MEMÓRIAS DE DIÁLOGO (Qdrant - knowledge_graph)
	memoryPoints, memErr := a.qdrant.Search("knowledge_graph", nil, 1500)
	if memErr != nil {
		fmt.Printf("[Sync] Aviso: Erro ao buscar memórias de chat: %v\n", memErr)
	} else {
		for _, p := range memoryPoints {
			subject, _ := p["subject"].(string)
			object, _ := p["object"].(string)
			sessionID, _ := p["session_id"].(string)
			predicate, _ := p["predicate"].(string)

			subjectID := "asteroid:" + utils.CleanNodeID(subject)
			objectID := "asteroid:" + utils.CleanNodeID(object)

			if subject != "" {
				a.GEngine.AddNode(subjectID, subject, "memory")
				nodeData := map[string]interface{}{
					"id":             subjectID,
					"name":           subject,
					"document-type":  "memory",
					"celestial-type": "asteroid",
					"session-id":     sessionID,
					"summary":        fmt.Sprintf("Fato semântico em memória: %s %s %s", subject, predicate, object),
					"what-it-does":   "Conecta fatos aprendidos no chat para dar contexto em respostas futuras.",
				}
				if pos, exists := savedPositions[subjectID]; exists {
					nodeData["x"] = pos[0]
					nodeData["y"] = pos[1]
					nodeData["z"] = pos[2]
				}
				addNode(nodeData)
			}
			if object != "" {
				a.GEngine.AddNode(objectID, object, "memory")
				nodeData := map[string]interface{}{
					"id":             objectID,
					"name":           object,
					"document-type":  "memory",
					"celestial-type": "asteroid",
					"session-id":     sessionID,
					"summary":        fmt.Sprintf("Entidade relacionada ao fato: %s %s %s", subject, predicate, object),
					"what-it-does":   "Serve como nó de ligação da memória semântica no grafo.",
				}
				if pos, exists := savedPositions[objectID]; exists {
					nodeData["x"] = pos[0]
					nodeData["y"] = pos[1]
					nodeData["z"] = pos[2]
				}
				addNode(nodeData)
			}
			if subject != "" && object != "" {
				a.GEngine.AddEdge(subjectID, objectID, 1.0, "memory")
				addEdge(subjectID, objectID, 1.0, "memory")
			}
		}
	}

	// 🧠 FASE 4: INTELIGÊNCIA NEURAL (Cálculos de Centralidade e Comunidades Louvain)
	if a.GEngine != nil && len(nodesBatch) > 0 {
		fmt.Println("[Sync] 🧠 Inteligência Neural: Calculando autoridade e comunidades Louvain...")
		a.GEngine.ComputePageRank()
		a.GEngine.ComputeCommunities()
		a.GEngine.ComputeBetweenness()
		a.GEngine.ComputeHITS()

		for i, n := range nodesBatch {
			id, _ := n["id"].(string)
			if id != "" {
				nodesBatch[i]["pagerank"] = a.GEngine.GetRank(id)
				nodesBatch[i]["community"] = a.GEngine.GetCommunity(id)
				nodesBatch[i]["betweenness"] = a.GEngine.GetBetweenness(id)
				h, auth := a.GEngine.GetHITS(id)
				nodesBatch[i]["hub"] = h
				nodesBatch[i]["authority"] = auth
			}
		}
	}

	// 🚀 FASE 5: PERSISTÊNCIA ATÔMICA E EMISSÃO PARA O FRONTEND
	if len(nodesBatch) > 0 {
		a.saveTopologyCache(nodesBatch, edgesBatch)
		fmt.Printf("[Sync] 🚀 Emitindo batch final UNIFICADO de %d nós e %d arestas para o Wails...\n", len(nodesBatch), len(edgesBatch))
		a.emitEvent("graph:nodes:batch", nodesBatch)
		a.emitEvent("graph:edges:batch", edgesBatch)
		fmt.Printf("[Sync] ✅ Sincronização de Massa concluída com sucesso.\n")
	} else {
		fmt.Println("[Sync] ⚠️ Nenhum nó encontrado para sincronização.")
	}

	// 🐝 Automação: Dispara saúde e tecelagem automaticamente após o Sync
	go func() {
		ctx := a.ctx // Ancoragem de segurança
		time.Sleep(500 * time.Millisecond) // Pequeno respiro para o motor físico
		stats, _ := a.AnalyzeGraphHealth()
		a.emitEvent("graph:health:update", stats)
		_ = ctx // Mantém a referência viva
	}()
}

// TriggerInitialSync é chamado pelo frontend ao montar o componente para garantir que os dados apareçam.
func (a *App) TriggerInitialSync() string {
	fmt.Println("[Sync] 📥 Requisição de Sincronização Inicial recebida do Frontend.")
	// ⚡ Garante o carregamento instantâneo do cache de topologia local primeiro
	go a.LoadFastGraph()
	// 🔄 Executa sincronização completa em background
	go a.SyncAllNodes()
	return "Sincronização em lote solicitada."
}

func summarizeNodeContent(payload map[string]interface{}) string {
	if s, ok := payload["summary"].(string); ok && strings.TrimSpace(s) != "" {
		return clampSummary(s, 220)
	}

	content, _ := payload["content"].(string)
	if strings.TrimSpace(content) == "" {
		return "Sem resumo disponível ainda. Faça uma sincronização completa para enriquecer o contexto."
	}

	clean := strings.ReplaceAll(content, "\n", " ")
	clean = strings.ReplaceAll(clean, "\r", " ")
	clean = strings.Join(strings.Fields(clean), " ")
	if clean == "" {
		return "Sem resumo disponível ainda."
	}

	if idx := strings.Index(clean, ". "); idx > 40 {
		return clampSummary(clean[:idx+1], 220)
	}

	return clampSummary(clean, 220)
}

func inferNodePurpose(payload map[string]interface{}, summary string) string {
	// Usa o campo armazenado se disponível (gerado individualmente por conteúdo do arquivo)
	if w, ok := payload["what-it-does"].(string); ok && strings.TrimSpace(w) != "" {
		return clampSummary(w, 220)
	}

	docType, _ := payload["document-type"].(string)
	fileType, _ := payload["type"].(string)

	switch strings.ToLower(strings.TrimSpace(docType)) {
	case "memory":
		return "Representa conhecimento consolidado do chat para melhorar respostas futuras."
	case "code-file":
		return "Arquivo de código indexado para responder perguntas técnicas com contexto real do projeto."
	case "project-file":
		return "Documento de repositório satélite usado pelo RAG radial para navegação contextual."
	case "source":
		return "Fonte multimodal (imagem/PDF) convertida em contexto pesquisável no RAG."
	case "markdown":
		return "Nota base de conhecimento usada para recuperação semântica e expansão por grafo."
	}

	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case ".go", ".js", ".ts", ".tsx", ".py", ".html", ".css":
		return "Trecho de código indexado para explicar implementação e dependências."
	case ".md":
		return "Nota documental que alimenta o contexto semântico das respostas."
	case ".pdf", ".png", ".jpg", ".jpeg":
		return "Fonte multimodal analisada para extrair descrição e fatos estruturados."
	}

	if strings.TrimSpace(summary) != "" {
		return "Nó de conhecimento disponível para busca semântica e conexão contextual."
	}

	return "Nó semântico do grafo utilizado pelo RAG para responder com contexto."
}

func clampSummary(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

// RunVectorDiagnostic executa um Stress Test pontual para validar Gemini + Qdrant Cloud.
func (a *App) RunVectorDiagnostic() map[string]interface{} {
	fmt.Println("[BACKEND] 🧪 Iniciando Diagnóstico de Integridade Vetorial...")

	// 🛡️ Segurança: Garante que os serviços básicos estejam inicializados
	if a.crawler == nil || a.ctx == nil {
		return map[string]interface{}{"success": false, "error": "Motor do Crawler ou Contexto não inicializados."}
	}

	// 🛡️ Segurança: Garante que os motores fundamentais (Embedder/Qdrant) estejam ativos
	if a.embedder == nil || a.qdrant == nil {
		fmt.Println("[BACKEND] ⚠️ Motores não inicializados. Tentando reativar para o diagnóstico...")
		if err := a.initServices(); err != nil || a.embedder == nil || a.qdrant == nil {
			return map[string]interface{}{"success": false, "error": "Motores de IA não inicializados ou offline. Verifique sua conectividade e API Key."}
		}
	}

	// 🏗️ Garantia de Infraestrutura: Cria as coleções se não existirem antes do teste
	if err := a.crawler.EnsureCollections(a.ctx); err != nil {
		fmt.Printf("[BACKEND] Erro ao preparar coleções: %v\n", err)
		return map[string]interface{}{"success": false, "error": "Falha ao preparar coleções no Qdrant: " + err.Error()}
	}

	start := time.Now()
	// 1. Teste de Embedding (Gemini)
	testText := "Maestro Vector Test: Sincronização Semântica Atômica."
	embedStart := time.Now()
	vector, err := a.embedder.GenerateEmbedding(a.ctx, testText, false)
	embedDuration := time.Since(embedStart).Milliseconds()

	if err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("Falha no Gemini: %v", err)}
	}

	// 2. Teste de Gravação e Busca (Qdrant)
	qdrantStart := time.Now()
	testID := uint64(999999) // ID Reservado para Testes
	collection := "obsidian_knowledge"

	// Upsert do ponto de teste
	err = a.qdrant.UpsertPoint(collection, testID, vector, map[string]interface{}{
		"name":    "TEST_NODE",
		"content": testText,
		"status":  "test",
	})
	if err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("Falha no Qdrant (Upsert): %v", err)}
	}

	// Search para validar recuperação
	res, err := a.qdrant.Search(collection, vector, 1)
	qdrantDuration := time.Since(qdrantStart).Milliseconds()

	if err != nil {
		return map[string]interface{}{"success": false, "error": fmt.Sprintf("Falha no Qdrant (Search): %v", err)}
	}

	totalDuration := time.Since(start).Milliseconds()

	return map[string]interface{}{
		"success":        true,
		"embed_ms":       embedDuration,
		"qdrant_ms":      qdrantDuration,
		"total_ms":       totalDuration,
		"vector_preview": vector[:5], // Mostra apenas os primeiros 5 números do vetor
		"result_found":   res != nil,
	}
}
