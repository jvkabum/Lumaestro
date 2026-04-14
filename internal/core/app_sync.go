package core

import (
	"Lumaestro/internal/config"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ScanVault percorre o Obsidian e indexa no Qdrant com Embeddings
func (a *App) ScanVault() string {
	fmt.Println("[BACKEND] ScanVault disparado assincronamente...")

	// Tenta auto-recuperar os motores antes de iniciar o job assíncrono.
	if a.crawler == nil || a.ctx == nil {
		_ = a.initServices()
	}

	if a.ctx == nil {
		return "⚠️ Sincronização indisponível: contexto do app ainda não inicializado."
	}

	if a.crawler == nil {
		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "SYSTEM",
			"content": "⚠️ Sync Obsidian 3D bloqueado: sem motor de embeddings ativo. Verifique se o seu provedor (Local, Gemini ou Claude) está configurado e online.",
		})
		return "⚠️ Sync Obsidian 3D bloqueado: sem motor de embeddings ativo. Garanta que o motor selecionado nas configurações está respondendo."
	}

	// 🕵️⚡ RAG em Segundo Plano: Previne travamento total da UI e do Chat
	go func() {
		// 1. Verificação Crítica de Motor e Contexto
		if a.crawler == nil || a.ctx == nil || a.qdrant == nil {
			fmt.Println("[BACKEND] ⏳ Scan ABORTADO: Motores em transição ou offline.")
			return
		}

		err := a.crawler.IndexVault(a.ctx)
		if err != nil {
			fmt.Printf("[BACKEND] Erro na Indexação do Vault: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Erro na Indexação do Obsidian: " + err.Error(),
			})
			return
		}

		// 2. Indexar a documentação do projeto (Lumaestro Core)
		// Isso garante que o conhecimento 'RAG' do sistema também esteja disponível.
		fmt.Println("[BACKEND] Indexando documentos internos do sistema...")
		err = a.crawler.IndexSystemDocs(a.ctx, "./")
		if err != nil {
			fmt.Printf("[BACKEND] Aviso: Erro ao indexar docs do sistema: %v\n", err)
		}

		// 3. Indexar Repositórios Dinâmicos Importados e fazer Code Crawl
		if len(a.config.ExternalProjects) > 0 {
			fmt.Println("[BACKEND] Iniciando expansão radial (Projetos satélites)...")
			err = a.crawler.IndexRepositories(a.ctx, a.config.ExternalProjects)
			if err != nil {
				fmt.Printf("[BACKEND] Erro ao sincronizar external projects: %v\n", err)
			}
		}

		// Silenciado para não sujar o Chat interativo
		// runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
		// 	"source":  "CRAWLER",
		// 	"content": "🏖️ Sincronização semântica completa concluída com sucesso!",
		// })

		// 3. Força a atualização visual de todos os nós (isolados e conectados)
		os.Remove(".lumaestro_topology.json") // Invalida Topology Cache
		a.SyncAllNodes()
	}()

	return "Indexação iniciada em segundo plano. O Maestro agora está integrando seu Obsidian e as memórias do sistema."
}

// FullSync limpa o cache e inicia uma indexação completa atômica.
func (a *App) FullSync() string {
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

	// 2. Limpa Cache Local
	if a.crawler != nil {
		fmt.Println("[RESET] 🧹 Limpando cache do Crawler...")
		a.crawler.PurgeCache()
	}
	os.Remove(".lumaestro_topology.json") // Expurga cache visual 3D

	// 3. Recria Infraestrutura do zero
	dim := 3072
	if a.config != nil && a.config.EmbeddingDimension > 0 {
		dim = a.config.EmbeddingDimension
	}
	fmt.Printf("[RESET] 🏗️ Recriando infraestrutura (%d dim)...\n", dim)
	if a.crawler != nil {
		a.crawler.EnsureCollections(a.ctx)
	}

	// 4. Notifica o Frontend
	runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
		"source":  "SYSTEM",
		"content": "☢️ RESET COMPLETO: Banco de dados Qdrant e cache local foram expurgados.",
	})

	return "✅ O banco de dados foi resetado com sucesso! Inicie um novo SCAN para repovoar."
}

// PurgeCache limpa todo o histórico de indexação local.
func (a *App) PurgeCache() string {
	os.Remove(".lumaestro_topology.json") // Invalida Topology Cache
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
	cache := TopologyCache{
		Nodes: nodes,
		Edges: edges,
	}
	data, err := json.Marshal(cache)
	if err == nil {
		os.WriteFile(".lumaestro_topology.json", data, 0644)
	}
}

func (a *App) loadTopologyCache() *TopologyCache {
	data, err := os.ReadFile(".lumaestro_topology.json")
	if err != nil {
		return nil
	}
	var cache TopologyCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	return &cache
}

