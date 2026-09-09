package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"Lumaestro/internal/utils"
)

// AntigravityEvent define o envelope do protocolo stream-json do Antigravity CLI (agy.exe).
type AntigravityEvent struct {
	Event          string                    `json:"event"`
	ConversationID string                    `json:"conversation_id,omitempty"`
	Init           *AntigravityInitPayload   `json:"init,omitempty"`
	StepUpdate     *AntigravityStepPayload   `json:"step_update,omitempty"`
	Result         *AntigravityResultPayload `json:"result,omitempty"`
}

// AntigravityInitPayload traz os detalhes de inicializacao da sessao.
type AntigravityInitPayload struct {
	Cwd            string   `json:"cwd"`
	Tools          []string `json:"tools"`
	PermissionMode string   `json:"permission_mode"`
}

// AntigravityUsage armazena as estatisticas de uso de tokens do turno.
type AntigravityUsage struct {
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
	ThinkingTokens  int `json:"thinking_tokens"`
	CacheReadTokens int `json:"cache_read_tokens"`
	TotalTokens     int `json:"total_tokens"`
}

// AntigravityStepPayload contem atualizacoes parciais de passos e streaming de texto.
type AntigravityStepPayload struct {
	ConversationID  string                 `json:"conversation_id"`
	StepIndex       int                    `json:"step_index"`
	State           string                 `json:"state"`
	StepType        string                 `json:"step_type"` // user_input, agent_response, tool, thought
	TextDelta       string                 `json:"text_delta,omitempty"`
	ToolName        string                 `json:"tool_name,omitempty"`
	ToolInfo        map[string]interface{} `json:"tool_info,omitempty"`
	DurationSeconds float64                `json:"duration_seconds,omitempty"`
	Usage           *AntigravityUsage      `json:"usage,omitempty"`
}

// AntigravityResultPayload e emitido no encerramento de cada turno.
type AntigravityResultPayload struct {
	ConversationID  string            `json:"conversation_id"`
	Status          string            `json:"status"` // SUCCESS ou ERROR
	Response        string            `json:"response"`
	DurationSeconds float64           `json:"duration_seconds"`
	NumTurns        int               `json:"num_turns"`
	Usage           *AntigravityUsage `json:"usage,omitempty"`
	Error           string            `json:"error,omitempty"`
}

