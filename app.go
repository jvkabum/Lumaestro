package main

import (
	"Lumaestro/internal/agents"
	"Lumaestro/internal/config"
	"Lumaestro/internal/db"
	"Lumaestro/internal/lightning"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/orchestration"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag"
	"Lumaestro/internal/rag/neural"
	"Lumaestro/internal/tools"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	executor     *agents.ACPExecutor
	orchestrator *agents.Orchestrator
	legacyExec   *agents.Executor // Apenas para ExecuteCLI fallback se necessário, ou podemos migrar.
	ontology     *provider.OntologyService
	crawler      *obsidian.Crawler
	qdrant       *provider.QdrantClient
	embedder     *provider.EmbeddingService
	chat         *rag.ChatService
	weaver       *rag.KnowledgeWeaver
	navigator    *rag.GraphNavigator
	ranker       *neural.Ranker
	installer    *tools.Installer
	config       *config.Config
	muInit       sync.Mutex // 🛡️ Trava de Segurança contra inicialização dupla (HMR/Wails)
	
	// ✨ Motores de Elite (Lightning)
	LStore       *lightning.DuckDBStore
	LReflector   *lightning.Reflector
	LOptimizer   *lightning.Optimizer
	LRouter      *lightning.LLMRouter

	// 🧠 Cérebro Relacional (V20, V22, V23)
	GEngine      *rag.GraphEngine
	Validator    *rag.AgentValidator
	Recon        *rag.AgentRecon
}

// NewApp creates a new App application struct
func NewApp() *App {
	a := &App{}
	a.executor = agents.NewACPExecutor()
	a.orchestrator = agents.NewOrchestrator(a.executor)
	a.legacyExec = agents.NewExecutor()
	a.installer = tools.NewInstaller()
	return a
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	// 🛡️ Detector de arquivos Go órfãos que quebram o Wails silenciosamente
	checkRogueMainFiles()

	// 🗄️ Iniciar o Banco de Dados Paperclip (Orquestração Corporativa)
	if err := db.InitDB(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no db.InitDB: %v\n", err)
	}

	a.ctx = ctx
	
	// Sincroniza o PATH imediatamente (Garante que claude/gemini sejam encontrados)
	a.installer.SyncPath()

	// Iniciar a Escuta de Logs e Terminal (não depende dos motores)
	go a.listenForLogs()
	go a.listenForInstallerLogs()
	go a.listenForTerminalOutput()

	// 🚀 Boot Assíncrono: Garante que o WebView esteja pronto antes de emitir eventos
	go a.bootSequence()
	
	// 🧠 Córtex Autônomo (APO): Monitora falhas e otimiza prompts em background
	go a.startAPOWorker()
}

// bootSequence executa a inicialização dos motores em background,
// emitindo eventos de diagnóstico para o frontend em tempo real.
// Roda em goroutine para não bloquear o startup do Wails (evita deadlock no EventsEmit).
func (a *App) bootSequence() {
	// Delay de 1s para o frontend renderizar e montar o listener Vue de boot:stage
	time.Sleep(1 * time.Second)

	a.emitBoot("config", "⚙️", "Carregando configurações...")

	// Tenta inicializar os serviços logo na subida
	if err := a.initServices(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no initServices: %v\n", err)
		a.emitBoot("error", "🔴", "Falha na inicialização: " + err.Error())
		return
	}

	// Injeta o contexto oficial em todos os serviços APÓS a inicialização para garantir estabilidade
	a.injectContexts()

	// 🚀 Auto-Start: Inicia os agentes e sincroniza conhecimento
	if a.config != nil && a.config.GetActiveGeminiKey() != "" {
		fmt.Println("[BOOT] Maestro Online. Sincronizando conhecimento e restaurando agentes...")
		
		// 1. Inicia o Agente Padrão (Se configurado para auto-start)
		if len(a.config.AutoStartAgents) > 0 {
			a.emitBoot("agent", "🤖", "Iniciando agente " + a.config.AutoStartAgents[0] + "...")
			a.StartAgentSession(a.config.AutoStartAgents[0])
		}

		// 2. Indexação Silenciosa (RAG) - Agora PARALELA e em Background
		if a.crawler != nil && a.config.ObsidianVaultPath != "" {
			a.emitBoot("scan", "✈️", "Sincronizando conhecimento em background...")
			fmt.Println("[BOOT] ✈️ Iniciando Crawler Paralelo em segundo plano...")
			
			// Execução em background para não travar o fluxo de boot da UI
			go func() {
				a.ScanVault()
				a.emitBoot("complete", "✅", "Sincronização concluída.")
			}()
		}

		// 3. Inicia o Coração (Orquestração Swarm)
		go a.startOrchestration()
	}
}

// ensureCollections garante que o banco de dados Qdrant esteja pronto para uso.
func (a *App) ensureCollections() {
	collections := []string{"obsidian_knowledge", "knowledge_graph"}
	dimension := 3072 // Gemini Embedding v2 Dimension (768 era v1)

	for _, name := range collections {
		exists, err := a.qdrant.CheckCollectionExists(name)
		if err != nil {
			fmt.Printf("[QDRANT] Erro ao verificar coleção %s: %v\n", name, err)
			continue
		}

		if !exists {
			fmt.Printf("[QDRANT] Criando coleção inexistente: %s...\n", name)
			err := a.qdrant.CreateCollection(name, dimension)
			if err != nil {
				fmt.Printf("[QDRANT] Erro fatal ao criar coleção %s: %v\n", name, err)
			} else {
				fmt.Printf("[QDRANT] Coleção %s criada com sucesso!\n", name)
			}
		}
	}
}

// initServices inicializa os motores de IA e RAG se as configurações permitirem
func (a *App) initServices() error {
	a.muInit.Lock()
	defer a.muInit.Unlock()

	if a.crawler != nil {
		return nil // Já inicializado
	}

	cfg, err := config.Load()
	if err != nil || cfg == nil {
		fmt.Printf("⚠️ Arquivo de configuração não encontrado ou vazio. Aguardando setup na UI...\n")
		return nil // Não retorna erro crítico. Permite o App iniciar sem motores.
	}
	a.config = cfg

	// Inicializa Qdrant e Embeddings
	fmt.Printf("[App] 🔋 Conectando ao motor vetorial Qdrant em: %s\n", cfg.QdrantURL)
	a.emitBoot("qdrant", "🔋", "Conectando ao banco vetorial Qdrant...")
	a.qdrant = provider.NewQdrantClient(cfg.QdrantURL, cfg.QdrantAPIKey)

	fmt.Println("[App] 🧬 Inicializando motor de Embeddings (Gemini)...")
	a.emitBoot("embeddings", "🧬", "Inicializando motor de Embeddings (Gemini)...")
	emb, err := provider.NewEmbeddingService(a.ctx, cfg.GetActiveGeminiKey())
	if err != nil {
		fmt.Printf("🔴 FALHA CRÍTICA: Motor de Embeddings não iniciou: %v\n", err)
		a.emitBoot("error", "🔴", "Motor de Embeddings falhou: "+err.Error())
		return err
	}

	a.embedder = emb
	// 🧠 Migração para Modo Híbrido: Ontologia e Web Weaving usam API Nativa GenAI (100x mais rápido e imune a falhas de IPC)
	a.ontology = provider.NewOntologyService(a.ctx, a.embedder)

	// Inicializa os órgãos de RAG e Aprendizado Neural
	fmt.Println("[App] 🧠 Ativando Córtex Neural (Ranker & Decay)...")
	a.emitBoot("neural", "🧠", "Ativando Córtex Neural — Esquecimento Natural (Decay)...")
	a.ranker = neural.NewRanker()
	a.ranker.Decay()

	search := rag.NewSearchService(a.qdrant, a.ranker)
	a.navigator = rag.NewGraphNavigator(a.qdrant, a.ranker)
	a.weaver = rag.NewKnowledgeWeaver(a.ontology, a.qdrant, a.embedder)

	fmt.Println("[App] 🎭 Orquestrando serviços de Chat...")
	a.emitBoot("chat", "🎭", "Orquestrando serviços de Chat e RAG...")
	a.chat = rag.NewChatService(a.legacyExec, a.orchestrator, search, a.navigator, a.embedder, a.installer)

	fmt.Println("[App] 🕸️ Tecendo o Crawler do Obsidian...")
	a.emitBoot("crawler", "🕸️", "Tecendo o Crawler do Obsidian...")
	
	// 🧠 Cérebro Relacional (V20) e Auditor Neural (V22)
	a.GEngine = rag.NewGraphEngine()
	// O LStore pode estar nulo se Lightning não ativado, então Validator/Recon precisam lidar com isso.
	a.Validator = rag.NewAgentValidator(a.LStore, a.GEngine)
	a.Recon = rag.NewAgentRecon(a.LStore, a.GEngine, a.qdrant)

	// Injetar GEngine no Crawler (dependendo da assinatura, aqui injetamos globalmente para uso futuro)
	a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology)

	// Restaurar Sinapses do DuckDB para a RAM (se disponível)
	if a.LStore != nil {
		nodes, edges, err := a.LStore.GetFullGraph()
		if err == nil {
			for _, n := range nodes {
				a.GEngine.AddNode(n["id"].(string), n["name"].(string), n["type"].(string))
			}
			for _, e := range edges {
				a.GEngine.AddEdge(e["source"].(string), e["target"].(string), e["weight"].(float64), e["relation_type"].(string))
			}
			a.GEngine.ComputePageRank()
			fmt.Printf("[App] 🧬 %d nós e %d arestas restaurados para o cérebro em RAM.\n", len(nodes), len(edges))
		}
	}

	// 🔥 Injeção de Autonomia: Maestro agora pode comandar o Crawler
	a.executor.Tools.Indexer = a.crawler

	if cfg.LightningEnabled {
		fmt.Println("[App] ⚡ Ativando Motores Lightning (Analytical & Reflective)...")
		a.emitBoot("lightning", "⚡", "Iniciando cérebro analítico DuckDB e Reflector...")
		
		// O DuckDBStore é inicializado no main.go, mas garantimos o Reflector aqui
		if a.LStore != nil {
			a.LReflector = lightning.NewReflector(a.LStore, cfg.ObsidianVaultPath)
			a.LOptimizer = lightning.NewOptimizer(a.LStore, a.executor.RewardEngine)
			a.LRouter = lightning.NewLLMRouter()
		}
	}

	fmt.Println("[App] ✅ TODOS OS MOTORES LIGADOS! Sistema RAG pronto para decolagem.")
	a.emitBoot("ready", "✅", "Todos os motores ligados! Maestro pronto.")

	// Blindagem: Injeta o contexto se as instâncias acabaram de ser criadas
	a.injectContexts()

	return nil
}

