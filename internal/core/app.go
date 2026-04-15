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
	"sync"
)

// ============================================================
// 🎖️ LUMAESTRO COGNITIVE ENGINE V25 - CORE (HUB CENTRAL)
// ============================================================

// App struct representa a instância soberana do Maestro
type App struct {
	ctx          context.Context
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

	isBooted bool // ✅ Travão de segurança contra loops de boot
}

// NewApp cria uma nova instância soberana do Lumaestro.
func NewApp() *App {
	a := &App{}
	a.executor = acp.NewACPExecutor()
	a.orchestrator = acp.NewOrchestrator(a.executor)
	a.legacyExec = agents.NewExecutor()
	a.installer = tools.NewInstaller()

	// 🧠 Motores Vitais (Sempre vivos para evitar Nil Panics)
	a.GEngine = rag.NewGraphEngine()
	a.ranker = neural.NewRanker()
	a.ranker.Decay() // Inicializa o estado neural

	return a
}

// BindLightning vincula o motor analítico e de recompensas após a instância original para uso seguro multi-pacote.
func (a *App) BindLightning(lStore *lightning.DuckDBStore) {
	a.LStore = lStore
	if a.executor != nil {
		a.executor.LStore = lStore
		a.executor.RewardEngine = lightning.NewRewardEngine(lStore)
	}
}
