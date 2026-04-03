package orchestration

import (
	"Lumaestro/internal/db"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RegistrarCusto contabiliza tokens gastos. Aciona "Hard Stop" se ultrapassar o mês.
func RegistrarCusto(agentID uuid.UUID, issueID *uuid.UUID, provider, model string, inTokens, outTokens, costCents int) error {
	// 1. Gravar Evento de Custo
	event := db.CostEvent{
		AgentID:      agentID,
		IssueID:      issueID,
		Provider:     provider,
		Model:        model,
		InputTokens:  inTokens,
		OutputTokens: outTokens,
		CostCents:    costCents,
		OccurredAt:   time.Now(),
	}

	if err := db.InstanceDB.Create(&event).Error; err != nil {
		return fmt.Errorf("falha ao salvar evento de custo: %w", err)
	}

	// 2. Acumular Gasto do Agente
	var agent db.Agent
	if err := db.InstanceDB.First(&agent, "id = ?", agentID).Error; err != nil {
		return err
	}

	// Incrementa
	agent.SpentMonthlyCents += costCents
	
	// 3. HARD STOP CHECK! (Budget Limits)
	if agent.BudgetMonthlyCents > 0 && agent.SpentMonthlyCents >= agent.BudgetMonthlyCents {
		// Estourou o limite! Pausa forçada de contenção.
		agent.Status = "paused"
		
		// Trilha de auditoria (Activity Log)
		log := db.ActivityLog{
			ActorType:  "system",
			ActorID:    "orchestrator",
			Action:     "agent_paused_out_of_budget",
			EntityType: "agent",
			EntityID:   agentID.String(),
			Details:    fmt.Sprintf("Agente excedeu o orçamento mensal: gasto %v / limit %v", agent.SpentMonthlyCents, agent.BudgetMonthlyCents),
		}
		db.InstanceDB.Create(&log)
	}

	// Salva novo saldo
	return db.InstanceDB.Save(&agent).Error
}
