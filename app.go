package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"Lumaestro/internal/agents"
	"Lumaestro/internal/config"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag"
	"Lumaestro/internal/tools"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx       context.Context
	executor      *agents.ACPExecutor
	orchestrator  *agents.Orchestrator
	legacyExec    *agents.Executor // Apenas para ExecuteCLI fallback se necessário, ou podemos migrar.
	ontology  *provider.OntologyService
	crawler   *obsidian.Crawler
	qdrant    *provider.QdrantClient
	embedder  *provider.EmbeddingService
	chat      *rag.ChatService
	weaver    *rag.KnowledgeWeaver
	navigator *rag.GraphNavigator
	installer *tools.Installer
	config    *config.Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	// 🛡️ Detector de arquivos Go órfãos que quebram o Wails silenciosamente
	checkRogueMainFiles()

	a.ctx = ctx
	a.executor = agents.NewACPExecutor()
	a.orchestrator = agents.NewOrchestrator(a.executor)
	a.legacyExec = agents.NewExecutor() // Mantemos temporariamente para métodos legacy
	a.installer = tools.NewInstaller()

	// Sincroniza o PATH imediatamente (Garante que claude/gemini sejam encontrados)
	a.installer.SyncPath()

	// Tenta inicializar os serviços logo na subida
	if err := a.initServices(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no initServices: %v\n", err)
	}

	// Iniciar a Escuta de Logs e Terminal
	go a.listenForLogs()
	go a.listenForInstallerLogs()
	go a.listenForTerminalOutput()

	// 🚀 Auto-Start: Inicia os agentes e sincroniza conhecimento no boot
	if a.config != nil && a.config.GeminiAPIKey != "" {
		go func() {
			time.Sleep(2000 * time.Millisecond)
			fmt.Println("[BOOT] Maestro Online. Sincronizando conhecimento e restaurando agentes...")
			
			// 1. Inicia o Agente Padrão
			a.StartAgentSession("gemini")

			// 2. Indexação Silenciosa (RAG)
			if a.crawler != nil && a.config.ObsidianVaultPath != "" {
				fmt.Println("[BOOT] Iniciando Auto-Scan do Obsidian em background...")
				a.ScanVault()
			}
		}()
	}
}

// initServices inicializa os motores de IA e RAG se as configurações permitirem
func (a *App) initServices() error {
	cfg, err := config.Load()
	if err != nil || cfg == nil {
		fmt.Printf("⚠️ Arquivo de configuração não encontrado ou vazio. Aguardando setup na UI...\n")
		return nil // Não retorna erro crítico. Permite o App iniciar sem motores.
	}
	a.config = cfg

	// Inicializa Qdrant e Embeddings
	a.qdrant = provider.NewQdrantClient(cfg.QdrantURL)
	emb, err := provider.NewEmbeddingService(a.ctx, cfg.GeminiAPIKey)
	if err != nil {
		return err
	}

	a.embedder = emb
	a.ontology = provider.NewOntologyService(a.embedder.Client)
	
	search := rag.NewSearchService(a.qdrant)
	a.navigator = rag.NewGraphNavigator(a.qdrant)
	a.weaver = rag.NewKnowledgeWeaver(a.ontology, a.qdrant, a.embedder)
	
	a.chat = rag.NewChatService(a.legacyExec, a.orchestrator, search, a.navigator, a.embedder, a.installer)
	a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology)

	// 🔥 Injeção de Autonomia: Maestro agora pode comandar o Crawler
	a.executor.Tools.Indexer = a.crawler

	return nil
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
	fmt.Printf("[BACKEND] AskAgent chamado para: %s com prompt: %s\n", agentName, prompt)
	// No modo TUDO ACP, as perguntas normais deverão ser injetadas na sessão principal ACP!
	if a.chat == nil {
		if err := a.initServices(); err != nil {
			return "⚠️ O motor do Maestro está desligado. Por favor, verifique sua Gemini API Key nas configurações."
		}
	}

	if agentName == "" {
		agentName = "gemini"
	}

	// Modo Legado AskAgent (RAG)
	if a.chat == nil {
		if err := a.initServices(); err != nil {
			return "⚠️ O motor do Maestro está desligado. Por favor, verifique sua Gemini API Key nas configurações."
		}
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
		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "CRAWLER",
			"content": "🚀 Iniciando Sincronização Semântica Completa em background...",
		})

		// 1. Indexar o cofre do Obsidian (Usuário)
		err := a.crawler.IndexVault(a.ctx)
		if err != nil {
			fmt.Printf("[BACKEND] Erro na Indexação do Vault: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Erro na Indexação do Obsidian: " + err.Error(),
			})
			return
		}

		// 2. Indexar a documentação do próprio sistema (Lumaestro Core)
		err = a.crawler.IndexSystemDocs(a.ctx, "./")
		if err != nil {
			fmt.Printf("[BACKEND] Aviso: Erro ao indexar docs do sistema: %v\n", err)
		}

		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"source":  "CRAWLER",
			"content": "🏛️ Sincronização semântica completa (Vault + Sistema)!",
		})
	}()

	return "Indexação iniciada em segundo plano. Você pode continuar usando o Maestro normalmente."
}

// CheckConnection verifica se o Qdrant está acessível
func (a *App) CheckConnection() bool {
	res := a.qdrant != nil && a.qdrant.BaseURL != ""
	fmt.Printf("[BACKEND-UI] CheckConnection disparado pelo frontend. Retornando: %v\n", res)
	return res
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
	return a.executor.StartSession(a.ctx, agent, sessionID, "LATEST")
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
	return a.executor.StartSession(a.ctx, agent, sessionID, acpSessionID)
}

// NewAgentSession força a criação de um novo chat (limpa o contexto)
func (a *App) NewAgentSession(agent string) error {
	fmt.Println("[App] Iniciando NOVO chat (limpando contexto)...")
	sessionID := "acp-session-" + agent
	return a.executor.StartSession(a.ctx, agent, sessionID, "")
}

func (a *App) SendAgentInput(agent string, input string, images []map[string]string) error {
	fmt.Printf("[App] SendAgentInput INVOCADO pela UI: agent=%s, input=%s, imagens=%d\n", agent, input, len(images))

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
	
	fmt.Println("[App] Input enviado com sucesso ao canal RPC!")
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
			"status": "legacy",
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
	fmt.Printf("[App] Lendo documentação: %s\n", name)
	path := filepath.Join(".", "docs", name)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("erro ao ler %s: %v", name, err)
	}

	return string(content), nil
}

// AnalyzeGraphHealth analisa a integridade semântica do grafo.
func (a *App) AnalyzeGraphHealth() (map[string]interface{}, error) {
	fmt.Println("[Audit] Analisando saúde do Grafo de Contexto...")
	
	// Busca pontos ativos no Qdrant (Simulação de Analytic do TrustGraph)
	// Para um sistema real, faríamos um Scroll filtrando por status: active
	// Aqui retornamos estatísticas baseadas na densidade atual
	stats := map[string]interface{}{
		"density": 0.85, // Exemplo: de cada 100 notas, 85 estão conectadas
		"conflicts": 0,
		"active_nodes": 0,
	}

	return stats, nil
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
