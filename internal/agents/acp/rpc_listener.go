package acp

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// runRPCListener inicia a escuta ndJSON ligando o pipe de stdout ao handler.
func (e *ACPExecutor) runRPCListener(s *ACPSession, stdout io.Reader) {
	handler := &ACPRpcHandler{Executor: e, Session: s}
	StartJSONRPCListener(stdout, handler)
}

// runStderrMonitor vigia o pipe de erro em busca de avisos da CLI ou de autenticação.
func (e *ACPExecutor) runStderrMonitor(s *ACPSession, stderr io.Reader) {
	reader := bufio.NewReader(stderr)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		cleanLine := strings.TrimSpace(line)
		if cleanLine == "" {
			continue
		}

		// Filtragem inteligente: Reportar apenas o que importa para o usuário no Chat
		lv := strings.ToLower(cleanLine)
		isRelevant := strings.Contains(lv, "login") ||
			strings.Contains(lv, "auth") ||
			strings.Contains(lv, "error") ||
			strings.Contains(lv, "warning") ||
			strings.Contains(lv, "denied") ||
			strings.Contains(lv, "not found")

		if isRelevant {
			e.LogChan <- ExecutionLog{
				Source:  "CLI/AVISO",
				Content: cleanLine,
			}
		}

		// Gatilhos específicos de login
		if strings.Contains(cleanLine, "Login required") {
			runtime.EventsEmit(e.Ctx, "agent:login_required", s.AgentName)
		}

		// Log interno para depuração
		fmt.Printf("[%s/stderr] %s\n", s.AgentName, cleanLine)
	}
}