// injectContexts garante que todos os motores de RAG tenham o contexto oficial do Wails para EventsEmit
func (a *App) injectContexts() {
	if a.ctx == nil {
		return
	}
	fmt.Printf("[App] 🛡️ Injetando Contexto de Ciclo de Vida do Wails nos motores...\n")
	if a.crawler != nil {
		a.crawler.SetContext(a.ctx)
	}
	if a.weaver != nil {
		a.weaver.SetContext(a.ctx)
	}
	if a.navigator != nil {
		a.navigator.SetContext(a.ctx)
	}
	if a.chat != nil {
		a.chat.SetContext(a.ctx)
	}
}

// emitBoot envia um evento de diagnóstico de boot para o frontend
func (a *App) emitBoot(stage string, icon string, message string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "boot:stage", map[string]string{
		"stage": stage, "icon": icon, "message": message,
	})
}

// listenForLogs ouve o Executor ACP (Logs da IA no formato JSON-RPC via STDOUT)
func (a *App) listenForLogs() {
	for log := range a.executor.LogChan {
		// Log discreto apenas para monitoramento técnico de fluxo
		// fmt.Printf("[Wails] Evento agent:log enviado\n")
		runtime.EventsEmit(a.ctx, "agent:log", log)
	}
}

// listenForInstallerLogs ouve o Instalador (Logs do Terminal/NPM/PS)
func (a *App) listenForInstallerLogs() {
	for log := range a.installer.LogChan {
		runtime.EventsEmit(a.ctx, "installer:log", log)
	}
}

// listenForTerminalOutput (Descontinuado para Renderização no Modo ACP, mantido para evitar quebra)
func (a *App) listenForTerminalOutput() {
	for td := range a.executor.TerminalOutput {
		if td.Data == nil {
			runtime.EventsEmit(a.ctx, "terminal:closed", td.Agent)
			continue
		}
		encoded := base64.StdEncoding.EncodeToString(td.Data)
		runtime.EventsEmit(a.ctx, "terminal:output", map[string]string{
			"agent": td.Agent,
			"data":  encoded,
		})
	}
}

// AskAgent processa a pergunta em segundo plano para permitir Streaming Real
func (a *App) AskAgent(agentName string, prompt string) string {
	fmt.Printf("[BACKEND] AskAgent chamado para: %s\n", agentName)

	if a.chat == nil {
		fmt.Println("[App] ⚠️ Motor de Chat nulo. Tentando inicialização de emergência...")
		if err := a.initServices(); err != nil || a.chat == nil {
			return "⚠️ O motor do Maestro está desligado. Verifique sua Gemini API Key nas configurações."
		}
	}

	if agentName == "" {
		agentName = "gemini"
	}

	go func() {
		fmt.Printf("[BACKEND] Iniciando chamada de Chat para: %s\n", agentName)
		// Usamos "default" como sessionID para manter o histórico em memória nesta sessão do app.
		response, err := a.chat.Ask(a.ctx, agentName, "default", prompt)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO no Chat: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha na Sinfonia: " + err.Error(),
			})
			return
		}

		fmt.Printf("[BACKEND] Resposta da Orquestração recebida. Injetando na sessão ACP...\n")

		// Injeta a pergunta (prompt completo com RAG e histórico) na sessão ACP ativa
		// O executor cuidará de enviar via StdIn seguindo o protocolo ndJSON
		err = a.executor.SendInput("default", response, nil)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO ao enviar para o agente: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha ao comunicar com o agente: " + err.Error(),
			})
			return
		}
	}()

	return "Orquestrando..."
}

