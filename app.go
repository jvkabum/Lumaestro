package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
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
	executor  *agents.ACPExecutor
	legacyExec *agents.Executor // Apenas para ExecuteCLI fallback se necessário, ou podemos migrar.
	ontology  *provider.OntologyService
	crawler   *obsidian.Crawler
	qdrant    *provider.QdrantClient
	embedder  *provider.EmbeddingService
	chat      *rag.ChatService
	installer *tools.Installer
	config    *config.Config
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.executor = agents.NewACPExecutor()
	a.legacyExec = agents.NewExecutor() // Mantemos temporariamente para métodos legacy
	a.installer = tools.NewInstaller()

	// Sincroniza o PATH imediatamente (Garante que claude/gemini sejam encontrados)
	a.installer.SyncPath()

	// Tenta inicializar os serviços logo na subida
	a.initServices()

	// Iniciar a Escuta de Logs e Terminal
	go a.listenForLogs()
	go a.listenForInstallerLogs()
	go a.listenForTerminalOutput()

	// 🚀 Auto-Start: Inicia os agentes favoritos automaticamente no boot
	if a.config != nil && a.config.GeminiAPIKey != "" {
		go func() {
			time.Sleep(2000 * time.Millisecond)
			fmt.Println("[BOOT] Gemini API Key detectada. Restaurando última Sinfonia...")
			// Se não passarmos nada, o StartAgentSession vai tentar carregar a última
			a.StartAgentSession("gemini")
		}()
	}
}

// initServices inicializa os motores de IA e RAG se as configurações permitirem
func (a *App) initServices() error {
	cfg, _ := config.Load()
	if cfg == nil || cfg.GeminiAPIKey == "" {
		return fmt.Errorf("configuração incompleta (API Key ausente)")
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
	nav := rag.NewGraphNavigator(a.qdrant)
	
	a.chat = rag.NewChatService(a.legacyExec, search, nav, a.embedder, a.installer)
	a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology)

	return nil
}

// listenForLogs ouve o Executor ACP (Logs da IA no formato JSON-RPC via STDOUT)
func (a *App) listenForLogs() {
	for log := range a.executor.LogChan {
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
		response, err := a.chat.Ask(a.ctx, agentName, prompt)
		if err != nil {
			fmt.Printf("[BACKEND] ERRO no Chat: %v\n", err)
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha na Sinfonia: " + err.Error(),
			})
			return
		}

		fmt.Printf("[BACKEND] Resposta da IA recebida (%d chars). Emitindo evento...\n", len(response))
		runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
			"role":    "assistant",
			"agent":   agentName,
			"content": response,
		})
	}()

	return "Orquestrando..."
}

// ScanVault percorre o Obsidian e indexa no Qdrant com Embeddings
func (a *App) ScanVault() string {
	err := a.crawler.IndexVault(a.ctx)
	if err != nil {
		return "Erro na Indexação: " + err.Error()
	}

	runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
		"source":  "CRAWLER",
		"content": "Indexação semântica concluída com sucesso!",
	})

	return "Pronto! Seu conhecimento agora é vetorial."
}

// CheckConnection verifica se o Qdrant está acessível
func (a *App) CheckConnection() bool {
	return a.qdrant != nil && a.qdrant.BaseURL != ""
}

// GetToolsStatus verifica se as IAs CLIs estão instaladas no PATH e os status de autenticação
func (a *App) GetToolsStatus() map[string]bool {
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

// SendAgentInput via prompt RPC na sessão ACP.
func (a *App) SendAgentInput(agent string, input string) error {
	fmt.Printf("[App] SendAgentInput INVOCADO pela UI: agent=%s, input=%s\n", agent, input)
	
	sessionID := "acp-session-" + agent
	err := a.executor.SendInput(sessionID, input)
	if err != nil {
		fmt.Printf("[App] ERRO no SendAgentInput: %v\n", err)
		return fmt.Errorf("erro ao enviar input para ACP: %v", err)
	}
	
	fmt.Println("[App] Input enviado com sucesso ao canal RPC!")
	return nil
}

// SendTerminalData está descontinuado fisicamente no ACP, retorna erro.
func (a *App) SendTerminalData(agent string, base64Data string) string {
	return "Não suportado em modo ACP"
}

// ResizeTerminal não faz mais sentido visual no ACP. Ignoramos graciosamente.
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
