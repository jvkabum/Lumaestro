package orchestration

import (
	"Lumaestro/internal/db"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CheckoutIssue tranca o ticket no banco de dados de forma atômica para um Agente, garantindo exclusividade.
func CheckoutIssue(agentID, issueID uuid.UUID) (*db.Issue, error) {
	var issue db.Issue

	// Transação para evitar race conditions no milissegundo 
	err := db.InstanceDB.Transaction(func(tx *gorm.DB) error {
		// Busca a issue garantindo que só tenta locar o que está livre
		if err := tx.Where("id = ? AND status IN (?)", issueID, []string{"todo", "backlog", "blocked"}).
			Where("assignee_agent_id IS NULL OR assignee_agent_id = ?", agentID).
			First(&issue).Error; err != nil {
			return fmt.Errorf("conflito 409 ou ticket indisponível: %w", err)
		}

		now := time.Now()
		issue.Status = "in_progress"
		issue.AssigneeAgentID = &agentID
		if issue.StartedAt == nil {
			issue.StartedAt = &now
		}

		// Save modifications
		if err := tx.Save(&issue).Error; err != nil {
			return err
		}

		// Trilha de auditoria explícita de saque
		log := db.ActivityLog{
			ActorType:  "agent",
			ActorID:    agentID.String(),
			Action:     "issue_checkout",
			EntityType: "issue",
			EntityID:   issueID.String(),
			Details:    "Agente fez checkout atômico e trancou o ticket para processamento.",
		}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &issue, nil
}

// ReleaseIssue libera um ticket que estava em progresso (ex: bloqueado por depender de alguém)
func ReleaseIssue(agentID, issueID uuid.UUID, newStatus string) error {
	return db.InstanceDB.Transaction(func(tx *gorm.DB) error {
		var issue db.Issue
		if err := tx.Where("id = ? AND assignee_agent_id = ?", issueID, agentID).First(&issue).Error; err != nil {
			return fmt.Errorf("agente não é o dono desse ticket ou ele não existe: %w", err)
		}

		issue.Status = newStatus
		if newStatus == "done" {
			now := time.Now()
			issue.CompletedAt = &now
		}

		if err := tx.Save(&issue).Error; err != nil {
			return err
		}

		log := db.ActivityLog{
			ActorType:  "agent",
			ActorID:    agentID.String(),
			Action:     "issue_release",
			EntityType: "issue",
			EntityID:   issueID.String(),
			Details:    fmt.Sprintf("Agente liberou o ticket para o status: %s", newStatus),
		}
		return tx.Create(&log).Error
	})
}
