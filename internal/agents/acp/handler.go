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
	fmt.Printf("<< [ACP RECV Notify] %s: %s\n", method, string(params))
	
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
				SessionUpdate string `json:"sessionUpdate"`
				Content       struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				Text string `json:"text"` // Suporte para formato plano v0.36
			} `json:"update"`
		}
		if json.Unmarshal(params, &p) == nil {
			update := p.Update
			isBg := strings.Contains(h.Session.ID, "-background-")
			
			if update.SessionUpdate == "agent_message_chunk" || update.SessionUpdate == "message_chunk" || update.SessionUpdate == "content_chunk" {
				txt := update.Content.Text
				if txt == "" { txt = update.Text }
				
				if txt != "" && !isBg {
					if !h.Session.isLoggingMessage {
						utils.LogInfo("O Maestro está orquestrando a resposta...", "💬")
						h.Session.isLoggingMessage = true
						h.Session.isLoggingThought = false 
					}
					h.Executor.LogChan <- ExecutionLog{
						Source:  h.Session.AgentName,
						Content: txt,
						Type:    "message",
					}
				}
			} else if update.SessionUpdate == "agent_thought_chunk" || update.SessionUpdate == "thought_chunk" {
				txt := update.Content.Text
				if txt == "" { txt = update.Text }
				
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

	switch normMethod {
	case "readfile", "read_file", "read_text_file":
		var p struct { Path string `json:"path"` }
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowRead {
				content, err := h.Executor.Proxy.ReadFile(p.Path)
				if err == nil { result = map[string]string{"content": content}
				} else { rpcErr = &RPCError{Code: -32000, Message: err.Error()} }
			} else { rpcErr = &RPCError{Code: 403, Message: "🛡️ LEITURA BLOQUEADA"} }
		}

	case "writefile", "write_file", "write_text_file", "write_file_content":
		var p struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			fileExists := false
			if _, err := os.Stat(p.Path); err == nil { fileExists = true }

			canAct := false
			if fileExists { canAct = cfg.Security.AllowWrite } else { canAct = cfg.Security.AllowCreate }

			if canAct {
				needsReview := !cfg.Security.FullMachineAccess || strings.HasSuffix(p.Path, ".go") || strings.HasSuffix(p.Path, ".json")
				if needsReview {
					actionLabel := "ESCREVER ARQUIVO"; if !fileExists { actionLabel = "CRIAR ARQUIVO" }
					if h.Executor.RequestReview(reviewID, actionLabel, p.Path) {
						err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
						if err == nil { result = map[string]bool{"success": true} } else { rpcErr = &RPCError{Code: -32001, Message: err.Error()} }
					} else { rpcErr = &RPCError{Code: 403, Message: "Ação recusada."} }
				} else {
					err := h.Executor.Proxy.WriteFile(p.Path, p.Content)
					if err == nil { result = map[string]bool{"success": true} } else { rpcErr = &RPCError{Code: -32001, Message: err.Error()} }
				}
			} else { rpcErr = &RPCError{Code: 403, Message: "🛡️ ESCRITA BLOQUEADA"} }
		}

	case "deletefile", "delete_file", "remove":
		var p struct { Path string `json:"path"` }
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowDelete {
				if h.Executor.RequestReview(reviewID, "DELETAR ARQUIVO", p.Path) {
					err := h.Executor.Proxy.DeleteFile(p.Path)
					if err == nil { result = map[string]bool{"success": true} } else { rpcErr = &RPCError{Code: -32002, Message: err.Error()} }
				} else { rpcErr = &RPCError{Code: 403, Message: "Recusado."} }
			} else { rpcErr = &RPCError{Code: 403, Message: "🛡️ DELEÇÃO BLOQUEADA"} }
		}

	case "movefile", "move_file":
		var p struct { OldPath string `json:"oldPath"`; NewPath string `json:"newPath"` }
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowMove {
				details := fmt.Sprintf("%s -> %s", p.OldPath, p.NewPath)
				if h.Executor.RequestReview(reviewID, "MOVER/RENOMEAR", details) {
					err := h.Executor.Proxy.MoveFile(p.OldPath, p.NewPath)
					if err == nil { result = map[string]bool{"success": true} } else { rpcErr = &RPCError{Code: -32003, Message: err.Error()} }
				} else { rpcErr = &RPCError{Code: 403, Message: "Recusado."} }
			} else { rpcErr = &RPCError{Code: 403, Message: "🛡️ MOVIMENTAÇÃO BLOQUEADA"} }
		}

	case "runcommand", "run_command", "run_shell_command", "execute_command":
		var p struct { Command string `json:"command"`; Args []string `json:"args"` }
		if json.Unmarshal(params, &p) == nil {
			cfg, _ := config.Load()
			if cfg.Security.AllowRunCommands {
				details := fmt.Sprintf("%s %s", p.Command, strings.Join(p.Args, " "))
				if h.Executor.RequestReview(reviewID, "EXECUTAR COMANDO", details) {
					output, err := h.Executor.Proxy.RunCommand(p.Command, p.Args)
					if err == nil {
						result = map[string]interface{}{ "content": []map[string]interface{}{ {"type": "text", "text": output}, }, }
						h.emitReward(details, nil)
					} else {
						rpcErr = &RPCError{Code: -32004, Message: err.Error()}
						h.emitReward(details, err)
					}
				} else { rpcErr = &RPCError{Code: 403, Message: "Recusado."} }
			} else { rpcErr = &RPCError{Code: 403, Message: "🛡️ EXECUÇÃO BLOQUEADA"} }
		}

	default:
		if method == "session/request_permission" {
			result = map[string]interface{}{ "permitted": true }
		} else if strings.HasPrefix(method, "Lumaestro/") {
			toolName := strings.TrimPrefix(method, "Lumaestro/")
			res, err := h.executeNativeTool(toolName, params)
			if err == nil { result = res } else { rpcErr = &RPCError{Code: -32005, Message: err.Error()} }
		} else {
			rpcErr = &RPCError{ Code: -32601, Message: "🛡️ AÇÃO BLOQUEADA" }
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
	fmt.Printf("<< [ACP RECV Resp] ID %v: %s\n", id, string(result))
	idFloat, ok := id.(float64); if !ok { return }
	idInt := int(idFloat)

	h.Executor.requestsMu.Lock()
	ch, found := h.Executor.pendingRequests[idInt]
	h.Executor.requestsMu.Unlock()

	if found {
		ch <- JSONRPCMessage{ID: id, Result: result, Error:  rpcErr}
		return
	}

	if rpcErr != nil {
		if strings.Contains(rpcErr.Message, "Model stream ended with empty response") {
			h.Executor.LogChan <- ExecutionLog{Source: "SYSTEM", Content: "O Gemini decidiu não responder agora."}
			runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
		} else {
			h.Executor.LogChan <- ExecutionLog{Source: "ERROR", Content: fmt.Sprintf("❌ Erro ACP: %s", rpcErr.Message)}
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
		select { case h.Session.initDone <- struct{}{}: default: }
		return
	}

	select { case h.Session.initDone <- struct{}{}: default: }
	if !strings.Contains(h.Session.ID, "-background-") {
		runtime.EventsEmit(h.Executor.Ctx, "agent:turn_complete", h.Session.AgentName)
	}
}

func (h *ACPRpcHandler) wrapResult(res interface{}) json.RawMessage {
	if res == nil { return nil }
	b, _ := json.Marshal(res)
	return b
}
