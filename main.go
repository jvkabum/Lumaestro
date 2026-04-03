package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	
	"Lumaestro/internal/db"
	"Lumaestro/internal/lightning"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 🧠 1. Inicializa Conectividade de Dados (SQLite)
	if err := db.InitDB(); err != nil {
		println("Falha fatal ao abrir bancos: ", err.Error())
		return
	}

	// 📊 2. Inicializa Motor de Elite (Lightning / DuckDB)
	lStore, err := lightning.NewDuckDBStore(".lumaestro/analytics.db")
	if err != nil {
		println("Aviso: Motor analítico não disponível: ", err.Error())
	} else {
		db.AnalyticsDB = lStore // Vincula ao DB global
	}

	// Create an instance of the app structure
	app := NewApp()
	app.LStore = lStore // Injeta o store analítico no App
	app.executor.LStore = lStore // ⚡ Injeção no Executor
	app.executor.RewardEngine = lightning.NewRewardEngine(lStore) // 🧬 Injeção de Recompensas
	
	// Ativa o Proxy se habilitado nas configurações (Poderia ser condicional aqui)
	lProxy := lightning.NewProxyServer(lStore, "8001")
	lProxy.Start()
	defer lProxy.Stop()

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "Lumaestro",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