// SyncAllNodes percorre o banco de dados e emite cada nota para o visualizador 3D.
func (a *App) SyncAllNodes() {
	if a.qdrant == nil || a.ctx == nil || a.GEngine == nil {
		fmt.Println("[Sync] ⚠️ Sincronização cancelada: Motores vitais indisponíveis.")
		return
	}

	// 1. FORÇA ATUALIZAÇÃO (Ignora Cache para garantir cores novas)
	os.Remove(".lumaestro_topology.json") 
	
	fmt.Println("[Sync] Sincronizando todos os nós do Qdrant com o Frontend (BATCH)...")
	// Busca um lote grande o suficiente para cobrir o vault do usuário (1500+)
	points, err := a.qdrant.Search("obsidian_knowledge", nil, 1500)
	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar nós para sincronização: %v\n", err)
		return
	}
	memoryPoints, err := a.qdrant.Search("knowledge_graph", nil, 1500)
	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar memórias para sincronização: %v\n", err)
	}

	var nodesBatch []map[string]interface{}
	var edgesBatch []map[string]interface{}
	batchIndex := map[string]struct{}{}
	edgeIndex := map[string]struct{}{}

	// 2. ADICIONA ARESTAS AO MOTOR (Passo 1: Construir a topologia em RAM)
	fmt.Println("[Sync] Construindo topologia neural em memória...")
	for _, p := range points {
		name, _ := p["name"].(string)
		if name == "" { continue }
		nodeID := strings.ToLower(name)
		a.GEngine.AddNode(nodeID, name, p["document-type"].(string))
		
		if linksRaw, ok := p["links"].([]interface{}); ok {
			for _, target := range linksRaw {
				if t, ok := target.(string); ok && t != "" {
					a.GEngine.AddEdge(nodeID, strings.ToLower(t), 1, "link")
				}
			}
		}
	}
	for _, p := range memoryPoints {
		subject, _ := p["subject"].(string)
		object, _ := p["object"].(string)
		if subject != "" && object != "" {
			a.GEngine.AddNode(subject, subject, "memory")
			a.GEngine.AddNode(object, object, "memory")
			a.GEngine.AddEdge(subject, object, 1, "memory")
		}
	}

	// 3. COMPUTAÇÃO ATÔMICA (O segredo das Nebulosas)
	fmt.Println("[Sync] 🧠 Inteligência Neural: Calculando autoridade e comunidades Louvain...")
	a.GEngine.ComputePageRank()
	a.GEngine.ComputeCommunities()
	a.GEngine.ComputeBetweenness()
	a.GEngine.ComputeHITS()

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

	addEdge := func(source, target string, weight int) {
		if source == "" || target == "" { return }
		pairID := fmt.Sprintf("%s->%s", source, target)
		if _, exists := edgeIndex[pairID]; exists { return }
		edgeIndex[pairID] = struct{}{}
		
		edge := map[string]interface{}{
			"source": source,
			"target": target,
			"weight": weight,
		}
		edgesBatch = append(edgesBatch, edge)
		runtime.EventsEmit(a.ctx, "graph:edge", edge)
	}

	for _, p := range points {
		name, _ := p["name"].(string)
		if name == "" {
			continue
		}

		nodeID := strings.ToLower(name)
		summary := summarizeNodeContent(p)
		whatItDoes := inferNodePurpose(p, summary)

		nodeData := map[string]interface{}{
			"id":            nodeID,
			"name":          name,
			"document-type": "markdown",
			"summary":       summary,
			"what-it-does":  whatItDoes,
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
					addEdge(nodeID, strings.ToLower(t), 1)
				}
			}
		}

		// 🧠 Extração de Triplas (Relações Explícitas extraídas por IA)
		if triplesRaw, ok := p["triples"].([]interface{}); ok {
			for _, t := range triplesRaw {
				if tm, ok := t.(map[string]interface{}); ok {
					if obj, ok := tm["object"].(string); ok && obj != "" {
						addEdge(nodeID, strings.ToLower(obj), 2)
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

		if subject != "" {
			addNode(map[string]interface{}{
				"id":            subject,
				"name":          subject,
				"document-type": "memory",
				"session-id":    sessionID,
				"summary":       fmt.Sprintf("Fato semântico em memória: %s %s %s", subject, predicate, object),
				"what-it-does":  "Conecta fatos aprendidos no chat para dar contexto em respostas futuras.",
			})
		}
		if object != "" {
			addNode(map[string]interface{}{
				"id":            object,
				"name":          object,
				"document-type": "memory",
				"session-id":    sessionID,
				"summary":       fmt.Sprintf("Entidade relacionada ao fato: %s %s %s", subject, predicate, object),
				"what-it-does":  "Serve como nó de ligação da memória semântica no grafo.",
			})
		}
		if subject != "" && object != "" {
			addEdge(subject, object, 1)
		}
	}

	// Grava o Cache novinho em folha (Nós + Arestas)
	a.saveTopologyCache(nodesBatch, edgesBatch)

	// Emite o pacote completo de nós de uma só vez
	runtime.EventsEmit(a.ctx, "graph:nodes:batch", nodesBatch)
	fmt.Printf("[Sync] ✅ %d nós e %d arestas sincronizados e cacheados.\n", len(nodesBatch), len(edgesBatch))

	// 🐝 Automação: Dispara saúde e tecelagem automaticamente após o Sync
	go func() {
		time.Sleep(500 * time.Millisecond) // Pequeno respiro para o motor físico
		stats, _ := a.AnalyzeGraphHealth()
		runtime.EventsEmit(a.ctx, "graph:health:update", stats)
	}()
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

	// 🏗️ Garantia de Infraestrutura: Cria as coleções se não existirem antes do teste
	if err := a.crawler.EnsureCollections(a.ctx); err != nil {
		fmt.Printf("[BACKEND] Erro ao preparar coleções: %v\n", err)
		return map[string]interface{}{"success": false, "error": "Falha ao preparar coleções no Qdrant: " + err.Error()}
	}

	// 🛡️ Segurança: Garante que os serviços estejam inicializados
	if a.embedder == nil || a.qdrant == nil {
		fmt.Println("[BACKEND] ⚠️ Motores não inicializados. Tentando reativar...")
		if err := a.initServices(); err != nil || a.embedder == nil {
			return map[string]interface{}{"success": false, "error": "Motores de IA n├úo inicializados. Verifique sua Gemini API Key."}
		}
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
