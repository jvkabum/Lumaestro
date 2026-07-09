package core

import (
	"Lumaestro/internal/agents"
	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/config"
	"Lumaestro/internal/lightning"
	"Lumaestro/internal/obsidian"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/rag"
	"Lumaestro/internal/rag/neural"
	"Lumaestro/internal/tools"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// ============================================================
// 🎖️ LUMAESTRO COGNITIVE ENGINE V25 - CORE (HUB CENTRAL)
// ============================================================

// App struct representa a instância soberana do Maestro
type App struct {
	ctx          context.Context
	NLPReady     bool // 🛡️ Sinalizador de motores prontos para indexação
	IsScanning   bool // 🔍 Trava para evitar scans simultâneos
	isBooted     bool // ✅ Travão de segurança contra loops de boot
	
	executor     *acp.ACPExecutor
	orchestrator *acp.Orchestrator
	legacyExec   *agents.Executor // Executor CLI veterano
	ontology     *provider.OntologyService
	crawler      *obsidian.Crawler
	qdrant       *provider.QdrantClient
	embedder     provider.Embedder
	chat         *rag.ChatService
	weaver       *rag.KnowledgeWeaver
	navigator    *rag.GraphNavigator
	ranker       *neural.Ranker
	installer    *tools.Installer
	config       *config.Config
	muInit       sync.Mutex // 🔒 Trava de Segurança contra inicialização dupla (HMR/Wails)

	// ⚡ Motores de Elite (Lightning)
	LStore     *lightning.DuckDBStore
	LReflector *lightning.Reflector
	LOptimizer *lightning.Optimizer
	LRouter    *lightning.LLMRouter

	// 🧠 Cérebro Relacional (V20, V22, V23)
	GEngine   *rag.GraphEngine
	Validator *rag.AgentValidator
	Recon     *rag.AgentRecon

	// 🤖 LM Studio (Motor Local)
	lmStudio *provider.LMStudioClient

	// 🧠 Motor Nativo (Interno)
	nativeEmbedder   *provider.NativeEmbedder
	nativeExtraction *provider.NativeGenerator // Qwen Reasoning (Port 8086)
	nativeGenerator  *provider.NativeGenerator // Gemma Chat (Port 8087)
}

// BindLightning vincula o motor analítico e de recompensas após a instância original para uso seguro multi-pacote.
func (a *App) BindLightning(lStore *lightning.DuckDBStore) {
	a.LStore = lStore
	if a.executor != nil {
		a.executor.LStore = lStore
		a.executor.RewardEngine = lightning.NewRewardEngine(lStore)
	}
}

// GetLastSessionID recupera o ID da última sessão ativa.
func (a *App) GetLastSessionID() (string, error) {
	// Garante que o Workspace está acessível a partir do executor
	workspace := a.executor.Workspace
	if workspace == "" && a.config != nil {
		workspace = a.config.ActiveWorkspace
	}

	lastSessionPath := filepath.Join(workspace, ".lumaestro", "last_session.json")
	data, err := os.ReadFile(lastSessionPath)
	if err != nil {
		return "", fmt.Errorf("nenhuma sessão anterior encontrada: %v", err)
	}

	var lastSession struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(data, &lastSession); err != nil {
		return "", fmt.Errorf("erro ao ler arquivo de sessão: %v", err)
	}

	return lastSession.SessionID, nil
}

// NewApp creates a new App application struct
func NewApp() *App {
	exec := acp.NewACPExecutor("", "")
	return &App{
		installer:    tools.NewInstaller(),
		executor:     exec,
		legacyExec:   agents.NewExecutor(),
		GEngine:      rag.NewGraphEngine(),
		orchestrator: acp.NewOrchestrator(exec),
	}
}
