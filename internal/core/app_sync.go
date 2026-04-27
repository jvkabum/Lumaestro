package core

import (
	"Lumaestro/internal/config"
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
		// Isso garante que o conhecimento 'RAG' do sistema também esteja disponível.
		fmt.Println("[BACKEND] Indexando documentos internos do sistema...")
		err = crawler.IndexSystemDocs(ctx, "./")
		if err != nil {
			fmt.Printf("[BACKEND] Aviso: Erro ao indexar docs do sistema: %v\n", err)
		}

		// 3. Indexar Repositórios Dinâmicos e o Workspace Ativo (Devorador de Código)
		projectsToScan := append([]config.ProjectScan{}, a.config.ExternalProjects...)

		// 🚀 INTEGRAÇÃO WORKSPACE: Se houver um workspace ativo, ele entra como prioridade no Devorador
		if a.executor.Workspace != "" {
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

	os.Remove(".lumaestro/topology.json") // Expurga cache visual 3D

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
	os.Remove(".lumaestro/topology.json") // Invalida Topology Cache
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
	Nodes []map[string]interface{} `json:"nodes"`
	Edges []map[string]interface{} `json:"edges"`
}

// Sincronização e I/O Desacoplado do Motor Físico
func (a *App) saveTopologyCache(nodes []map[string]interface{}, edges []map[string]interface{}) {
	// 🛡️ Proteção contra gravação de cache vazio que apagaria dados anteriores
	if len(nodes) == 0 {
		fmt.Println("[Sync] ⚠️ Ignorando gravação de cache vazio (0 nós). Cache anterior preservado.")
		return
	}
	cache := TopologyCache{
		Nodes: nodes,
		Edges: edges,
	}
	data, err := json.Marshal(cache)
	if err == nil {
		os.WriteFile(".lumaestro/topology.json", data, 0644)
	}
}

func (a *App) loadTopologyCache() *TopologyCache {
	data, err := os.ReadFile(".lumaestro/topology.json")
	if err != nil {
		return nil
	}
	var cache TopologyCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	return &cache
}

// LoadFastGraph realiza o "Início a Frio": emite o grafo do cache/DuckDB instantaneamente
func (a *App) LoadFastGraph() {
	// Aguarda um pouco para o Wails Handshake estabilizar
	time.Sleep(1500 * time.Millisecond)
	
	fmt.Println("[Sync] ⚡ Acionando Início a Frio (Fast-Track)...")

	// 1. Tenta carregar do Cache de Topologia (Nós + Arestas)
	cache := a.loadTopologyCache()
	if cache != nil && len(cache.Nodes) > 0 {
		fmt.Printf("[Sync] 🚀 Emitindo %d nós do cache para carregamento instantâneo.\n", len(cache.Nodes))
		a.emitEvent("graph:nodes:batch", cache.Nodes)

		// Delay maior para garantir que o Deck.gl montou a camada de nós antes das arestas
		time.Sleep(500 * time.Millisecond)
		fmt.Printf("[Sync] 🚀 Emitindo %d arestas do cache.\n", len(cache.Edges))
		a.emitEvent("graph:edges:batch", cache.Edges)
		return
	}

	// 2. Fallback: Se não houver cache, tenta ler apenas os nós do DuckDB
	if a.LStore != nil {
		nodes, _, err := a.LStore.GetFullGraph(a.config.ObsidianVaultPath)
		if err == nil && len(nodes) > 0 {
			fmt.Printf("[Sync] 💾 Fallback: Emitindo %d nós do DuckDB.\n", len(nodes))
			a.emitEvent("graph:nodes:batch", nodes)
		}
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
	if cache == nil {
		// Se não existe cache, cria um novo com o que recebemos
		a.saveTopologyCache(nodes, []map[string]interface{}{})
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
				// Adiciona novo nó descoberto ao cache!
				cache.Nodes = append(cache.Nodes, n)
			}
		}
		a.saveTopologyCache(cache.Nodes, cache.Edges)
	}

	return fmt.Sprintf("Layout sincronizado com sucesso (%d nós atualizados).", len(nodes))
}