// runAntigravityListener processa a saida NDJSON em tempo real do agy.exe (--output-format stream-json).
func (e *ACPExecutor) runAntigravityListener(s *ACPSession, stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	// Buffer grande de ate 10MB para absorver respostas ricas e payloads de ferramentas
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

	var turnDeltaReceived bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var evt AntigravityEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			fmt.Printf("[AGY RECV-RAW] %s\n", line)
			continue
		}

		switch evt.Event {
		case "init":
			if evt.ConversationID != "" {
				s.ACPSessID = evt.ConversationID
			}
			numTools := 0
			if evt.Init != nil {
				numTools = len(evt.Init.Tools)
			}
			fmt.Printf("[AGY] 🚀 Sessao Antigravity pronta! ID: %s | Ferramentas: %d\n", s.ACPSessID, numTools)

			// Libera sinal de inicializacao
			select {
			case s.initDone <- struct{}{}:
			default:
			}

			// 💾 Persiste a sinfonia atual no last_session.json do workspace e notifica a UI
			if s.ACPSessID != "" {
				ws := e.Workspace
				if ws == "" || ws == "." {
					ws, _ = os.Getwd()
				}
				lastSessionDir := filepath.Join(ws, ".lumaestro")
				_ = os.MkdirAll(lastSessionDir, 0755)
				_ = os.WriteFile(filepath.Join(lastSessionDir, "last_session.json"), []byte(fmt.Sprintf(`{"sessionId":"%s"}`, s.ACPSessID)), 0644)
			}

			if e.Ctx != nil {
				if s.ACPSessID != "" {
					utils.SafeEmit(e.Ctx, "sessions:current", s.ACPSessID)
					utils.SafeEmit(e.Ctx, "sessions:updated", nil)
				}
				utils.SafeEmit(e.Ctx, "agent:status", map[string]string{
					"agent":  s.AgentName,
					"action": fmt.Sprintf("Motor Antigravity pronto (%d ferramentas ativas)", numTools),
					"kind":   "status",
				})
			}

		case "step_update":
			if evt.StepUpdate == nil {
				continue
			}
			step := evt.StepUpdate

			// 📊 Atualizacao de Tokens em tempo real
			if step.Usage != nil {
				s.LastPromptTokens = step.Usage.InputTokens
				s.LastCandidatesTokens = step.Usage.OutputTokens
				s.LastCacheTokens = step.Usage.CacheReadTokens

				if e.Ctx != nil {
					utils.SafeEmit(e.Ctx, "agent:tokens", map[string]interface{}{
						"agent":        s.AgentName,
						"prompt":       step.Usage.InputTokens,
						"candidates":   step.Usage.OutputTokens,
						"cacheCurrent": step.Usage.CacheReadTokens,
						"cacheTotal":   s.TotalCacheTokens + step.Usage.CacheReadTokens,
					})

					statsInfo := fmt.Sprintf("%d in | %d out", step.Usage.InputTokens, step.Usage.OutputTokens)
					if step.Usage.CacheReadTokens > 0 {
						statsInfo = fmt.Sprintf("🧊 %d cache | %d in | %d out", step.Usage.CacheReadTokens, step.Usage.InputTokens, step.Usage.OutputTokens)
					}
					if step.Usage.ThinkingTokens > 0 {
						statsInfo += fmt.Sprintf(" (🧠 %d thinking)", step.Usage.ThinkingTokens)
					}
					utils.SafeEmit(e.Ctx, "agent:stats", map[string]string{
						"agent": s.AgentName,
						"info":  statsInfo,
					})
				}
			}

			// 📝 Streaming de resposta de texto do agente
			if step.StepType == "agent_response" && step.TextDelta != "" {
				turnDeltaReceived = true
				s.isLoggingMessage = true
				s.isLoggingThought = false

				// Streaming direto para a UI
				e.LogChan <- ExecutionLog{
					Source:  s.AgentName,
					Content: step.TextDelta,
					Type:    "message",
				}

				// Push para canal sincrono (AskSync)
				e.turnMu.Lock()
				if ch, ok := e.turnChannels[s.ID]; ok {
					ch <- step.TextDelta
				}
				e.turnMu.Unlock()
			} else if step.StepType == "thought" && step.TextDelta != "" {
				s.isLoggingThought = true
				s.isLoggingMessage = false

				e.LogChan <- ExecutionLog{
					Source:  s.AgentName,
					Content: step.TextDelta,
					Type:    "thought",
				}
			} else if step.StepType == "tool" {
				toolName := step.ToolName
				if toolName == "" && step.ToolInfo != nil {
					if name, ok := step.ToolInfo["name"].(string); ok {
						toolName = name
					}
				}
				action := fmt.Sprintf("Executando: %s", toolName)
				if step.ToolInfo != nil {
					if desc, ok := step.ToolInfo["description"].(string); ok && desc != "" {
						action = desc
					}
				}
				if e.Ctx != nil {
					utils.SafeEmit(e.Ctx, "agent:status", map[string]string{
						"agent":  s.AgentName,
						"tool":   toolName,
						"action": action,
						"kind":   "tool",
					})
				}
			}

		case "result":
			res := evt.Result
			if res != nil {
				if res.Usage != nil {
					s.LastPromptTokens = res.Usage.InputTokens
					s.LastCandidatesTokens = res.Usage.OutputTokens
					s.LastCacheTokens = res.Usage.CacheReadTokens
				}

				if res.Status == "ERROR" {
					errMsg := res.Error
					if errMsg == "" {
						errMsg = res.Response
					}
					e.LogChan <- ExecutionLog{
						Source:  "ERROR",
						Content: fmt.Sprintf("🔴 Erro no motor Antigravity: %s", errMsg),
						Type:    "error",
					}
				} else if !turnDeltaReceived && res.Response != "" {
					// Fallback: se nao houve streaming de deltas no turno, emite a resposta completa
					e.LogChan <- ExecutionLog{
						Source:  s.AgentName,
						Content: res.Response,
						Type:    "message",
					}

					e.turnMu.Lock()
					if ch, ok := e.turnChannels[s.ID]; ok {
						ch <- res.Response
					}
					e.turnMu.Unlock()
				}
			}

			// Reseta controle de streaming do turno
			turnDeltaReceived = false
			s.isLoggingThought = false
			s.isLoggingMessage = false

			// Registra custos no banco de dados analitico e telemetria
			s.ReportTurnCost(e)

			// Notifica o frontend que o turno encerrou
			if e.Ctx != nil {
				utils.SafeEmit(e.Ctx, "agent:turn_complete", s.AgentName)
			}

			// Fecha canais de sincronizacao (AskSync)
			e.turnMu.Lock()
			if ch, ok := e.turnChannels[s.ID]; ok {
				close(ch)
				delete(e.turnChannels, s.ID)
			}
			e.turnMu.Unlock()
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		fmt.Printf("[AGY] Erro lendo pipe stdout do Antigravity: %v\n", err)
	}
}
