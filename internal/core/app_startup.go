package core

import (
	"Lumaestro/internal/db"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Startup é o gatilho inicial quando o sistema decola.
func (a *App) Startup(ctx context.Context) {
	// 🛡️ Detector de arquivos Go órfãos que quebram o Wails silenciosamente
	checkRogueMainFiles()

	// 📋 Iniciar o Banco de Dados Paperclip (Orquestração Corporativa)
	if err := db.InitDB(); err != nil {
		fmt.Printf("🔴 PANICO SILENCIOSO do Backend no db.InitDB: %v\n", err)
	}

	a.ctx = ctx
	
	// Sincroniza o PATH em background para não bloquear o handshake do Wails (runtime:ready)
	go a.installer.SyncPath()

	// Iniciar a Escuta de Logs e Terminal (não depende dos motores)
	go a.listenForLogs()
	go a.listenForInstallerLogs()
	go a.listenForTerminalOutput()

	// 🚀 Boot Assíncrono: Garante que o WebView esteja pronto antes de emitir eventos
	go a.bootSequence()

	// 🧠 Córtex Autônomo (APO): Monitora falhas e otimiza prompts em background
	// Função existe no app_agents.go
	go a.startAPOWorker()
}

// Shutdown é acionado quando o Lumaestro é fechado.
func (a *App) Shutdown(ctx context.Context) {
	if a.nativeEmbedder != nil {
		fmt.Println("🛑 Encerrando motor nativo interno (embeddings)...")
		a.nativeEmbedder.Stop()
	}
	if a.nativeExtraction != nil {
		a.nativeExtraction.Stop()
	}
	if a.nativeGenerator != nil {
		a.nativeGenerator.Stop()
	}
}

// injectContexts garante que todos os motores de RAG tenham o contexto oficial.
func (a *App) injectContexts() {
	// 📂 Garante que os diretórios de infraestrutura existam
	os.MkdirAll(".context", 0755)
	os.MkdirAll(".lumaestro", 0755)

	// 💎 Inicialização Atômica do Cache de Topologia (se não existir)
	topologyPath := filepath.Join(".lumaestro", "cache", "topology.json")
	if _, err := os.Stat(topologyPath); os.IsNotExist(err) {
		fmt.Println("[Init] 🛠️ Criando arquivo de topologia base em .lumaestro/...")
		baseCache := `{"nodes":[], "edges":[]}`
		os.WriteFile(topologyPath, []byte(baseCache), 0644)
	}

	if a.ctx == nil {
		return
	}
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

// CheckConnection verifica se os sistemas de suporte vitais estão online.
func (a *App) CheckConnection() bool {
	return a.config != nil
}

// DeleteSession remove o arquivo físico de uma Sinfonia (Sessão).
func (a *App) DeleteSession(filePath string) error {
	if a.executor == nil {
		return fmt.Errorf("executor de agentes não inicializado")
	}
	return a.executor.DeleteSession(filePath)
}