// ScanVault percorre o Obsidian e indexa no Qdrant com Embeddings
func (a *App) ScanVault() string {
	fmt.Println("[BACKEND] ScanVault disparado assincronamente...")

	// 🕊️ RAG em Segundo Plano: Previne travamento total da UI e do Chat
	go func() {
		// 1. Verificação Crítica de Motor e Contexto
		if a.crawler == nil || a.ctx == nil {
			fmt.Println("[BACKEND] ⏳ Scan ADIADO: Aguardando prontidão dos motores...")
			return
		}

		// 2. Mensagem silenciada no chat UI para respeitar o ambiente Black/Background
		// runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
		// 	"source":  "CRAWLER",
		// 	"content": "🚀 Iniciando Sincronização Semântica Completa em background...",
		// })

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
		// 	"content": "🏛️ Sincronização semântica completa concluída com sucesso!",
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
		return "⚠️ Motor de indexação indisponível."
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
	fmt.Println("[RESET] 🏗️ Recriando infraestrutura (3072 dim)...")
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

// Sincronização e I/O Desacoplado do Motor Físico
func (a *App) saveTopologyCache(batch []map[string]interface{}) {
	data, err := json.Marshal(batch)
	if err == nil {
		os.WriteFile(".lumaestro_topology.json", data, 0644)
	}
}

func (a *App) loadTopologyCache() []map[string]interface{} {
	data, err := os.ReadFile(".lumaestro_topology.json")
	if err != nil {
		return nil
	}
	var batch []map[string]interface{}
	json.Unmarshal(data, &batch)
	return batch
}

// SyncAllNodes percorre o banco de dados e emite cada nota para o visualizador 3D.
func (a *App) SyncAllNodes() {
	if a.qdrant == nil || a.ctx == nil {
		return
	}

	// 1. TENTA TOPOLOGY CACHE (IGNORA O QDRANT INSTANTANEAMENTE SE EXISTIR)
	cachedBatch := a.loadTopologyCache()
	if cachedBatch != nil && len(cachedBatch) > 0 {
		fmt.Printf("[Sync] ⚡ Carregando %d nós instantaneamente via Fast Topology Cache (0 delay de infraestrutura)...\n", len(cachedBatch))
		runtime.EventsEmit(a.ctx, "graph:nodes:batch", cachedBatch)
		
		go func() {
			time.Sleep(500 * time.Millisecond) // Pequeno respiro para o motor físico
			stats, _ := a.AnalyzeGraphHealth()
			runtime.EventsEmit(a.ctx, "graph:health:update", stats)
		}()
		return
	}

	fmt.Println("[Sync] Sincronizando todos os nós do Qdrant com o Frontend (BATCH)...")
	// Busca um lote grande o suficiente para cobrir o vault do usuário (1500+)
	points, err := a.qdrant.Search("obsidian_knowledge", nil, 1500)
	if err != nil {
		fmt.Printf("[Sync] Erro ao buscar nós para sincronização: %v\n", err)
		return
	}
	var batch []map[string]interface{}
	for _, p := range points {
		name, _ := p["name"].(string)
		if name == "" {
			continue
		}

		nodeID := strings.ToLower(name)
		
		nodeData := map[string]interface{}{
			"id":            nodeID,
			"name":          name,
			"document-type": "markdown",
		}

		// ✨ Injeta métricas do Cérebro Relacional (se disponível)
		if a.GEngine != nil {
			nodeData["pagerank"] = a.GEngine.GetRank(nodeID)
			nodeData["community"] = a.GEngine.GetCommunity(nodeID)
			nodeData["betweenness"] = a.GEngine.GetBetweenness(nodeID)
			
			h, auth := a.GEngine.GetHITS(nodeID)
			nodeData["hub"] = h
			nodeData["authority"] = auth
		}

		batch = append(batch, nodeData)
	}
	
	// Grava o Cache novinho em folha
	a.saveTopologyCache(batch)

	// Emite o pacote completo de uma só vez para evitar sobrecarga no motor gráfico
	runtime.EventsEmit(a.ctx, "graph:nodes:batch", batch)
	fmt.Printf("[Sync] ✅ %d nós sincronizados em lote.\n", len(batch))

	// 🪐 Automação: Dispara saúde e tecelagem automaticamente após o Sync
	go func() {
		time.Sleep(500 * time.Millisecond) // Pequeno respiro para o motor físico
		stats, _ := a.AnalyzeGraphHealth()
		runtime.EventsEmit(a.ctx, "graph:health:update", stats)
	}()
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
			return map[string]interface{}{"success": false, "error": "Motores de IA não inicializados. Verifique sua Gemini API Key."}
		}
	}

	start := time.Now()
	// 1. Teste de Embedding (Gemini)
	testText := "Maestro Vector Test: Sincronização Semântica Atômica."
	embedStart := time.Now()
	vector, err := a.embedder.GenerateEmbedding(a.ctx, testText)
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

// CheckConnection verifica se o Qdrant está acessível e se o motor de RAG (Crawler) já decolou.
func (a *App) CheckConnection() bool {
	if a.qdrant == nil || a.config == nil || a.crawler == nil {
		fmt.Println("[BACKEND-UI] CheckConnection: Motores ainda aquecendo...")
		return false
	}
	return true
}

// GetToolsStatus verifica se as IAs CLIs estão instaladas no PATH e os status de autenticação
func (a *App) GetToolsStatus() map[string]bool {
	// Reduzimos o ruído no log para esse porque ele é feito a cada refresh
	return map[string]bool{
		"gemini":      a.installer.CheckStatus("gemini"),
		"claude":      a.installer.CheckStatus("claude"),
		"obsidian":    a.installer.CheckStatus("obsidian"),
		"claude_auth": a.installer.CheckClaudeAuth(),
		"gemini_auth": a.installer.CheckGeminiAuth(),
	}
}

// InstallTool dispara a instalação via CLI oficial
func (a *App) InstallTool(name string) string {
	var err error
	switch name {
	case "gemini":
		err = a.installer.InstallGemini()
	case "claude":
		err = a.installer.InstallClaude()
	case "obsidian":
		err = a.installer.InstallObsidian()
	default:
		return "Ferramenta desconhecida."
	}

	if err != nil {
		return "Erro na instalação: " + err.Error()
	}
	return "Instalação de " + name + " concluída com sucesso!"
}

// FixEnvironment tenta corrigir caminhos de ambiente manualmente
func (a *App) FixEnvironment() string {
	err := a.installer.FixClaudePath()
	if err != nil {
		return "Erro ao corrigir ambiente: " + err.Error()
	}
	return "Ambiente corrigido com sucesso! Reinicie o aplicativo."
}

// GetConfig retorna as configurações atuais para o Vue
func (a *App) GetConfig() *config.Config {
	cfg, _ := config.Load()
	fmt.Printf("[BACKEND-UI] GetConfig disparado pelo frontend. Enviando URL Qdrant: %s\n", cfg.QdrantURL)
	return cfg
}

// SaveConfig persiste as novas configurações no config.json
func (a *App) SaveConfig(cfg config.Config) string {
	err := config.Save(cfg)
	if err != nil {
		return "Erro ao salvar: " + err.Error()
	}

	a.startup(a.ctx)
	return "Configurações salvas e serviços reiniciados!"
}

// SetupTool abre um terminal externo - Legado.
func (a *App) SetupTool(name string) string {
	err := a.installer.SetupTool(name)
	if err != nil {
		return "Erro ao abrir terminal: " + err.Error()
	}
	return "Janela de configuração aberta!"
}

// StartLoginSession inicia uma sessão de terminal interativa interna para login.
func (a *App) StartLoginSession(agent string) string {
	binary, args := a.installer.GetSetupCommand(agent)
	sessionID := "login-session-" + agent

	err := a.legacyExec.StartCustomSession(a.ctx, agent, binary, args, sessionID)
	if err != nil {
		return "Erro ao iniciar sessão de login: " + err.Error()
	}

	runtime.EventsEmit(a.ctx, "terminal:started", map[string]interface{}{
		"agent":     agent,
		"mode":      "Configuração/Login",
		"isRealPTY": true,
	})

	return "Sessão de login iniciada no terminal interno."
}

// ============================================================
// TERMINAL ACP — JSON RPC 2.0 (O CÉREBRO)
// ============================================================

// StartAgentSession inicia a CLI do Gemini em modo seguro ACP (JSON RPC 2.0).
func (a *App) StartAgentSession(agent string) error {
	sessionID := "acp-session-" + agent

	// 🛡️ Trava de Segurança: Não inicia se já houver uma sessão ativa ou iniciando para este agente.
	a.executor.Mu.Lock()
	_, exists := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if exists {
		fmt.Printf("[App] Agente %s já está no Ar. Orquestra pronta.\n", agent)
		return nil
	}

	fmt.Printf("[App] Iniciando agente: %s\n", agent)
	// No primeiro boot ou reinício, passamos loadSessionID como "LATEST" para carregar a última Sinfonia.
	return a.executor.StartSession(a.ctx, agent, sessionID, "LATEST", uuid.Nil, nil)
}

// StartBackgroundAgentSession cria uma instância paralela silenciosa exclusiva para o processamento de RAG
func (a *App) StartBackgroundAgentSession(agent string) error {
	sessionID := "acp-session-background-" + agent

	a.executor.Mu.Lock()
	_, exists := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if exists {
		fmt.Printf("[App] Agente de Background (%s) já está online.\n", agent)
		return nil
	}

	fmt.Printf("[App] Iniciando Agente de BACKGROUND (Black): %s\n", agent)
	// Background NUNCA deve carregar histórico (LATEST) para não misturar os contextos. Inicia sempre limpo.
	return a.executor.StartSession(a.ctx, agent, sessionID, "", uuid.Nil, nil)
}

// ListAgentSessions retorna a lista de conversas salvas para o agente
func (a *App) ListAgentSessions(agent string) ([]agents.SessionInfo, error) {
	sessionID := "acp-session-" + agent
	a.executor.Mu.Lock()
	session, ok := a.executor.ActiveSessions[sessionID]
	a.executor.Mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("inicie o agente antes de listar o histórico")
	}

	return a.executor.ListSessions(session)
}

// LoadAgentSession encerra a atual e carrega uma antiga (Checkpoint)
func (a *App) LoadAgentSession(agent string, acpSessionID string) error {
	fmt.Printf("[App] Trocando para sessão: %s\n", acpSessionID)
	sessionID := "acp-session-" + agent
	return a.executor.StartSession(a.ctx, agent, sessionID, acpSessionID, uuid.Nil, nil)
}

// NewAgentSession força a criação de um novo chat (limpa o contexto)
func (a *App) NewAgentSession(agent string) error {
	fmt.Println("[App] Iniciando NOVO chat (limpando contexto)...")
	sessionID := "acp-session-" + agent
	return a.executor.StartSession(a.ctx, agent, sessionID, "", uuid.Nil, nil)
}

