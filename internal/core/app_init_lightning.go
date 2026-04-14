package core

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/lightning"
)

// initLightningAnalytics inicializa o motor analítico DuckDB, refletor de vault e roteador de LLMs.
func (a *App) initLightningAnalytics(cfg *config.Config) {
	if cfg.LightningEnabled && a.LStore != nil {
		a.emitBoot("lightning", "⚡", "Iniciando cérebro analítico DuckDB...")
		a.LReflector = lightning.NewReflector(a.LStore, cfg.ObsidianVaultPath)
		a.LOptimizer = lightning.NewOptimizer(a.LStore, a.executor.RewardEngine)
		a.LRouter = lightning.NewLLMRouter()
		if cfg.BlendActiveModels {
			a.LRouter.Providers = cfg.GetActiveProviders()
		}
	}
}
