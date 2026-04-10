package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"Lumaestro/internal/config"
	"Lumaestro/internal/utils"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// HandleNotification processa notificações assíncronas do processo ACP (streaming, progresso, etc).
func (h *ACPRpcHandler) HandleNotification(method string, params json.RawMessage) {
	// fmt.Printf("<< [ACP RECV Notify] %s: %s\n", method, string(params))

	// 1. Notificações de Progresso
	if method == "agent/progress" || method == "agentProgress" {
		var p struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(params, &p) == nil {
			if !strings.Contains(h.Session.ID, "-background-") {
				h.Executor.LogChan <- ExecutionLog{
					Source:  h.Session.AgentName,
					Content: fmt.Sprintf("⏳ %s...", p.Message),
				}
			}
		}
	}

	// 2. Notificações de Streaming de Sessão (O texto real da resposta)
	if method == "session/update" || method == "sessionUpdate" {
		h.logNetworkActivity()

		var p struct {
			SessionId string `json:"sessionId"`
			Update    struct {
				SessionUpdate string      `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				Text  string      `json:"text"`  // Suporte para formato plano v0.36
				Usage interface{} `json:"usage"` // 📊 Estatísticas de Token
				Stats interface{} `json:"stats"` // ⚡ Latência e Quota
			} `json:"update"`
		}
		if json.Unmarshal(params, &p) == nil {
			update := p.Update
			isBg := strings.Contains(h.Session.ID, "-background-")

			// 📊 Monitoramento de Cotas e Performance
			if (update.Usage != nil || update.Stats != nil) && !isBg {
				info := ""
				if usageMap, ok := update.Usage.(map[string]interface{}); ok {
					pt := usageMap["prompt_tokens"]
					ct := usageMap["candidates_tokens"]
					cache := usageMap["cachedContentTokenCount"]
					if cache == nil {
						cache = usageMap["cached_content_token_count"]
					}

					// Salva os valores na sessão para reportar no final do turno
					if pt != nil { h.Session.LastPromptTokens = int(pt.(float64)) }
					if ct != nil { h.Session.LastCandidatesTokens = int(ct.(float64)) }
					if cache != nil { h.Session.LastCacheTokens = int(cache.(float64)) }

					if pt != nil && ct != nil {
						if cache != nil && cache.(float64) > 0 {
							info = fmt.Sprintf("🧊 %.0f cache | %.0f in | %.0f out", cache, pt, ct)
						} else {
							info = fmt.Sprintf("%.0f in | %.0f out", pt, ct)
						}
					}
				}
				if statsMap, ok := update.Stats.(map[string]interface{}); ok {
					latency := statsMap["latency"]
					if latency != nil {
						if info != "" {
							info += fmt.Sprintf(" (%.0fms)", latency)
						} else {
							info = fmt.Sprintf("%.0fms", latency)
						}
					}
				}

				if info != "" {
					runtime.EventsEmit(h.Executor.Ctx, "agent:stats", map[string]string{
						"agent": h.Session.AgentName,
						"info":  info,
					})
				}
			}

			if update.SessionUpdate == "agent_message_chunk" || update.SessionUpdate == "message_chunk" || update.SessionUpdate == "content_chunk" || 
			   update.SessionUpdate == "user_message_chunk" || update.SessionUpdate == "user_message" || update.Content.Type == "user" {
				
				txt := update.Content.Text
				if txt == "" {
					txt = update.Text
				}

				msgType := "message"
				if update.Content.Type == "user" || update.SessionUpdate == "user_message" || update.SessionUpdate == "user_message_chunk" {
					msgType = "user"
					
					// 🧹 LIMPEZA DE HISTÓRICO: Remove diretrizes de sistema do prompt restaurado
					if strings.Contains(txt, "OBJETIVO ATUAL:") {
						parts := strings.Split(txt, "OBJETIVO ATUAL:")
						if len(parts) > 1 {
							txt = strings.TrimSpace(parts[1])
						}
					}
				}

				if txt != "" && !isBg {
					if !h.Session.isLoggingMessage && msgType == "message" {
						utils.LogInfo("O Maestro está orquestrando a resposta...", "💬")
						h.Session.isLoggingMessage = true
						h.Session.isLoggingThought = false
					}
					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: txt,
						Type:    msgType,
					}
				}
			} else if update.SessionUpdate == "agent_thought_chunk" || update.SessionUpdate == "thought_chunk" {
				txt := update.Content.Text
				if txt == "" {
					txt = update.Text
				}

				if txt != "" && !isBg {
					if !h.Session.isLoggingThought {
						utils.LogInfo(fmt.Sprintf("Processando raciocínio: %s...", strings.ToUpper(h.Session.AgentName)), "🧠")
						h.Session.isLoggingThought = true
						h.Session.isLoggingMessage = false
					}
					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: txt,
						Type:    "thought",
					}
				}
			} else if update.SessionUpdate == "agent_turn_complete" {
				h.Session.isLoggingThought = false
				h.Session.isLoggingMessage = false
				h.reportTurnCost()

				if !isBg {
					runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
				}

				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					close(ch)
					delete(h.Executor.turnChannels, h.Session.ID)
				}
				h.Executor.turnMu.Unlock()

			} else if update.SessionUpdate == "agent_message_error" || update.SessionUpdate == "error" {
				if !strings.Contains(h.Session.ID, "-background-") {
					h.Executor.LogChan <- ExecutionLog{
						Source:  "ERROR",
						Content: "⚠️ Aviso do Gemini: O formato da sua mensagem (prompt) pode ter sido rejeitado internamente.",
					}
				}
				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					close(ch)
					delete(h.Executor.turnChannels, h.Session.ID)
				}
				h.Executor.turnMu.Unlock()
			} else if update.SessionUpdate == "tool_call" {
				// 📡 TRANSPARÊNCIA: Avisa o Frontend qual ferramenta está sendo preparada
				// Gemini v0.36 envia tool_call via session/update
				if !isBg {
					action := "Executando ferramenta de análise..."
					if strings.TrimSpace(update.Content.Text) != "" {
						action = update.Content.Text
					} else if strings.TrimSpace(update.Text) != "" {
						action = update.Text
					}
					runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
						"agent":  h.Session.AgentName,
						"tool":   "thinking",
						"action": action,
						"kind":   "tool",
					})
				}
			}

			if update.SessionUpdate == "agent_message_chunk" && update.Content.Text != "" {
				h.Executor.turnMu.Lock()
				if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
					ch <- update.Content.Text
				}
				h.Executor.turnMu.Unlock()
			}
		}
	}
}

// HandleRequest lida com os pedidos de ferramenta (hands) da IA.
func (h *ACPRpcHandler) HandleRequest(id interface{}, method string, params json.RawMessage) {
	h.Executor.execLock <- struct{}{}
	defer func() { <-h.Executor.execLock }()

	utils.LogSection(fmt.Sprintf("FERRAMENTA: %s", method))
	utils.LogInfo(fmt.Sprintf("IA solicitou acesso a: %s", method), "🔧")
	normMethod := strings.ToLower(method)
	normMethod = strings.TrimPrefix(normMethod, "client/")
	normMethod = strings.TrimPrefix(normMethod, "fs/")

	var result interface{}
	var rpcErr *RPCError
	reviewID := fmt.Sprintf("rev-%v", id)

	// 🔒 PLAN MODE: Bloqueia ferramentas destrutivas
	if h.Session.PlanMode {
		writeTools := map[string]bool{
			"writefile": true, "write_file": true, "write_text_file": true, "write_file_content": true,
			"deletefile": true, "delete_file": true, "remove": true,
			"movefile": true, "move_file": true,
			"runcommand": true, "run_command": true, "run_shell_command": true, "execute_command": true,
		}
		if writeTools[normMethod] {
			rpcErr = &RPCError{
				Code:    403,
				Message: "🔒 Plan Mode ativo — operações de escrita bloqueadas. Aprove o plano para prosseguir para a fase de execução.",
			}
			h.Executor.SendRPC(h.Session, JSONRPCMessage{JSONRPC: JSONRPCVersion, ID: id, Error: rpcErr})
			return
		}
	}

	// 🪝 HOOKS: Pre-Tool Execution
	hookCtx := &HookContext{Session: h.Session, Method: method, Params: params}
	for _, hook := range GlobalHooks {
		if res := hook.BeforeTool(hookCtx); res != nil {
			if res.Abort {
				rpcErr = res.Error
				if rpcErr == nil {
					rpcErr = &RPCError{Code: 403, Message: res.Message}
				}
				h.Executor.SendRPC(h.Session, JSONRPCMessage{JSONRPC: JSONRPCVersion, ID: id, Error: rpcErr})
				return
			}
		}
	}

	defer func() {
		// 🪝 HOOKS: Post-Tool Execution
		for _, hook := range GlobalHooks {
			hook.AfterTool(hookCtx, result, rpcErr)
		}
	}()

	switch normMethod {
	case "readfile", "read_file", "read_text_file":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(params, &p) == nil {
			runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
				"agent":  h.Session.AgentName,
				"tool":   "read_file",
				"action": fmt.Sprintf("Lendo arquivo: %s", p.Path),
			})
			cfg, _ := config.Load()
			if cfg.Security.AllowRead {
				content, err := h.Executor.Proxy.ReadFile(p.Path)
				if err == nil {
					result = map[string]string{"content": content}
				} else {
					rpcErr = &RPCError{Code: -32000, Message: err.Error()}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ LEITURA BLOQUEADA"}
			}
		}

	case "writefile", "write_file", "write_text_file", "write_file_content":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if json.Unmarshal(params, &p) == nil {
			runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
				"agent":  h.Session.AgentName,
				"tool":   "write_file",
				"action": fmt.Sprintf("Escrevendo em: %s", p.Path),
			})
			cfg, _ := config.Load()
			fileExists := false
			if _, err := os.Stat(p.Path); err == nil {
				fileExists = true
			}

			canAct := false
			if fileExists {
				canAct = cfg.Security.AllowWrite
			} else {
				canAct = cfg.Security.AllowCreate
			}

			if canAct {
				needsReview := !cfg.Security.FullMachineAccess || strings.HasSuffix(p.Path, ".go") || strings.HasSuffix(p.Path, ".json")
				if needsReview {
					actionLabel := "ESCREVER ARQUIVO"
					if !fileExists {
						actionLabel = "CRIAR ARQUIVO"
					}
					if h.Executor.RequestReview(reviewID, actionLabel, p.Path) {
						err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
						if err == nil {
							result = map[string]bool{"success": true}
						} else {
							rpcErr = &RPCError{Code: -32001, Message: err.Error()}
						}
					} else {
						rpcErr = &RPCError{Code: 403, Message: "Ação recusada."}
					}
				} else {
					err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32001, Message: err.Error()}
					}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ ESCRITA BLOQUEADA"}
			}
		}

	case "deletefile", "delete_file", "remove":
		var p struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(params, &p) == nil {
			runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
				"agent":  h.Session.AgentName,
				"tool":   "delete_file",
				"action": fmt.Sprintf("Deletando: %s", p.Path),
			})
			cfg, _ := config.Load()
			if cfg.Security.AllowDelete {
				if h.Executor.RequestReview(reviewID, "DELETAR ARQUIVO", p.Path) {
					err := h.Executor.Proxy.DeleteFile(p.Path)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32002, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Recusado."}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ DELEÇÃO BLOQUEADA"}
			}
		}

	case "movefile", "move_file":
		var p struct {
			OldPath string `json:"oldPath"`
			NewPath string `json:"newPath"`
		}
		if json.Unmarshal(params, &p) == nil {
			runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
				"agent":  h.Session.AgentName,
				"tool":   "move_file",
				"action": fmt.Sprintf("Movendo: %s", p.OldPath),
			})
			cfg, _ := config.Load()
			if cfg.Security.AllowMove {
				details := fmt.Sprintf("%s -> %s", p.OldPath, p.NewPath)
				if h.Executor.RequestReview(reviewID, "MOVER/RENOMEAR", details) {
					err := h.Executor.Proxy.MoveFile(p.OldPath, p.NewPath)
					if err == nil {
						result = map[string]bool{"success": true}
					} else {
						rpcErr = &RPCError{Code: -32003, Message: err.Error()}
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Recusado."}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ MOVIMENTAÇÃO BLOQUEADA"}
			}
		}

	case "runcommand", "run_command", "run_shell_command", "execute_command":
		var p struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if json.Unmarshal(params, &p) == nil {
			runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
				"agent":  h.Session.AgentName,
				"tool":   "run_command",
				"action": fmt.Sprintf("Executando: %s", p.Command),
			})
			cfg, _ := config.Load()
			if cfg.Security.AllowRunCommands {
				details := fmt.Sprintf("%s %s", p.Command, strings.Join(p.Args, " "))
				runtime.EventsEmit(h.Executor.Ctx, "agent:status", map[string]string{
					"agent":  h.Session.AgentName,
					"action": "Executando comando: " + details,
					"kind":   "command",
				})
				h.Executor.LogChan <- ExecutionLog{Source: h.Session.AgentName, Content: "🧰 " + details, Type: "thought"}
				if h.Executor.RequestReview(reviewID, "EXECUTAR COMANDO", details) {
					output, err := h.Executor.Proxy.RunCommand(p.Command, p.Args)
					if err == nil {
						result = map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": output}}}
						out := strings.TrimSpace(output)
						if len(out) > 300 {
							out = out[:300] + "..."
						}
						h.Executor.LogChan <- ExecutionLog{Source: h.Session.AgentName, Content: "✅ Comando concluído: " + out, Type: "thought"}
						h.emitReward(details, nil)
					} else {
						rpcErr = &RPCError{Code: -32004, Message: err.Error()}
						h.Executor.LogChan <- ExecutionLog{Source: "ERROR", Content: "❌ Erro no comando: " + err.Error()}
						h.emitReward(details, err)
					}
				} else {
					rpcErr = &RPCError{Code: 403, Message: "Recusado."}
				}
			} else {
				rpcErr = &RPCError{Code: 403, Message: "🛡️ EXECUÇÃO BLOQUEADA"}
			}
		}

	case "delegate_task", "spawn_subagent", "subagent":
		var p struct {
			Agent string `json:"agent"`
			Goal  string `json:"goal"`
		}
		if json.Unmarshal(params, &p) == nil {
			// 1. Spawning do Subagente Isolado
			subSess, err := h.Executor.SpawnSubagent(h.Session, p.Agent, p.Goal)
			if err != nil {
				rpcErr = &RPCError{Code: -32007, Message: "Falha ao spawnar subagente: " + err.Error()}
			} else {
				// 2. Execução Síncrona (Aguardando resposta do subagente)
				resp, errAsk := h.Executor.AskSync(subSess.ID, p.Goal, nil)
				if errAsk != nil {
					rpcErr = &RPCError{Code: -32008, Message: "Falha na execução do subagente: " + errAsk.Error()}
				} else {
					result = map[string]string{"result": resp}
				}
				// 3. Cleanup automático do subagente efêmero
				h.Executor.StopSession(subSess.ID)
			}
		}

	default:
		if method == "session/request_permission" {
			result = map[string]interface{}{"permitted": true}
		} else if strings.HasPrefix(method, "Lumaestro/") {
			toolName := strings.TrimPrefix(method, "Lumaestro/")
			res, err := h.executeNativeTool(toolName, params)
			if err == nil {
				result = res
			} else {
				rpcErr = &RPCError{Code: -32005, Message: err.Error()}
			}
		} else {
			rpcErr = &RPCError{Code: -32601, Message: "🛡️ AÇÃO BLOQUEADA"}
		}
	}

	h.Executor.SendRPC(h.Session, JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  h.wrapResult(result),
		Error:   rpcErr,
	})

	if !strings.Contains(h.Session.ID, "-background-") {
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
}

// HandleResponse processa as respostas diretas às requisições feitas pelo executor.
func (h *ACPRpcHandler) HandleResponse(id interface{}, result json.RawMessage, rpcErr *RPCError) {
	// fmt.Printf("<< [ACP RECV Resp] ID %v: %s\n", id, string(result))
	idFloat, ok := id.(float64)
	if !ok {
		return
	}
	idInt := int(idFloat)

	h.Executor.requestsMu.Lock()
	ch, found := h.Executor.pendingRequests[idInt]
	h.Executor.requestsMu.Unlock()

	if found {
		ch <- JSONRPCMessage{ID: id, Result: result, Error: rpcErr}
		if idInt > 3 && strings.EqualFold(h.Session.AgentName, "lmstudio") && !strings.Contains(h.Session.ID, "-background-") {
			// Fallback para LM Studio: alguns fluxos podem não publicar session/update final.
			runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
		}
		return
	}

	if rpcErr != nil {
		isResilienceError := strings.Contains(rpcErr.Message, "exhausted your daily quota") || 
						 strings.Contains(rpcErr.Message, "429") || 
						 strings.Contains(rpcErr.Message, "quota exceeded") ||
						 strings.Contains(rpcErr.Message, "INTERNAL") ||
						 strings.Contains(rpcErr.Message, "500")

		if strings.Contains(rpcErr.Message, "Model stream ended with empty response") {
			h.Executor.LogChan <- ExecutionLog{Source: "SYSTEM", Content: "O Gemini decidiu não responder agora."}
		} else if isResilienceError {
			h.Executor.LogChan <- ExecutionLog{Source: "RESILIENCE", Content: "🔄 Instabilidade ou Cota detectada! Rotacionando frota para garantir a resposta..."}
			// Tenta rotacionar a chave e o modelo em background
			go h.Executor.HandleQuotaExhausted(h.Session.ID)
		} else {
			h.Executor.LogChan <- ExecutionLog{Source: "ERROR", Content: fmt.Sprintf("❌ Erro ACP: %s", rpcErr.Message)}
		}

		// 🔓 LIBERAÇÃO DE ERRO: Garante que a UI destrave se houver erro no motor
		if !strings.Contains(h.Session.ID, "-background-") {
			runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
		}

		h.Executor.turnMu.Lock()
		if ch, ok := h.Executor.turnChannels[h.Session.ID]; ok {
			close(ch)
			delete(h.Executor.turnChannels, h.Session.ID)
		}
		h.Executor.turnMu.Unlock()
		return
	}

	var response map[string]interface{}
	if json.Unmarshal(result, &response) == nil && response != nil {
		if sessID, ok := response["sessionId"].(string); ok {
			h.Session.ACPSessID = sessID
		}
	}

	if idInt <= 3 {
		select {
		case h.Session.initDone <- struct{}{}:
		default:
		}
		return
	}

	select {
	case h.Session.initDone <- struct{}{}:
	default:
	}
	if !strings.Contains(h.Session.ID, "-background-") {
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
}

func (h *ACPRpcHandler) wrapResult(res interface{}) json.RawMessage {
	if res == nil {
		return nil
	}
	b, _ := json.Marshal(res)
	return b
}