func (a *App) SendAgentInput(agent string, input string, images []map[string]string) error {
	// 🚨 Log Premium e Limpo
	previewInput := input
	if len(previewInput) > 60 {
		previewInput = previewInput[:57] + "..."
	}
	fmt.Printf("[App] 📨 Sincronizando Mensagem >> Motor: %s | Preview: '%s'\n", agent, previewInput)

	// 🚨 Idioma Dinâmico
	lang := a.GetConfig().AgentLanguage
	if lang == "" {
		lang = "Português do Brasil"
	}

	// 🧠 Injetor de Memória Semântica com Ligações Nervosas (RAG + Grafo)
	contextInfo := ""
	if a.embedder != nil && a.navigator != nil && a.config.ObsidianVaultPath != "" {
		fmt.Println("[RAG] Explorando ligações nervosas no Grafo de Conhecimento...")
		vector, err := a.embedder.GenerateEmbedding(a.ctx, input)
		if err == nil {
			// 1. Busca as notas âncoras (Top 3)
			nodes, err := a.qdrant.Search("obsidian_knowledge", vector, 3)
			if err == nil && len(nodes) > 0 {
				// 2. Navegação de Sinapses: Expandir o contexto seguindo os links neurais
				fullContext := a.navigator.ExpandContext(a.ctx, nodes)

				contextInfo = "\n\n[CONHECIMENTO ORQUESTRADO (OBSIDIAN + SINAPSES)]\n"
				for _, ctxPart := range fullContext {
					contextInfo += ctxPart + "\n\n"
				}
				fmt.Printf("[RAG] Contexto expandido via Grafo com %d fontes.\n", len(fullContext))
			}
		} else {
			fmt.Printf("[RAG] Erro ao gerar embedding para contexto: %v\n", err)
		}
	}

	// Diretiva Técnica Dinâmica: Força o idioma em todos os canais (Resposta e Raciocínio).
	directive := fmt.Sprintf("\n\n[SYSTEM DIRECTIVE: You MUST think, reason, and respond exclusively in %s. This applies to your 'Thought Channel' and your final response. DO NOT use English for internal reasoning. ORGANIZATION RULES: 1. Use clear Markdown headers (##). 2. Use horizontal rules (---) to separate major sections. 3. Keep paragraphs short (max 3 lines). 4. Use bold text for key terms.]", lang)

	// A Sinfonia Final: Contexto + Input + Diretiva
	enhancedInput := contextInfo + "\n\nMENSAGEM DO USUÁRIO:\n" + input + directive

	sessionID := "acp-session-" + agent
	err := a.executor.SendInput(sessionID, enhancedInput, images)
	if err != nil {
		fmt.Printf("[App] ERRO no SendAgentInput: %v\n", err)
		return fmt.Errorf("erro ao enviar input para ACP: %v", err)
	}

	fmt.Printf("[App] ✅ Sinfonia roteada para %s com sucesso via JSON-RPC!\n", agent)
	return nil
}

// ConsolidateChatKnowledge analisa o diálogo recente e cria ligações nervosas (sinapses).
func (a *App) ConsolidateChatKnowledge(sessionID string, chatText string) string {
	if a.weaver == nil {
		return "⚠️ Motor de memórias não inicializado."
	}

	fmt.Printf("[App] Consolidando ligações nervosas para sessão %s...\n", sessionID)
	err := a.weaver.WeaveChatKnowledge(a.ctx, sessionID, chatText)
	if err != nil {
		return "Erro ao tecer sinapses: " + err.Error()
	}

	return "✅ Sinapses consolidadas com sucesso no Grafo de Conhecimento."
}

// ResolveConflict executa a decisão do usuário sobre uma contradição semântica detectada.
func (a *App) ResolveConflict(decision string, subject string, predicate string, oldID uint64, newValue string, sessionID string) string {
	if decision == "new" {
		// 1. Marcar o antigo como LEGADO
		a.qdrant.SetPayload("knowledge_graph", oldID, map[string]interface{}{
			"status":      "legacy",
			"archived_at": time.Now().Format(time.RFC3339),
		})

		// 2. Salvar o NOVO como ativo
		factText := fmt.Sprintf("%s %s %s", subject, predicate, newValue)
		vector, _ := a.crawler.Embedder.GenerateEmbedding(a.ctx, factText)

		h := fnv.New64a()
		h.Write([]byte(factText + sessionID))
		newID := h.Sum64()

		payload := map[string]interface{}{
			"id":         newID,
			"session_id": sessionID,
			"subject":    subject,
			"predicate":  predicate,
			"object":     newValue,
			"source":     "chat_memory",
			"status":     "active",
			"timestamp":  time.Now().Format(time.RFC3339),
			"content":    factText,
		}

		a.qdrant.UpsertPoint("knowledge_graph", newID, vector, payload)

		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "RESOLVER",
			"content": fmt.Sprintf("✅ Conflito resolvido: '%s' agora é a verdade sobre '%s'.", newValue, subject),
		})
	} else {
		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "RESOLVER",
			"content": fmt.Sprintf("🏛️ Conflito resolvido: Mantida a informação histórica para '%s'.", subject),
		})
	}

	return "Conflito resolvido."
}

func (a *App) GetProjectDoc(name string) (string, error) {
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	fmt.Printf("[App] Lendo documentação: %s\n", name)
	path := filepath.Join(".", "docs", name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("erro ao ler %s: %v", name, err)
	}

	return string(content), nil
}

func (a *App) GetNodeDetails(nodeID string) (map[string]interface{}, error) {
	fmt.Printf("[Audit] Buscando origem de: %s\n", nodeID)

	// 1. Tentar buscar em Notas do Obsidian ou Sistema (Chave: name)
	res, err := a.qdrant.SearchByField("obsidian_knowledge", "name", nodeID)

	// Fallback: Se não achar campo exato (slug mismatch), tentar busca similar textual
	if err != nil || res == nil {
		fmt.Printf("[Audit] Nó '%s' não encontrado por campo exato.\n", nodeID)
	}

	if err == nil && res != nil {
		return map[string]interface{}{
			"path":    res["path"],
			"content": res["content"],
			"type":    res["type"],
			"source":  res["document-type"], // Retorna se é "system" ou "vault"
		}, nil
	}

	// 2. Tentar buscar em Memórias de Chat (Chave: subject)
	res, err = a.qdrant.SearchByField("knowledge_graph", "subject", nodeID)
	if err == nil && res != nil {
		return map[string]interface{}{
			"path":    "Memória de Chat",
			"content": res["content"],
			"type":    "memory",
			"source":  "RAG Synapse",
		}, nil
	}

	// 3. Fallback: Se não existe no banco, é uma dedução/especulação da IA (Nó Virtual)
	return map[string]interface{}{
		"path":    "Conceito Neural",
		"content": fmt.Sprintf("O nó '%s' é um conceito abstrato detectado pela IA durante a tecelagem do conhecimento. Ele ainda não possui uma nota física dedicada no seu Obsidian.", nodeID),
		"type":    "virtual",
		"source":  "Inteligência Artificial",
	}, nil
}

// AnalyzeGraphHealth analisa a integridade semântica do grafo.
func (a *App) AnalyzeGraphHealth() (map[string]interface{}, error) {
	fmt.Println("[Audit] Analisando saúde do Grafo de Contexto...")

	count, err := a.qdrant.CountPoints("obsidian_knowledge")
	if err != nil {
		return nil, err
	}

	// Cálculo de Densidade Orgânica (Progressão Logarítmica)
	// Com 816 notas, queremos um valor que faça sentido visual.
	densityValue := 0.05 // Base 5%
	if count > 0 {
		// Quanto mais notas, mais o cérebro se torna denso (Log10)
		// Ex: Log10(816) ~ 2.9. 2.9 * 0.15 = 0.43 + 0.05 = 48%
		densityValue += (float64(count) / 1000.0) * 0.2 // Linear suave até 1000 notas
	}
	if densityValue > 1.0 { densityValue = 1.0 }

	stats := map[string]interface{}{
		"density":      densityValue,
		"conflicts":    0,
		"active_nodes": count,
	}

	// 🧠 Dispara cálculos pesados do Cérebro Relacional de forma assíncrona ou síncrona dependendo da escala
	if a.GEngine != nil {
		a.GEngine.ComputePageRank()    // Centralidade de Autoridade
		a.GEngine.ComputeCommunities() // Afinidade Semântica
		a.GEngine.ComputeBetweenness() // Notas Ponte (Gargalos)
		a.GEngine.ComputeHITS()        // Hubs vs Authorities
		
		cycles := a.GEngine.DetectCycles()
		stats["conflicts"] = len(cycles)
		stats["communities"] = countCommunities(a.GEngine)
	}

	// Gatilho: Se o usuário pediu saúde, aproveitamos para tecer pontes neurais
	// Aumentamos o lote de processamento conforme o tamanho do cofre
	batchSize := 100
	if count > 500 { batchSize = 250 }
	go a.WeaveNeuralLinks(batchSize)

	return stats, nil
}

