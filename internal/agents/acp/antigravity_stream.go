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

// AntigravityToolCall define uma invocação de ferramenta no protocolo Antigravity.
type AntigravityToolCall struct {
	Name       string                 `json:"name"`
	Args       map[string]interface{} `json:"args,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
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
	ToolCalls       []AntigravityToolCall  `json:"tool_calls,omitempty"`
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

		s.UpdateActivity()

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
				utils.SafeEmit(e.Ctx, "terminal:started", map[string]string{
					"agent": s.AgentName,
				})
				utils.SafeEmit(e.Ctx, "agent:status", map[string]string{
					"agent":  s.AgentName,
					"action": fmt.Sprintf("Motor Antigravity pronto (%d ferramentas ativas)", numTools),
					"kind":   "ready",
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

				// Push para canal sincrono (AskSync) - não-bloqueante para evitar deadlock em respostas longas
				e.turnMu.Lock()
				if ch, ok := e.turnChannels[s.ID]; ok {
					select {
					case ch <- step.TextDelta:
					default:
					}
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
				toolName, action, targetFile := extractToolDetails(step, e.Workspace)

				// 🛡️ DEDUPLICAÇÃO INTELIGENTE:
				// O Antigravity emite múltiplos eventos para cada ferramenta: ACTIVE (ao iniciar) e DONE (ao concluir).
				// Se já reportamos este mesmo stepIndex com esta ferramenta em ACTIVE, evitamos re-emitir no DONE para não poluir o terminal.
				if step.State == "DONE" && s.lastToolStepIdx == step.StepIndex && s.lastToolName == toolName {
					continue
				}

				s.lastToolStepIdx = step.StepIndex
				s.lastToolName = toolName

				if e.Ctx != nil {
					statusPayload := map[string]string{
						"agent":  s.AgentName,
						"tool":   toolName,
						"action": action,
						"kind":   "tool",
						"state":  step.State,
					}
					if targetFile != "" {
						statusPayload["file"] = targetFile
					}
					utils.SafeEmit(e.Ctx, "agent:status", statusPayload)
				}
			}

		case "result":
			s.lastToolStepIdx = -1
			s.lastToolName = ""
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
						select {
						case ch <- res.Response:
						default:
						}
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

// extractToolDetails analisa detalhadamente o passo de ferramenta e extrai o nome, a ação amigável e o arquivo alvo.
func extractToolDetails(step *AntigravityStepPayload, workspace string) (toolName string, action string, targetFile string) {
	toolName = step.ToolName
	if toolName == "" && step.ToolInfo != nil {
		if name, ok := step.ToolInfo["name"].(string); ok {
			toolName = name
		}
	}
	if toolName == "" && len(step.ToolCalls) > 0 {
		toolName = step.ToolCalls[0].Name
	}
	if toolName == "" {
		toolName = "ferramenta"
	}

	// Coleta todos os parâmetros possíveis (suportando tool_info.parameters, tool_info.args, tool_info raiz e tool_calls)
	paramSources := make([]map[string]interface{}, 0, 4)
	if step.ToolInfo != nil {
		if p, ok := step.ToolInfo["parameters"].(map[string]interface{}); ok && p != nil {
			paramSources = append(paramSources, p)
		}
		if a, ok := step.ToolInfo["args"].(map[string]interface{}); ok && a != nil {
			paramSources = append(paramSources, a)
		}
		paramSources = append(paramSources, step.ToolInfo)
	}
	for _, tc := range step.ToolCalls {
		if tc.Parameters != nil {
			paramSources = append(paramSources, tc.Parameters)
		}
		if tc.Args != nil {
			paramSources = append(paramSources, tc.Args)
		}
	}

	cleanStr := func(v interface{}) string {
		if v == nil {
			return ""
		}
		s, ok := v.(string)
		if !ok {
			s = fmt.Sprintf("%v", v)
		}
		s = strings.TrimSpace(s)
		s = strings.Trim(s, "\"")
		s = strings.TrimSpace(s)
		return s
	}

	getParam := func(keys ...string) string {
		for _, ps := range paramSources {
			for _, key := range keys {
				// Match exato
				if val, ok := ps[key]; ok && val != nil {
					if res := cleanStr(val); res != "" {
						return res
					}
				}
				// Match case-insensitive
				for k, v := range ps {
					if strings.EqualFold(k, key) && v != nil {
						if res := cleanStr(v); res != "" {
							return res
						}
					}
				}
			}
		}
		return ""
	}

	formatPath := func(rawPath string) string {
		p := cleanStr(rawPath)
		if p == "" {
			return ""
		}
		// Normaliza separadores de diretório
		p = filepath.ToSlash(filepath.Clean(p))
		if workspace != "" && workspace != "." {
			wsClean := filepath.ToSlash(filepath.Clean(workspace))
			// Se o caminho iniciar com o workspace, torna-o relativo e limpo
			if strings.HasPrefix(strings.ToLower(p), strings.ToLower(wsClean)) {
				rel := strings.TrimPrefix(p, wsClean)
				rel = strings.TrimPrefix(rel, "/")
				if rel != "" {
					return rel
				}
			}
		}
		return p
	}

	// Normaliza nome da ferramenta
	normTool := strings.ToLower(toolName)

	switch {
	case strings.Contains(normTool, "view_file") || strings.Contains(normTool, "read_file") || normTool == "readfile" || normTool == "read_text_file":
		rawPath := getParam("AbsolutePath", "TargetFile", "path", "file", "filePath", "Target")
		if rawPath != "" {
			targetFile = formatPath(rawPath)
			action = fmt.Sprintf("📖 Lendo: %s", targetFile)
		}

	case strings.Contains(normTool, "replace_file_content") || strings.Contains(normTool, "multi_replace") || strings.Contains(normTool, "sed_file"):
		rawPath := getParam("TargetFile", "AbsolutePath", "path", "file", "filePath", "Target")
		if rawPath != "" {
			targetFile = formatPath(rawPath)
			action = fmt.Sprintf("✏️ Editando: %s", targetFile)
		}

	case strings.Contains(normTool, "write_to_file") || strings.Contains(normTool, "write_file") || normTool == "writefile":
		rawPath := getParam("TargetFile", "AbsolutePath", "path", "file", "filePath", "Target")
		if rawPath != "" {
			targetFile = formatPath(rawPath)
			action = fmt.Sprintf("📝 Salvando: %s", targetFile)
		}

	case strings.Contains(normTool, "delete_file") || normTool == "remove" || normTool == "deletefile":
		rawPath := getParam("path", "file", "TargetFile", "AbsolutePath")
		if rawPath != "" {
			targetFile = formatPath(rawPath)
			action = fmt.Sprintf("🗑️ Deletando: %s", targetFile)
		}

	case strings.Contains(normTool, "grep_search"):
		q := getParam("Query", "query", "pattern", "regex")
		searchPath := getParam("SearchPath", "searchPath", "path", "dir")
		if searchPath != "" {
			searchPath = formatPath(searchPath)
		}
		if q != "" {
			if len(q) > 30 {
				q = q[:27] + "..."
			}
			if searchPath != "" {
				action = fmt.Sprintf("🔍 Buscando \"%s\" em %s", q, searchPath)
			} else {
				action = fmt.Sprintf("🔍 Buscando \"%s\"", q)
			}
		}

	case strings.Contains(normTool, "find_by_name"):
		pattern := getParam("Pattern", "pattern", "glob", "Query")
		searchDir := getParam("SearchDirectory", "searchDirectory", "dir", "path")
		if searchDir != "" {
			searchDir = formatPath(searchDir)
		}
		if pattern != "" {
			if searchDir != "" {
				action = fmt.Sprintf("🔎 Procurando \"%s\" em %s", pattern, searchDir)
			} else {
				action = fmt.Sprintf("🔎 Procurando arquivo \"%s\"", pattern)
			}
		}

	case strings.Contains(normTool, "list_dir") || strings.Contains(normTool, "list_directory"):
		dir := getParam("DirectoryPath", "directoryPath", "path", "dir")
		if dir != "" {
			dir = formatPath(dir)
			action = fmt.Sprintf("📁 Listando pasta: %s", dir)
		} else {
			action = "📁 Listando arquivos do projeto"
		}

	case strings.Contains(normTool, "run_command") || strings.Contains(normTool, "execute_command") || strings.Contains(normTool, "run_shell_command"):
		cmd := getParam("CommandLine", "commandLine", "command", "cmd")
		if cmd != "" {
			if len(cmd) > 40 {
				cmd = cmd[:37] + "..."
			}
			action = fmt.Sprintf("💻 Executando: %s", cmd)
		}

	case strings.Contains(normTool, "search_web") || strings.Contains(normTool, "web_search"):
		q := getParam("query", "Query", "q")
		if q != "" {
			if len(q) > 35 {
				q = q[:32] + "..."
			}
			action = fmt.Sprintf("🌐 Pesquisando web: \"%s\"", q)
		}

	case strings.Contains(normTool, "read_url_content") || strings.Contains(normTool, "read_browser_page") || strings.Contains(normTool, "open_browser_url"):
		url := getParam("Url", "url", "URL")
		if url != "" {
			if len(url) > 40 {
				url = url[:37] + "..."
			}
			action = fmt.Sprintf("🌐 Acessando: %s", url)
		}

	case strings.Contains(normTool, "manage_subagents") || strings.Contains(normTool, "invoke_subagent"):
		subRole := getParam("Role", "TypeName", "Action")
		if subRole != "" {
			action = fmt.Sprintf("🤖 Subagente: %s", subRole)
		}
	}

	// Fallback inteligente
	if action == "" {
		if act := getParam("toolAction", "tool_action"); act != "" {
			action = act
		} else if sum := getParam("toolSummary", "tool_summary"); sum != "" {
			action = sum
		} else if desc := getParam("description", "Description"); desc != "" {
			action = desc
		} else {
			action = fmt.Sprintf("Executando: %s", toolName)
		}
	}

	return toolName, action, targetFile
}

