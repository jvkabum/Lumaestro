package orchestration

import (
	"Lumaestro/internal/db"
	"Lumaestro/internal/lightning" // ✨ Importação Lightning
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RequestApproval cria um pedido de portão de aprovação que pausa o agente até que um humano decida.
func RequestApproval(agentID uuid.UUID, approvalType string, payload interface{}) (uuid.UUID, error) {
	payloadJSON, _ := json.Marshal(payload)

	approval := db.Approval{
		Type:               approvalType,
		RequestedByAgentID: &agentID,
		Status:             "pending",
		Payload:            string(payloadJSON),
	}

	if err := db.InstanceDB.Create(&approval).Error; err != nil {
		return uuid.Nil, err
	}

	// Pausa o Agente automaticamente (Portão Ativo)
	var agent db.Agent
	if err := db.InstanceDB.First(&agent, "id = ?", agentID).Error; err == nil {
		agent.Status = "paused"
		db.InstanceDB.Save(&agent)
	}

	// Log de auditoria do pedido
	log := db.ActivityLog{
		ActorType:  "agent",
		ActorID:    agentID.String(),
		Action:     "approval_requested",
		EntityType: "approval",
		EntityID:   approval.ID.String(),
		Details:    fmt.Sprintf("Agente solicitou aprovação para: %s", approvalType),
	}
	db.InstanceDB.Create(&log)

	return approval.ID, nil
}

// ProcessApproval responde ao pedido (aprovado ou rejeitado) e libera o agente se necessário.
func ProcessApproval(approvalID uuid.UUID, approved bool, note string) error {
	var approval db.Approval
	if err := db.InstanceDB.First(&approval, "id = ?", approvalID).Error; err != nil {
		return err
	}

	status := "approved"
	if !approved {
		status = "rejected"
	}

	now := db.Timestamp{Time: time.Now()}
	approval.Status = status
	approval.DecisionNote = note
	approval.DecidedAt = &now

	if err := db.InstanceDB.Save(&approval).Error; err != nil {
		return err
	}

	// ⚡ LIGHTNING REWARD: Transmitir a decisão humana para o cérebro analítico
	if db.AnalyticsDB != nil {
		if lStore, ok := db.AnalyticsDB.(*lightning.DuckDBStore); ok {
			re := lightning.NewRewardEngine(lStore)
			rewardValue := -1.0
			if approved {
				rewardValue = 1.0
			}

			// Tenta capturar o Agente para logar a recompensa corretamente
			agentID := "unknown"
			if approval.RequestedByAgentID != nil {
				agentID = approval.RequestedByAgentID.String()
			}

			re.EmitReward("roll-human-"+approvalID.String(), "att-1", rewardValue, "human_approval", map[string]interface{}{
				"agent_id": agentID,
				"note":     note,
				"type":     approval.Type,
			})
		}
	}

	// Se aprovado, podemos querer liberar o agente ou triggar uma ação específica
	if approval.RequestedByAgentID != nil {
		var agent db.Agent
		if err := db.InstanceDB.First(&agent, "id = ?", *approval.RequestedByAgentID).Error; err == nil {
			// Voltamos para IDLE para que o Heartbeat o pegue novamente
			agent.Status = "idle"
			db.InstanceDB.Save(&agent)
		}
	}

	log := db.ActivityLog{
		ActorType:  "user",
		ActorID:    "board",
		Action:     "approval_decision",
		EntityType: "approval",
		EntityID:   approvalID.String(),
		Details:    fmt.Sprintf("Human Board decidiu: %s. Nota: %s", status, note),
	}
	db.InstanceDB.Create(&log)

	return nil
}