// WeaveNeuralLinks percorre o grafo e cria conexões por similaridade (brain mapping).
func (a *App) WeaveNeuralLinks(limit int) {
	fmt.Printf("[Neural] Tecendo pontes em lote de %d notas...\n", limit)
	
	// 1. Busca as notas (as 50 mais recentes + uma amostra aleatória se possível)
	notes, err := a.qdrant.Search("obsidian_knowledge", nil, limit)
	if err != nil || len(notes) == 0 {
		return
	}

	for _, note := range notes {
		name, _ := note["name"].(string)
		content, _ := note["content"].(string)
		if name == "" || content == "" {
			continue
		}

		// 2. Usamos o embedding para encontrar vizinhos próximos
		vector, err := a.embedder.GenerateEmbedding(a.ctx, content)
		if err != nil {
			continue
		}

		// 3. Busca os 5 vizinhos mais próximos (aumentado de 3 para 5)
		similars, err := a.qdrant.SearchWithScores("obsidian_knowledge", vector, 6) 
		if err != nil {
			continue
		}

		for _, sim := range similars {
			targetName, _ := sim["name"].(string)
			score, _ := sim["_score"].(float64)

			// Filtro de Qualidade: Score > 0.82 (Sensibilidade ajustada)
			if targetName == "" || targetName == name || score < 0.82 {
				continue
			}

			// Emite link visual (Peso maior para similaridade alta)
			runtime.EventsEmit(a.ctx, "graph:edge", map[string]interface{}{
				"source": strings.ToLower(name),
				"target": strings.ToLower(targetName),
				"weight": int(score * 6), // Reforço visual
				"type":   "neural",
			})
		}
	}
	fmt.Println("[Neural] ✅ Tecelagem concluída para o lote.")
}

// OpenFileInEditor abre o arquivo fonte usando o handler padrão do SO.
func (a *App) OpenFileInEditor(path string) error {
	fmt.Printf("[App] Abrindo arquivo na fonte: %s\n", path)
	// No Windows usamos 'cmd /c start'
	cmd := exec.Command("cmd", "/c", "start", "", path)
	return cmd.Run()
}

// SendTerminalData envia input do usuário para o processo do terminal (stdin).
func (a *App) SendTerminalData(agent string, data string) {
	sessionID := "acp-session-" + agent
	a.executor.SendInput(sessionID, data, nil)
}

func (a *App) ResizeTerminal(agent string, cols int, rows int) {
	// Ignored on JSON RPC mode.
}

// StopAgentSession encerra a sessão ativa.
func (a *App) StopAgentSession(agent string) error {
	sessionID := "acp-session-" + agent
	err := a.executor.StopSession(sessionID)
	if err != nil {
		return fmt.Errorf("nenhuma sessão ativa ACP encontrada para %s", agent)
	}

	runtime.EventsEmit(a.ctx, "terminal:closed", agent)
	return nil
}

// ============================================================
// NOVAS INTEGRAÇÕES (Autonomia, Regras e MCP)
// ============================================================

// SetAutonomousMode ativa ou desativa globalmente o modo YOLO
func (a *App) SetAutonomousMode(enabled bool) string {
	a.executor.AutonomousMode = enabled
	if enabled {
		return "Modo Autônomo ATIVADO. Executará tarefas de terminal sem permissão (Comandos destrutivos ainda requerem review de Hands Security)."
	}
	return "Modo Autônomo DESATIVADO. A CLI voltará a pedir aprovação."
}

// SubmitReview aprova ou rejeita uma ação pendente da IA
func (a *App) SubmitReview(id string, approved bool) {
	a.executor.SubmitReview(id, approved)
}

// GenerateGeminiMD cria um arquivo base GEMINI.md no diretório atual
func (a *App) GenerateGeminiMD() string {
	content := `# Project Instructions

Você agora está sendo orquestrado pelo Lumaestro (Modo ACP).

- **Manejo de Arquivos**: O Backend ditará suas permissões. Se receber "Acesso Negado", pergunte ao usuário.
- **Autonomia Limitada**: Só prossiga ativamente se a sessão permitir.

`
	err := os.WriteFile("GEMINI.md", []byte(content), 0644)
	if err != nil {
		return "Erro ao gerar arquivo de contexto: " + err.Error()
	}
	return "Contexto GEMINI.md gerado com sucesso no diretório atual!"
}

// 🧠 NEURAL BINDINGS: Métodos que expõem o aprendizado ativo para a UI

// HandleNodeClick recebe o feedback positivo (clique) e aplica reforço sináptico.
func (a *App) HandleNodeClick(nodeID string) {
	if a.ranker != nil {
		a.ranker.Reinforce(nodeID)

		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "NEURAL",
			"content": fmt.Sprintf("🧠 Reforço sináptico aplicado ao nó: %s", nodeID),
		})
	}
}

// SetExplorationMode ativa ou desativa o filtro neural no grafo.
func (a *App) SetExplorationMode(enabled bool) string {
	if a.ranker != nil {
		a.ranker.SetExplorationMode(enabled)
		if enabled {
			return "Modo Exploração Ativado (Pesos neurais ignorados)."
		}
		return "Modo Neural Ativado (Pesos aprendidos influenciam o grafo)."
	}
	return "Motor neural não inicializado."
}

// IsExplorationMode retorna o estado atual para sincronização da UI.
func (a *App) IsExplorationMode() bool {
	if a.ranker != nil {
		return a.ranker.IsExplorationMode()
	}
	return false
}

// AddMCPServer instala um novo servidor MCP na CLI local
func (a *App) AddMCPServer(name string, command string) string {
	cmd := exec.Command("cmd", "/c", "gemini", "mcp", "add", name, command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("Erro ao adicionar MCP: %s\nOutput: %s", err.Error(), string(output))
	}
	return fmt.Sprintf("MCP '%s' adicionado com sucesso!\n%s", name, string(output))
}

// ListMCPServers retorna a lista de MCPs instalados
func (a *App) ListMCPServers() string {
	cmd := exec.Command("cmd", "/c", "gemini", "mcp", "list")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return fmt.Sprintf("Erro ao listar MCPs: %s\nOutput: %s", err.Error(), string(output))
	}
	return string(output)
}

// AddGeminiAccount adiciona uma nova conta e prepara seu diretório de sessão
func (a *App) AddGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	accountPath := filepath.Join(cwd, ".gemini_accounts", name)

	// Cria o diretório de sessão se não existir
	if err := os.MkdirAll(accountPath, 0755); err != nil {
		return fmt.Errorf("falha ao criar pasta de conta: %w", err)
	}

	// Verifica se já existe na config
	for i := range cfg.GeminiAccounts {
		if cfg.GeminiAccounts[i].Name == name {
			cfg.GeminiAccounts[i].HomeDir = accountPath
			return config.Save(*cfg)
		}
	}

	cfg.GeminiAccounts = append(cfg.GeminiAccounts, config.GeminiAccount{
		Name:    name,
		HomeDir: accountPath,
		Active:  false,
	})

	return config.Save(*cfg)
}

// LoginGeminiAccount abre um terminal para realizar o login OAuth em uma conta específica
func (a *App) LoginGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var targetDir string
	for _, acc := range cfg.GeminiAccounts {
		if acc.Name == name {
			targetDir = acc.HomeDir
			break
		}
	}

	if targetDir == "" {
		return fmt.Errorf("conta '%s' não encontrada ou sem diretório configurado", name)
	}

	// Comando para abrir o terminal com GEMINI_CLI_HOME isolado
	binaryPath := "gemini"
	if _, err := exec.LookPath("gemini"); err != nil {
		cwd, _ := os.Getwd()
		binaryPath = filepath.Join(cwd, "node_modules", ".bin", "gemini.cmd")
	}

	// Script para o PowerShell forçar o ambiente de sessão desta conta
	script := fmt.Sprintf(`$env:GEMINI_CLI_HOME='%s'; $env:NO_BROWSER='true'; & '%s' login`, targetDir, binaryPath)

	fmt.Printf("[Maestro] 🔑 Iniciando fluxo de Login OAuth para: %s\n", name)
	return exec.Command("cmd", "/c", "start", "powershell", "-NoExit", "-Command", script).Run()
}

