package main

import (
	"context"
	"encoding/base64"
	"fmt"
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
	executor  *agents.Executor
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
	a.executor = agents.NewExecutor()
	a.installer = tools.NewInstaller()

	// Sincroniza o PATH imediatamente (Garante que claude/gemini sejam encontrados)
	a.installer.SyncPath()

	// Tenta inicializar os serviços logo na subida
	a.initServices()

	// Iniciar a Escuta de Logs e Terminal
	go a.listenForLogs()
	go a.listenForInstallerLogs()
	go a.listenForTerminalOutput()

	// 🚀 Auto-Start: Inicia o agente favorito automaticamente no boot se configurado
	if a.config != nil && a.config.ActiveAgent != "" {
		go func() {
			time.Sleep(1500 * time.Millisecond) // Buffer p/ frontend carregar xterm.js
			a.StartAgentSession(a.config.ActiveAgent)
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
	
	a.chat = rag.NewChatService(a.executor, search, nav, a.embedder, a.installer)
	a.crawler = obsidian.NewCrawler(cfg.ObsidianVaultPath, a.embedder, a.qdrant, a.ontology)

	return nil
}

// listenForLogs ouve o Executor (Logs da IA)
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

// listenForTerminalOutput ouve os bytes brutos do ConPTY e envia para o xterm.js.
// Os dados são encodados em base64 para transporte seguro via Wails Events.
func (a *App) listenForTerminalOutput() {
	for data := range a.executor.TerminalOutput {
		if data == nil {
			// Sessão encerrada — notifica o frontend
			runtime.EventsEmit(a.ctx, "terminal:closed", true)
			continue
		}
		// Encoda em base64 para preservar bytes brutos (sequências ANSI, etc.)
		encoded := base64.StdEncoding.EncodeToString(data)
		runtime.EventsEmit(a.ctx, "terminal:output", encoded)
	}
}

// AskAgent processa a pergunta em segundo plano para permitir Streaming Real
func (a *App) AskAgent(prompt string) string {
	// Garante que os serviços estejam prontos
	if a.chat == nil {
		if err := a.initServices(); err != nil {
			return "⚠️ O motor do Maestro está desligado. Por favor, verifique sua Gemini API Key nas configurações."
		}
	}

	cfg, _ := config.Load()
	agentName := "gemini"
	if cfg != nil && cfg.ActiveAgent != "" {
		agentName = cfg.ActiveAgent
	}

	// Executa em uma goroutine para não travar o frontend
	// O retorno real virá linha a linha via Evento "agent:log"
	go func() {
		_, err := a.chat.Ask(a.ctx, agentName, prompt)
		if err != nil {
			runtime.EventsEmit(a.ctx, "agent:log", map[string]string{
				"source":  "ERROR",
				"content": "❌ Falha na Sinfonia: " + err.Error(),
			})
		}
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

// SetupTool abre um terminal para configuração interativa
func (a *App) SetupTool(name string) string {
	err := a.installer.SetupTool(name)
	if err != nil {
		return "Erro ao abrir terminal: " + err.Error()
	}
	return "Janela de configuração aberta!"
}

// ============================================================
// TERMINAL REAL — ConPTY + xterm.js Bidirectional
// ============================================================

// StartAgentSession inicia uma sessão interativa (Terminal Mode com ConPTY real).
// O frontend deve escutar o evento "terminal:output" para receber bytes brutos.
func (a *App) StartAgentSession(agent string) string {
	sessionID := "maestro-session"
	
	// Limpeza preventiva de qualquer sessão anterior travada
	_ = a.executor.StopSession(sessionID)

	err := a.executor.StartSession(a.ctx, agent, sessionID)
	if err != nil {
		return "Erro ao iniciar sessão: " + err.Error()
	}

	// Verifica se conseguiu ConPTY real ou caiu no fallback
	isReal := a.executor.IsTerminalSession(sessionID)
	mode := "ONE-SHOT PROXY"
	if isReal {
		mode = "CONPTY REAL"
	}

	runtime.EventsEmit(a.ctx, "terminal:started", map[string]interface{}{
		"agent":      agent,
		"mode":       mode,
		"isRealPTY":  isReal,
	})

	return fmt.Sprintf("Sessão %s iniciada [%s]", agent, mode)
}

// SendAgentInput envia texto para o agente ativo.
// No modo ConPTY, os bytes são escritos direto no PTY (como teclado real).
func (a *App) SendAgentInput(input string) string {
	sessionID := "maestro-session"
	err := a.executor.SendInput(sessionID, input)
	if err != nil {
		return "Erro ao enviar input: " + err.Error()
	}
	return "OK"
}

// SendTerminalData envia bytes brutos (base64) para o PTY.
// Usado pelo xterm.js onData — cada tecla é enviada individualmente.
func (a *App) SendTerminalData(base64Data string) string {
	sessionID := "maestro-session"

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "Erro ao decodificar: " + err.Error()
	}

	err = a.executor.SendRawInput(sessionID, data)
	if err != nil {
		return "Erro: " + err.Error()
	}
	return "OK"
}

// ResizeTerminal informa ao ConPTY as novas dimensões do xterm.js.
func (a *App) ResizeTerminal(cols int, rows int) {
	sessionID := "maestro-session"
	a.executor.ResizePTY(sessionID, cols, rows)
}

// StopAgentSession encerra a sessão interativa atual.
func (a *App) StopAgentSession() string {
	sessionID := "maestro-session"
	err := a.executor.StopSession(sessionID)
	if err != nil {
		return "Nenhuma sessão ativa encontrada ou erro: " + err.Error()
	}

	runtime.EventsEmit(a.ctx, "terminal:closed", true)
	return "Sessão encerrada com sucesso."
}