// SyncAllNodes percorre o banco de dados e emite cada nota para o visualizador 3D.
func (a *App) SyncAllNodes() {
	if a.qdrant == nil || a.ctx == nil || a.GEngine == nil {
		fmt.Println("[Sync] ⚠️ Sincronização cancelada: Motores vitais indisponíveis.")
		return
	}

	// 1. FORÇA ATUALIZAÇÃO (Comentado para preservar layout salvo em cache/db)
	// os.Remove(".lumaestro_topology.json") 
	
	// ⚡ Carrega posições salvas do DuckDB para merge
	savedPositions := make(map[string][]float64)
	if a.LStore != nil {
		nodes, _, _ := a.LStore.GetFullGraph(a.config.ObsidianVaultPath)
		for _, n := range nodes {
			id, _ := n["id"].(string)
			x, _ := n["x"].(float64)
			y, _ := n["y"].(float64)
			z, _ := n["z"].(float64)
			savedPositions[id] = []float64{x, y, z}
		}
	}

	fmt.Println("[Sync] Sincronizando todos os nós do Qdrant com o Frontend (BATCH)...")
	// Busca um lote grande o suficiente para cobrir o vault do usuário (1500+)
	points, err := a.qdrant.Search("obsidian_knowledge", nil, 1500)
	nodesBatch := make([]map[string]interface{}, 0)
	edgesBatch := make([]map[string]interface{}, 0)
	var memoryPoints []map[string]interface{}

	batchIndex := map[string]struct{}{}
	edgeIndex := map[string]struct{}{}
	nameToID := make(map[string]string) // 👈 Movido para o topo para evitar erro de 'goto jumps'

	addNode := func(node map[string]interface{}) {
		id, _ := node["id"].(string)
		if id == "" {
			return
		}
		if _, exists := batchIndex[id]; exists {
			return
		}
		batchIndex[id] = struct{}{}
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

	// 🛠️ FALLBACK: Se Qdrant estiver vazio, tenta carregar a estrutura básica do DuckDB (Fase 1)
	if len(points) == 0 && a.LStore != nil {
		fmt.Println("[Sync] ⚠️ Qdrant vazio. Utilizando estrutura local do DuckDB (Modo Estrutural)...")
		dbNodes, dbEdges, _ := a.LStore.GetFullGraph(a.config.ObsidianVaultPath)
		
		if len(dbNodes) > 0 {
			a.emitEvent("agent:log", map[string]string{
				"source":  "SYNC",
				"content": fmt.Sprintf("🌐 Exibindo estrutura de arquivos (%d objetos). Sincronização IA em progresso...", len(dbNodes)),
			})
			
			// Processamento via GEngine para layout
			for _, n := range dbNodes {
				id, _ := n["id"].(string)
				name, _ := n["name"].(string)
				docType, _ := n["type"].(string)
				parent, _ := n["parent_gravity_id"].(string)
				a.GEngine.AddNode(id, name, docType)
				
				nodeData := map[string]interface{}{
					"id":                id,
					"name":              name,
					"document-type":     docType,
					"parent_gravity_id": parent,
					"summary":           fmt.Sprintf("Objeto estrutural: %s", name),
					"what-it-does":      "Carregado via DuckDB (Modo Estrutural).",
				}
				if pos, exists := savedPositions[id]; exists {
					nodeData["x"] = pos[0]
					nodeData["y"] = pos[1]
					nodeData["z"] = pos[2]
				}
				addNode(nodeData)
			}
			for _, e := range dbEdges {
				src, _ := e["source"].(string)
				tgt, _ := e["target"].(string)
				weight, _ := e["weight"].(float64)
				relType, _ := e["relation_type"].(string)

				if src == "" || tgt == "" || src == tgt { continue }
				a.GEngine.AddEdge(src, tgt, weight, relType)
				addEdge(src, tgt, weight, relType)
			}

			// 🧠 Cálculos de Inteligência para layout visual coerente
			a.GEngine.ComputePageRank()
			a.GEngine.ComputeCommunities()
			
			goto finalize_sync
		}
	}

	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar nós para sincronização: %v\n", err)
		
		// 🛠️ AUTO-REPARO: Se for erro 404 (coleção não existe), tenta criar
		if strings.Contains(err.Error(), "Status 404") || strings.Contains(err.Error(), "Not found") {
			fmt.Println("[Sync] 🏗️ Gatilho de Auto-Reparo: Coleção não encontrada. Criando agora...")
			if a.crawler != nil && a.ctx != nil {
				_ = a.crawler.EnsureCollections(a.ctx)
			} else {
				// Fallback: Cria a coleção diretamente via Qdrant (sem crawler)
				dim := 1024 // Default para embeddings nativos
				if a.config != nil && a.config.EmbeddingDimension > 0 {
					dim = a.config.EmbeddingDimension
				}
				_ = a.qdrant.CreateCollection("obsidian_knowledge", dim)
				_ = a.qdrant.CreateCollection("knowledge_graph", dim)
				fmt.Printf("[Sync] 🏗️ Coleções criadas diretamente (%d dim). Execute um SCAN para popular.\n", dim)
			}
			
			// Notifica o usuário via UI
			a.emitEvent("agent:log", map[string]string{
				"source":  "SYSTEM",
				"content": "⚠️ Base de conhecimento vazia. Clique em SINCRONIZAR no grafo para indexar seus dados.",
			})
		}
		return
    }
	memoryPoints, err = a.qdrant.Search("knowledge_graph", nil, 1500)
	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar memórias para sincronização: %v\n", err)
	}

	// 2. ADICIONA ARESTAS AO MOTOR (Passo 1: Construir a topologia em RAM)
	fmt.Println("[Sync] Construindo topologia neural em memória...")
	for _, p := range points {
		name, _ := p["name"].(string)
		if name == "" { continue }
		nodeID := strings.ToLower(name)
		
		docType, _ := p["document-type"].(string)
		if docType == "" { docType = "markdown" }
		
		a.GEngine.AddNode(nodeID, name, docType)
		
		if linksRaw, ok := p["links"].([]interface{}); ok {
			for _, target := range linksRaw {
				if t, ok := target.(string); ok && t != "" {
					targetID := strings.ToLower(t)
					if targetID != nodeID {
						a.GEngine.AddEdge(nodeID, targetID, 1, "link")
					}
				}
			}
		}
	}
	for _, p := range memoryPoints {
		subject, _ := p["subject"].(string)
		object, _ := p["object"].(string)
		if subject != "" && object != "" {
			subjectID := strings.ToLower(subject)
			objectID := strings.ToLower(object)
			
			a.GEngine.AddNode(subjectID, subject, "memory")
			a.GEngine.AddNode(objectID, object, "memory")
			
			if subjectID != objectID {
				a.GEngine.AddEdge(subjectID, objectID, 1, "memory")
			}
		}
	}

	// 3. COMPUTAÇÃO ATÔMICA (O segredo das Nebulosas)
	fmt.Println("[Sync] 🧠 Inteligência Neural: Calculando autoridade e comunidades Louvain...")
	a.GEngine.ComputePageRank()
	a.GEngine.ComputeCommunities()
	a.GEngine.ComputeBetweenness()
	a.GEngine.ComputeHITS()

	// (Limpando declarações antigas que foram movidas para o topo)

	// 🗺️ Mapeamento de nomes para IDs para resolver links Obsidian [[ ]]
	for _, p := range points {
		name, _ := p["name"].(string)
		id, _ := p["id"].(string)
		if name != "" && id != "" {
			nameToID[strings.ToLower(name)] = id
		}
	}

	for _, p := range points {
		name, _ := p["name"].(string)
		nodeID, _ := p["id"].(string) // 👈 Usa o ID estrutural persistido
		
		if nodeID == "" {
			if name == "" { continue }
			nodeID = strings.ToLower(name)
		}
		if name == "" { name = nodeID }

		summary := summarizeNodeContent(p)
		whatItDoes := inferNodePurpose(p, summary)

		nodeData := map[string]interface{}{
			"id":            nodeID,
			"name":          name,
			"document-type": "markdown",
			"summary":       summary,
			"what-it-does":  whatItDoes,
		}

		// 📍 Injeta coordenadas salvas (se existirem)
		if pos, exists := savedPositions[nodeID]; exists {
			nodeData["x"] = pos[0]
			nodeData["y"] = pos[1]
			nodeData["z"] = pos[2]
		}

		if docType, ok := p["document-type"].(string); ok && strings.TrimSpace(docType) != "" {
			nodeData["document-type"] = docType
		}
		if fileType, ok := p["type"].(string); ok && strings.TrimSpace(fileType) != "" {
			nodeData["file-type"] = fileType
		}

		// ⚖️ Injeta métricas do Cérebro Relacional (se disponível)
		if a.GEngine != nil {
			nodeData["pagerank"] = a.GEngine.GetRank(nodeID)
			nodeData["community"] = a.GEngine.GetCommunity(nodeID)
			nodeData["betweenness"] = a.GEngine.GetBetweenness(nodeID)

			h, auth := a.GEngine.GetHITS(nodeID)
			nodeData["hub"] = h
			nodeData["authority"] = auth
		}

		addNode(nodeData)

		// 🖇️ Extração de Links Diretos (Obsidian [[Bracket Links]])
		if linksRaw, ok := p["links"].([]interface{}); ok {
			for _, target := range linksRaw {
				if t, ok := target.(string); ok && t != "" {
					targetNameLower := strings.ToLower(t)
					targetID := targetNameLower
					if realID, ok := nameToID[targetNameLower]; ok {
						targetID = realID
					}
					addEdge(nodeID, targetID, 1.0, "link")
				}
			}
		}

		// 🧠 Extração de Triplas (Relações Explícitas extraídas por IA)
		if triplesRaw, ok := p["triples"].([]interface{}); ok {
			for _, t := range triplesRaw {
				if tm, ok := t.(map[string]interface{}); ok {
					if obj, ok := tm["object"].(string); ok && obj != "" {
						targetNameLower := strings.ToLower(obj)
						targetID := targetNameLower
						if realID, ok := nameToID[targetNameLower]; ok {
							targetID = realID
						}
						addEdge(nodeID, targetID, 2.0, "semantic")
					}
				}
			}
		}
	}

	for _, p := range memoryPoints {
		subject, _ := p["subject"].(string)
		object, _ := p["object"].(string)
		sessionID, _ := p["session_id"].(string)
		predicate, _ := p["predicate"].(string)

		subjectID := strings.ToLower(subject)
		objectID := strings.ToLower(object)

		if subject != "" {
			nodeData := map[string]interface{}{
				"id":            subjectID,
				"name":          subject,
				"document-type": "memory",
				"session-id":    sessionID,
				"summary":       fmt.Sprintf("Fato semântico em memória: %s %s %s", subject, predicate, object),
				"what-it-does":  "Conecta fatos aprendidos no chat para dar contexto em respostas futuras.",
			}
			if pos, exists := savedPositions[subjectID]; exists {
				nodeData["x"] = pos[0]
				nodeData["y"] = pos[1]
				nodeData["z"] = pos[2]
			}
			addNode(nodeData)
		}
		if object != "" {
			nodeData := map[string]interface{}{
				"id":            objectID,
				"name":          object,
				"document-type": "memory",
				"session-id":    sessionID,
				"summary":       fmt.Sprintf("Entidade relacionada ao fato: %s %s %s", subject, predicate, object),
				"what-it-does":  "Serve como nó de ligação da memória semântica no grafo.",
			}
			if pos, exists := savedPositions[objectID]; exists {
				nodeData["x"] = pos[0]
				nodeData["y"] = pos[1]
				nodeData["z"] = pos[2]
			}
			addNode(nodeData)
		}
		if subject != "" && object != "" {
			addEdge(subjectID, objectID, 1.0, "memory")
		}
	}

finalize_sync:
	// Grava o Cache novinho em folha (Nós + Arestas)
	a.saveTopologyCache(nodesBatch, edgesBatch)

	// Emite o pacote completo de nós e arestas de uma só vez
	fmt.Printf("[Sync] Emitindo batch final de %d nós e %d arestas para o Wails...\n", len(nodesBatch), len(edgesBatch))
	a.emitEvent("graph:nodes:batch", nodesBatch)
	a.emitEvent("graph:edges:batch", edgesBatch) // 🚀 Lançamento em Lote Atômico
	fmt.Printf("[Sync] ✅ Sincronização de Massa concluída.\n")

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