// SwitchGeminiAccount alterna a conta ativa do Gemini e reinicia a sessão
func (a *App) SwitchGeminiAccount(name string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	found := false
	for i := range cfg.GeminiAccounts {
		if cfg.GeminiAccounts[i].Name == name {
			cfg.GeminiAccounts[i].Active = true
			found = true
		} else {
			cfg.GeminiAccounts[i].Active = false
		}
	}

	if !found {
		return fmt.Errorf("conta '%s' não encontrada", name)
	}

	if err := config.Save(*cfg); err != nil {
		return err
	}

	fmt.Printf("[Maestro] 🔄 Trocando para sessão de login: %s\n", name)
	return a.StartAgentSession("gemini")
}

// 🛡️ checkRogueMainFiles escaneia subpastas procurando arquivos .go com "package main"
// que causariam conflito silencioso durante o build do Wails (go build ./...).
// Se encontrar, emite um AVISO GIGANTE no terminal para o desenvolvedor.
func checkRogueMainFiles() {
	rogueFiles := []string{}

	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		// Ignora a raiz (main.go e app.go são legítimos), build/ e frontend/
		dir := filepath.Dir(path)
		if dir == "." || strings.HasPrefix(path, "build") || strings.HasPrefix(path, "frontend") {
			return nil
		}
		// Só arquivos .go
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		// Lê as primeiras linhas para checar "package main"
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(data)
		if len(content) > 200 {
			content = content[:200]
		}
		if strings.Contains(content, "package main") {
			rogueFiles = append(rogueFiles, path)
		}
		return nil
	})

	if len(rogueFiles) > 0 {
		fmt.Println("")
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║  ⚠️  ALERTA: ARQUIVOS GO CONFLITANTES DETECTADOS!           ║")
		fmt.Println("║                                                              ║")
		fmt.Println("║  Os seguintes arquivos contêm 'package main' em subpastas:   ║")
		fmt.Println("║  Isso QUEBRA o 'wails dev' silenciosamente!                  ║")
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		for _, f := range rogueFiles {
			fmt.Printf("║  🔴 %s\n", f)
		}
		fmt.Println("╠══════════════════════════════════════════════════════════════╣")
		fmt.Println("║  SOLUÇÃO: Delete ou mova esses arquivos para fora do projeto ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Println("")
	}
}

// ============================================================
// ORQUESTRAÇÃO SWARM (PAPERCLIP MODE)
// ============================================================

func (a *App) startOrchestration() {
	orchestration.StartHeartbeatDaemon(a.handleAgentWakeUp)
}

func (a *App) handleAgentWakeUp(agent db.Agent, runID uuid.UUID) {
	sessionID := "acp-session-" + agent.ID.String()

	// 1. Buscar Ocupação Atual ou Nova Tarefa
	var issue db.Issue
	err := db.InstanceDB.Where("assignee_agent_id = ? AND status = ?", agent.ID, "in_progress").First(&issue).Error
	
	isNewTask := false
	if err != nil {
		// Tenta buscar algo novo na fila (TODO)
		err = db.InstanceDB.Where("status = ? AND (assignee_agent_id IS NULL OR assignee_agent_id = ?)", "todo", agent.ID).First(&issue).Error
		if err == nil {
			// Realiza Checkout Atômico
			lockedIssue, lockedErr := orchestration.CheckoutIssue(agent.ID, issue.ID)
			if lockedErr != nil {
				orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Conflito de checkout na fila.")
				return
			}
			issue = *lockedIssue
			isNewTask = true
		} else {
			// Nada para fazer
			orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Nenhuma tarefa pendente na fila.")
			return
		}
	}

	// 2. Iniciar ou Reutilizar Sessão ACP vinculada à Identidade
	err = a.executor.StartSession(a.ctx, "gemini", sessionID, "LATEST", agent.ID, &issue.ID)
	if err != nil {
		orchestration.FinalizeHeartbeat(agent.ID, runID, false, "Erro ACP Swarm: "+err.Error())
		return
	}

	// 3. Construir Fat Context (Timeline + Metas)
	timeline, _ := orchestration.GetTimelineByIssue(issue.ID)
	historyStr := ""
	for i, c := range timeline {
		if i > 5 { break } // Apenas os últimos 5 para economia de tokens
		author := "Sistema"
		if c.AuthorAgentID != nil { author = "Agente" }
		historyStr += fmt.Sprintf("- %s: %s\n", author, c.Body)
	}

	prompt := ""
	if isNewTask {
		prompt = fmt.Sprintf("Você é o agente corporativo %s (Cargo: %s). Você acaba de assumir a tarefa: %s\nDescrição: %s\nInicie o trabalho imediatamente. Use as ferramentas de 'Lumaestro/' para Handoff ou Conclusão se necessário.", agent.Name, agent.Role, issue.Title, issue.Description)
	} else {
		prompt = fmt.Sprintf("Você é o agente corporativo %s (Cargo: %s). Continuando tarefa: %s\nHistórico recente:\n%s\nPor favor, prossiga com os próximos passos.", agent.Name, agent.Role, issue.Title, historyStr)
	}

	// 3. Injetar Pulso de Inteligência no Agente Concorrente
	// O SendInput é assíncrono e lidará com o fluxo JSON-RPC
	err = a.executor.SendInput(sessionID, prompt, nil)
	if err != nil {
		orchestration.FinalizeHeartbeat(agent.ID, runID, false, "Erro ao injetar comando: "+err.Error())
		return
	}

	// Reporta que o pulso foi injetado com sucesso
	orchestration.FinalizeHeartbeat(agent.ID, runID, true, "Agente acordado e processando tarefa.")
}

// Bindings para interação do Front-End (Wails)

// CreateAgent 'contrata' um novo agente no banco de dados corporativo local
func (a *App) CreateAgent(name, role string, listSkills string, budget int) string {
	agent := db.Agent{
		Name:               name,
		Role:               role,
		Capabilities:       listSkills,
		BudgetMonthlyCents: budget,
		Status:             "idle",
	}
	if err := db.InstanceDB.Create(&agent).Error; err != nil {
		return "Erro ao contratar: " + err.Error()
	}
	return "Agente " + name + " contratado e aguardando pulso de vida!"
}

// CreateTask adiciona uma nova tarefa atômica na fila da empresa
func (a *App) CreateTask(title, description, priority string) string {
	issue := db.Issue{
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      "todo",
	}
	if err := db.InstanceDB.Create(&issue).Error; err != nil {
		return "Erro ao criar tarefa: " + err.Error()
	}
	return "Tarefa '" + title + "' injetada na fila de orquestração."
}

// GetAgents retorna a lista de todos os agentes contratados
func (a *App) GetAgents() []db.Agent {
	var agents []db.Agent
	db.InstanceDB.Find(&agents)
	return agents
}

// GetIssues retorna todas as tarefas e seus respectivos donos (agentes)
func (a *App) GetIssues() []db.Issue {
	var issues []db.Issue
	// Preload carrega o objeto Agent associado via Foreign Key
	db.InstanceDB.Preload("AssigneeAgent").Find(&issues)
	return issues
}

// --- GOVERNANÇA V2 (METAS, TIMELINE E APROVAÇÕES) ---

// CreateGoal cria um novo objetivo estratégico para nortear o enxame
func (a *App) CreateGoal(title, description, level, parentIDStr, ownerIDStr string) string {
	var parentID *uuid.UUID
	if parentIDStr != "" {
		u, err := uuid.Parse(parentIDStr)
		if err == nil { parentID = &u }
	}
	var ownerID *uuid.UUID
	if ownerIDStr != "" {
		u, err := uuid.Parse(ownerIDStr)
		if err == nil { ownerID = &u }
	}

	goal := db.Goal{
		Title:        title,
		Description:  description,
		Level:        level,
		ParentID:     parentID,
		OwnerAgentID: ownerID,
	}
	if err := db.InstanceDB.Create(&goal).Error; err != nil {
		return "Erro ao criar meta: " + err.Error()
	}
	return "Meta '" + title + "' estabelecida no plano estratégico!"
}

