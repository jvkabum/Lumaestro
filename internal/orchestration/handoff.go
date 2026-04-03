package orchestration

import (
	"Lumaestro/internal/db"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DelegateTask permite que um agente passe o bastão para outro. 
// Cria uma nova tarefa na fila do destinatário e vincula à tarefa pai (se houver).
func DelegateTask(fromAgentID, toAgentID uuid.UUID, parentIssueID *uuid.UUID, title, description string) (uuid.UUID, error) {
	// 1. Validar Agente Destino
	var toAgent db.Agent
	if err := db.InstanceDB.First(&toAgent, "id = ?", toAgentID).Error; err != nil {
		return uuid.Nil, fmt.Errorf("agente de destino não encontrado: %w", err)
	}

	// 2. Criar Nova Issue (Ticket Delegado)
	newIssue := db.Issue{
		ParentID:         parentIssueID,
		Title:            title,
		Description:      description,
		Status:           "todo",
		Priority:         "high",
		AssigneeAgentID:  &toAgentID,
		CreatedByAgentID: &fromAgentID,
	}

	if err := db.InstanceDB.Create(&newIssue).Error; err != nil {
		return uuid.Nil, fmt.Errorf("erro ao criar tarefa delegada: %w", err)
	}

	// 3. Registrar o Evento na Timeline da Tarefa Pai (se houver)
	if parentIssueID != nil {
		body := fmt.Sprintf("Deleguei o trabalho para %s: %s", toAgent.Name, title)
		AddIssueComment(fromAgentID, *parentIssueID, body)
	}

	// 4. Log de Auditoria do Handoff
	log := db.ActivityLog{
		ActorType:  "agent",
		ActorID:    fromAgentID.String(),
		Action:     "task_delegated",
		EntityType: "issue",
		EntityID:   newIssue.ID.String(),
		Details:    fmt.Sprintf("Handoff: %s -> %s | Ticket: %s", fromAgentID, toAgentID, title),
	}
	db.InstanceDB.Create(&log)

	// 5. Opcional: Pausar o agente de origem se ele estiver esperando resposta (Workflow dependente)
	// No Paperclip V1, o handoff é assíncrono. O original pode continuar ou dormir.
	
	return newIssue.ID, nil
}

// CompleteTask finaliza uma tarefa e notifica o log
func CompleteTask(agentID, issueID uuid.UUID) error {
	var issue db.Issue
	if err := db.InstanceDB.First(&issue, "id = ?", issueID).Error; err != nil {
		return err
	}

	now := time.Now()
	issue.Status = "done"
	issue.CompletedAt = &now

	if err := db.InstanceDB.Save(&issue).Error; err != nil {
		return err
	}

	AddIssueComment(agentID, issueID, "✔ Tarefa concluída com sucesso!")
	
	return nil
}
