package acp

import (
	"encoding/json"
	"fmt"

	"Lumaestro/internal/db"
	"Lumaestro/internal/orchestration"
	"github.com/google/uuid"
)

// AgentTool define uma função que a IA pode invocar.
type AgentTool struct {
	Name     string
	Function func(args map[string]interface{}) (string, error)
}

// NewToolRegistry inicializa a biblioteca padrão de ferramentas (Obsidian).
func NewToolRegistry() *ToolRegistry {
	tr := &ToolRegistry{
		Tools: make(map[string]AgentTool),
	}
	// As ferramentas do Obsidian são injetadas pelo StartSession via Ctx
	return tr
}

// executeNativeTool processa ferramentas internas do Lumaestro (Handoff, Ticket, etc).
func (h *ACPRpcHandler) executeNativeTool(toolName string, params json.RawMessage) (interface{}, error) {
	switch toolName {
	case "delegate_task":
		var p struct {
			ToAgentID   string `json:"to_agent_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(params, &p); err == nil {
			targetID, _ := uuid.Parse(p.ToAgentID)
			_, errTask := orchestration.DelegateTask(h.Session.AgentID, targetID, h.Session.CurrentIssueID, p.Title, p.Description)
			if errTask == nil { return map[string]string{"success": "Trabalho delegado com sucesso!"}, nil }
			return nil, errTask
		}
		return nil, fmt.Errorf("parâmetros inválidos para delegação")

	case "complete_task":
		if h.Session.CurrentIssueID != nil {
			err := orchestration.CompleteTask(h.Session.AgentID, *h.Session.CurrentIssueID)
			if err == nil { return map[string]string{"success": "Tarefa marcada como concluída e arquivada."}, nil }
			return nil, err
		}
		return nil, fmt.Errorf("nenhum ticket ativo vinculado a esta sessão")

	case "request_approval":
		var p struct {
			Topic   string `json:"topic"`
			Details string `json:"details"`
		}
		if err := json.Unmarshal(params, &p); err == nil {
			approval := db.Approval{
				Type:               "agent_request",
				RequestedByAgentID: &h.Session.AgentID,
				Payload:            fmt.Sprintf("TÓPICO: %s\n\nDETALHES: %s", p.Topic, p.Details),
			}
			if err := db.InstanceDB.Create(&approval).Error; err == nil {
				// PAUSA O AGENTE IMEDIATAMENTE (Portão Ativo)
				db.InstanceDB.Model(&db.Agent{}).Where("id = ?", h.Session.AgentID).Update("status", "paused")
				return map[string]string{
					"success": "Solicitação enviada. A execução permanecerá pausada até a aprovação humana.",
					"approval_id": approval.ID.String(),
				}, nil
			}
			return nil, err
		}
		return nil, fmt.Errorf("parâmetros inválidos para pedido de aprovação")

	default:
		return nil, fmt.Errorf("ferramenta nativa '%s' não reconhecida", toolName)
	}
}