// GetGoals lista a árvore de objetivos da empresa
func (a *App) GetGoals() []db.Goal {
	var goals []db.Goal
	db.InstanceDB.Find(&goals)
	return goals
}

// AddComment insere uma nota na linha do tempo de uma tarefa (Audit Chain)
func (a *App) AddComment(issueIDStr, body string) string {
	issueID, _ := uuid.Parse(issueIDStr)
	// Comentário manual via UI usa Actor System (uuid.Nil)
	err := orchestration.AddIssueComment(uuid.Nil, issueID, body)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Nota registrada na tarefa."
}

// GetIssueTimeline recupera a história completa de uma tarefa
func (a *App) GetIssueTimeline(issueIDStr string) []db.IssueComment {
	issueID, _ := uuid.Parse(issueIDStr)
	comments, _ := orchestration.GetTimelineByIssue(issueID)
	return comments
}

// ApproveAction libera um Portão de Aprovação (Board Decision)
func (a *App) ApproveAction(approvalIDStr, note string) string {
	id, _ := uuid.Parse(approvalIDStr)
	err := orchestration.ProcessApproval(id, true, note)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Ação aprovada e registrada na auditoria."
}

// RejectAction bloqueia permanentemente uma intenção da IA
func (a *App) RejectAction(approvalIDStr, note string) string {
	id, _ := uuid.Parse(approvalIDStr)
	err := orchestration.ProcessApproval(id, false, note)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Ação rejeitada. O agente permanecerá em pausa para reavaliação."
}

// --- SUITE EXECUTIVA (KPIs E DOCUMENTAÇÃO RAG) ---

// GetExecutiveSummary retorna os KPIs do Enxame para o Dashboard de Comando
func (a *App) GetExecutiveSummary() orchestration.ExecSummary {
	summary, _ := orchestration.GetExecutiveSummary()
	return summary
}

// GetDocuments retorna a lista de documentos (entregas) de uma tarefa
func (a *App) GetDocuments(issueIDStr string) []db.Document {
	issueID, _ := uuid.Parse(issueIDStr)
	var docs []db.Document
	db.InstanceDB.Where("issue_id = ?", issueID).Find(&docs)
	return docs
}

// UpsertDocument sincroniza um documento (PRD, Spec, Relatório) com o banco e o RAG
func (a *App) UpsertDocument(issueIDStr, title, body, change string) string {
	issueID, _ := uuid.Parse(issueIDStr)
	// Operação manual via UI usa Actor System (uuid.Nil)
	_, err := orchestration.UpsertDocument(uuid.Nil, issueID, title, body, change)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "Documento '" + title + "' guardado e indexado para o RAG."
}
// --- GESTÃO DE SEGREDOS (AGENT VAULT) ---

// GetAgentSecrets retorna as chaves de API cadastradas para um agente
func (a *App) GetAgentSecrets(agentIDStr string) []db.AgentSecret {
	agentID, _ := uuid.Parse(agentIDStr)
	var secrets []db.AgentSecret
	db.InstanceDB.Where("agent_id = ?", agentID).Find(&secrets)
	return secrets
}

// UpdateAgentSecret salva ou atualiza uma credencial (ex: OPENAI_API_KEY) para um agente
func (a *App) UpdateAgentSecret(agentIDStr, key, value string) string {
	agentID, _ := uuid.Parse(agentIDStr)
	var secret db.AgentSecret
	err := db.InstanceDB.Where("agent_id = ? AND key = ?", agentID, key).First(&secret).Error
	
	if err != nil {
		secret = db.AgentSecret{AgentID: agentID, Key: key, Value: value}
		db.InstanceDB.Create(&secret)
	} else {
		secret.Value = value
		db.InstanceDB.Save(&secret)
	}
	return "Segredo '" + key + "' atualizado para o agente."
}

// GetPendingApprovals retorna todas as solicitações de aprovação que aguardam decisão humana.
func (a *App) GetPendingApprovals() []db.Approval {
	var approvals []db.Approval
	db.InstanceDB.Where("status = ?", "pending").Order("created_at DESC").Find(&approvals)
	return approvals
}

// --- ⚡ MÓDULO LIGHTNING (DASHBOARD ANALÍTICO) ---

// GetLightningStats retorna estatísticas calculadas pelo DuckDB para o Dashboard.
func (a *App) GetLightningStats() map[string]interface{} {
	stats := make(map[string]interface{})
	if a.LStore == nil {
		return map[string]interface{}{"status": "offline"}
	}

	// 1. Total de Rollouts
	var totalRollouts int64
	a.LStore.GetDB().QueryRow("SELECT count(*) FROM rollouts").Scan(&totalRollouts)
	stats["total_rollouts"] = totalRollouts

	// 2. Média de Recompensa
	var avgReward float64
	a.LStore.GetDB().QueryRow("SELECT avg(reward) FROM rewards").Scan(&avgReward)
	stats["avg_reward"] = avgReward

	// 💰 3. Métricas Financeiras (Tokens e Custo Estimado)
	var pTokens, cTokens int64
	a.LStore.GetDB().QueryRow("SELECT sum(prompt_tokens), sum(completion_tokens) FROM spans").Scan(&pTokens, &cTokens)
	stats["prompt_tokens"] = pTokens
	stats["completion_tokens"] = cTokens
	
	// Estimativa: $0.15/1M in, $0.60/1M out (Gemini Flash)
	totalUSD := (float64(pTokens)*0.15 + float64(cTokens)*0.60) / 1000000.0
	stats["total_cost_usd"] = totalUSD

	// 3. Status do Proxy
	stats["status"] = "online"
	
	return stats
}

// TriggerReflection destila o conhecimento de um rollout específico no Obsidian.
func (a *App) TriggerReflection(rolloutID string) string {
	if a.LReflector == nil {
		return "Erro: Motor de Reflexão não inicializado."
	}
	err := a.LReflector.DistillLesson(rolloutID)
	if err != nil {
		return "Erro na reflexão: " + err.Error()
	}
	return "Sucesso: Lição destilada no Obsidian Vault!"
}

// startAPOWorker monitora o desempenho do enxame e sugere otimizações (APO).
func (a *App) startAPOWorker() {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.LStore == nil || a.LOptimizer == nil { continue }

			// 1. Identificar agentes com "Dopamina Crítica" (Média < -0.2 nos últimos rollouts)
			var agentIDStr string
			var avgReward float64
			err := a.LStore.GetDB().QueryRow(`
				SELECT agent_name, avg(reward) as avg_r 
				FROM rewards r
				JOIN spans s ON r.rollout_id = s.rollout_id
				GROUP BY agent_name 
				HAVING avg_r < -0.2
				LIMIT 1
			`).Scan(&agentIDStr, &avgReward)

			if err == nil && agentIDStr != "" {
				fmt.Printf("[🧠 APO Cortex] Desempenho crítico para %s (RR: %.2f). Iniciando Evolução...\n", agentIDStr, avgReward)
				
				// 2. Obter o prompt atual (do config ou do DB)
				currentPrompt := "Você é o Maestro, um assistente técnico de elite."
				if latest, err := a.LStore.GetLatestPrompt(agentIDStr); err == nil && latest != "" {
					currentPrompt = latest
				}

				// 3. Gerar a Crítica APO
				criticInput, failures, err := a.LOptimizer.RefinePrompt(a.ctx, agentIDStr, currentPrompt)
				if err != nil || failures == "Nenhuma falha crítica detectada." { continue }

				// 4. Chamar o LLM para gerar o FEIXE de 3 candidatos (Com Resiliência Automática)
				fmt.Println("[🧠 APO Beam] Gerando 3 variantes de evolução estratégica com Escudo de Resiliência...")
				beamOutput, provider, err := a.LRouter.ExecuteWithFallback(a.ctx, "", criticInput)
				if err == nil && beamOutput != "" {
					fmt.Printf("[🧪 RESILIÊNCIA] Variantes geradas via: %s\n", provider)
					// 5. Novo: Loop de Regressão Gold
					goldSamples, _ := a.LStore.GetGoldSamples(agentIDStr)
					
					re := regexp.MustCompile(`(?s)<variant name="([^"]+)">\s*<critique>(.*?)</critique>\s*<prompt>(.*?)</prompt>\s*</variant>`)
					matches := re.FindAllStringSubmatch(beamOutput, -1)

					for _, m := range matches {
						name, critique, content := m[1], m[2], m[3]
						
						// Calcular Acurácia contra os "Gold Samples"
						accuracy := 100.0
						if len(goldSamples) > 0 {
							hits := 0
							for _, gs := range goldSamples {
								// Executa o novo prompt contra o input de ouro (Com Fallback)
								fmt.Printf("[🧪 TEST] Validando variante '%s' contra Caso de Ouro (Manto Ativo)...\n", name)
								testOutput, _, err := a.LRouter.ExecuteWithFallback(a.ctx, content, gs["input"])
								if err == nil && strings.Contains(strings.ToLower(testOutput), strings.ToLower(gs["output"])) {
									hits++
								}
							}
							accuracy = (float64(hits) / float64(len(goldSamples))) * 100.0
						}

						a.LStore.InsertCandidate(agentIDStr, name, content, critique, accuracy)
					}

					if len(matches) > 0 {
						fmt.Printf("[✨ BEAM SUCCESS] %d candidatos validados (Gold Check) para %s!\n", len(matches), agentIDStr)
						runtime.EventsEmit(a.ctx, "lightning:beam_ready", agentIDStr)
					}
				}
			}
		}
	}
}

