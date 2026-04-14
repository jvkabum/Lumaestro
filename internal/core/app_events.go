package core

import (
	"encoding/base64"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// emitBoot envia um evento de diagnóstico de boot para o frontend. (DNA 1:1)
func (a *App) emitBoot(stage string, icon string, message string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "boot:stage", map[string]string{
		"stage": stage, "icon": icon, "message": message,
	})
}

// listenForLogs ouve o Executor ACP (Logs da IA no formato JSON-RPC). (DNA 1:1)
func (a *App) listenForLogs() {
	for log := range a.executor.LogChan {
		runtime.EventsEmit(a.ctx, "agent:log", log)
	}
}

// listenForInstallerLogs ouve o Instalador (Logs do Terminal/NPM/PS). (DNA 1:1)
func (a *App) listenForInstallerLogs() {
	for log := range a.installer.LogChan {
		runtime.EventsEmit(a.ctx, "installer:log", log)
	}
}

// listenForTerminalOutput (Descontinuado para ACP, mantido para compatibilidade). (DNA 1:1)
func (a *App) listenForTerminalOutput() {
	for td := range a.executor.TerminalOutput {
		if td.Data == nil {
			runtime.EventsEmit(a.ctx, "terminal:closed", td.Agent)
			continue
		}
		encoded := base64.StdEncoding.EncodeToString(td.Data)
		runtime.EventsEmit(a.ctx, "terminal:output", map[string]string{
			"agent": td.Agent, "data": encoded,
		})
	}
}
