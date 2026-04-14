package core

import (
	"encoding/base64"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// emitEvent é a central de rádio blindada do Lumaestro.
// Valida se o contexto Wails está ativo antes de qualquer emissão assíncrona.
func (a *App) emitEvent(name string, data interface{}) {
	if a.ctx == nil {
		return
	}

	// 🛡️ Verificação de Vida do Contexto
	select {
	case <-a.ctx.Done():
		// Contexto invalidado ou app fechando: abortar missão.
		return
	default:
		// Contexto saudável: liberar transmissão.
		runtime.EventsEmit(a.ctx, name, data)
	}
}

// emitBoot envia um evento de diagnóstico de boot para o frontend.
func (a *App) emitBoot(stage string, icon string, message string) {
	a.emitEvent("boot:stage", map[string]string{
		"stage": stage, "icon": icon, "message": message,
	})
}

// listenForLogs ouve o Executor ACP (Logs da IA no formato JSON-RPC).
func (a *App) listenForLogs() {
	for log := range a.executor.LogChan {
		a.emitEvent("agent:log", log)
	}
}

// listenForInstallerLogs ouve o Instalador (Logs do Terminal/NPM/PS).
func (a *App) listenForInstallerLogs() {
	for log := range a.installer.LogChan {
		a.emitEvent("installer:log", log)
	}
}

// listenForTerminalOutput (Descontinuado para ACP, mantido para compatibilidade).
func (a *App) listenForTerminalOutput() {
	for td := range a.executor.TerminalOutput {
		if td.Data == nil {
			a.emitEvent("terminal:closed", td.Agent)
			continue
		}
		encoded := base64.StdEncoding.EncodeToString(td.Data)
		a.emitEvent("terminal:output", map[string]string{
			"agent": td.Agent, "data": encoded,
		})
	}
}