// GetLatestSpans retorna os últimos traces analíticos do DuckDB para o Dashboard.
func (a *App) GetLatestSpans() []map[string]interface{} {
	if a.LStore == nil { return []map[string]interface{}{} }
	
	rows, err := a.LStore.GetDB().Query(`
		SELECT rollout_id, attempt_id, name, prompt_tokens + completion_tokens as tokens, start_time,
		       attributes->>'$.image_path' as media
		FROM spans 
		ORDER BY start_time DESC 
		LIMIT 10
	`)
	if err != nil {
		fmt.Printf("[App] Erro ao buscar spans: %v\n", err)
		return []map[string]interface{}{}
	}
	defer rows.Close()

	var spans []map[string]interface{}
	for rows.Next() {
		var rid, aid, name string
		var tokens int
		var startTime float64
		var media sql.NullString
		rows.Scan(&rid, &aid, &name, &tokens, &startTime, &media)
		
		spans = append(spans, map[string]interface{}{
			"id": rid,
			"agent": aid,
			"op": name,
			"usage": tokens,
			"media": media.String,
			"time": time.Unix(int64(startTime), 0).Format("15:04:05"),
		})
	}
	return spans
}

// ExportTelemetry exporta os traces do DuckDB para um arquivo JSON estruturado.
func (a *App) ExportTelemetry() string {
	if a.LStore == nil { return "⚠️ Motor analítico offline." }
	path := "lumaestro_telemetry_export.json"
	err := lightning.ExportSpansToJSON(a.LStore, path)
	if err != nil { return "🔴 Erro na exportação: " + err.Error() }
	return "✅ Telemetria de Elite exportada para: " + path
}

// GetPromptHistory retorna o histórico de evolução de prompts de um agente.
func (a *App) GetPromptHistory(agentName string) []map[string]interface{} {
	if a.LStore == nil { return nil }
	rows, err := a.LStore.GetDB().Query(`
		SELECT content, avg_reward, created_at 
		FROM prompts 
		WHERE agent_name = ? 
		ORDER BY created_at DESC 
		LIMIT 10`, agentName)
	if err != nil { return nil }
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var content string
		var reward, createdAt float64
		if err := rows.Scan(&content, &reward, &createdAt); err == nil {
			history = append(history, map[string]interface{}{
				"content": content,
				"reward": reward,
				"date": time.Unix(int64(createdAt), 0).Format("02/01 15:04"),
			})
		}
	}
	return history
}

// GetPromptCandidates retorna os candidatos aguardando aprovação.
func (a *App) GetPromptCandidates() []map[string]interface{} {
	if a.LStore == nil { return nil }
	cands, _ := a.LStore.GetPendingCandidates()
	return cands
}

// ApprovePromptVariant aprova um candidato e o torna o prompt oficial.
func (a *App) ApprovePromptVariant(candidateID string) string {
	if a.LStore == nil { return "🔴 Motor analítico offline." }
	err := a.LStore.ApproveCandidate(candidateID)
	if err != nil { return "🔴 Erro na aprovação: " + err.Error() }
	return "✅ Variante aprovada com sucesso! Evolução concluída."
}

// AddGoldSample registra manualmente uma interação perfeita como referência de regressão.
func (a *App) AddGoldSample(agentName, input, output string) string {
	if a.LStore == nil { return "🔴 Motor analítico offline." }
	err := a.LStore.InsertGoldSample(agentName, input, output)
	if err != nil { return "🔴 Erro ao salvar: " + err.Error() }
	return "💎 Caso de Ouro registrado! O enxame usará isso para validar futuras evoluções."
}

// ExportRLHFDataset gera um arquivo JSONL com conversas perfeitas para Fine-tuning.
func (a *App) ExportRLHFDataset() string {
	if a.LStore == nil { return "⚠️ Motor analítico offline." }
	path := "dataset_lumaestro_rlhf.jsonl"
	
	file, err := os.Create(path)
	if err != nil { return "🔴 Erro ao criar arquivo: " + err.Error() }
	defer file.Close()

	// 1. Exportar Gold Samples como SFT (Conversational)
	rows, _ := a.LStore.GetDB().Query(`SELECT agent_name, input, output FROM gold_samples`)
	defer rows.Close()

	count := 0
	for rows.Next() {
		var agent, in, out string
		rows.Scan(&agent, &in, &out)

		entry := map[string]interface{}{
			"messages": []map[string]string{
				{"role": "system", "content": "Você é o agente " + agent + " do enxame Lumaestro."},
				{"role": "user", "content": in},
				{"role": "assistant", "content": out},
			},
		}
		data, _ := json.Marshal(entry)
		file.Write(data)
		file.Write([]byte("\n"))
		count++
	}

	return fmt.Sprintf("✅ Fábrica RLHF: %d exemplos de elite exportados para: %s", count, path)
}

// SendMessageToSwarm permite ao Comandante intervir diretamente no enxame via Dashboard.
func (a *App) SendMessageToSwarm(agentName, message string) string {
	if a.LRouter == nil { return "🔴 Roteador offline." }
	
	fmt.Printf("[🕹️ COMANDO] Enviando ordem para %s: %s\n", agentName, message)
	
	// Executa a ordem com resiliência total
	response, provider, err := a.LRouter.ExecuteWithFallback(a.ctx, "Você é o Maestro do enxame Lumaestro. Responda à ordem do Comandante de forma executiva.", message)
	if err != nil { return "🔴 Falha no comando: " + err.Error() }
	
	return fmt.Sprintf("💎 [%s] Resposta do Enxame: %s", provider, response)
}

// RunReconScan dispara a busca por conexões perdidas e informa o frontend das propostas.
func (a *App) RunReconScan() string {
	if a.Recon == nil { return "🔴 Agente Recon offline." }
	
	proposals, err := a.Recon.ScanMissingLinks(a.ctx)
	if err != nil { return "🔴 Erro no Scan: " + err.Error() }
	
	count := 0
	for _, p := range proposals {
		if !p.Auto {
			count++
			runtime.EventsEmit(a.ctx, "agent:proposal", p)
		}
	}
	
	return fmt.Sprintf("🕵️‍♂️ Recon Scan concluído! %d novas sinapses propostas para sua revisão.", count)
}

// PruneGraph executa a poda neural baseada em PageRank para limpar o Dashboard.
func (a *App) PruneGraph(threshold float64) string {
	if a.GEngine == nil { return "🔴 Motor de grafos offline." }
	
	removed := a.GEngine.Prune(threshold)
	if len(removed) > 0 {
		// Sincroniza visualmente (Reset de Grafo no Front)
		a.SyncAllNodes()
		return fmt.Sprintf("🧹 Poda Neural concluída: %d nós irrelevantes removidos.", len(removed))
	}
	
	return "✅ O grafo já está otimizado (sem nós abaixo do threshold)."
}

// GetSkeletalGraph retorna apenas as arestas vitais (MST) para despoluir a visão.
func (a *App) GetSkeletalGraph() map[string]interface{} {
	if a.GEngine == nil { return nil }
	
	mstEdges := a.GEngine.GetMSTEdges()
	return map[string]interface{}{
		"edges": mstEdges,
	}
}

// helper para contar comunidades únicas
func countCommunities(ge *rag.GraphEngine) int {
	return 5 // Placeholder estático para o HUD
}
