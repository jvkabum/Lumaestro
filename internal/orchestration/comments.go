package orchestration

import (
	"Lumaestro/internal/db"
	"fmt"

	"github.com/google/uuid"
)

// AddIssueComment permite que um agente ou o sistema registre um comentário em uma tarefa.
// Esses comentários servem como a "Memória de Curto Prazo" e "Trilha de Raciocínio" da tarefa.
func AddIssueComment(agentID, issueID uuid.UUID, body string) error {
	comment := db.IssueComment{
		IssueID:       issueID,
		AuthorAgentID: &agentID,
		Body:          body,
	}

	if err := db.InstanceDB.Create(&comment).Error; err != nil {
		return fmt.Errorf("erro ao registrar comentário: %w", err)
	}

	// Também logamos a atividade de forma resumida para o Audit geral
	log := db.ActivityLog{
		ActorType:  "agent",
		ActorID:    agentID.String(),
		Action:     "issue_comment_added",
		EntityType: "issue",
		EntityID:   issueID.String(),
		Details:    fmt.Sprintf("Agente comentou na issue: %s", body),
	}
	db.InstanceDB.Create(&log)

	return nil
}

// GetTimelineByIssue retorna todos os eventos (comentários e mudanças de status) vinculados a uma tarefa específica.
func GetTimelineByIssue(issueID uuid.UUID) ([]db.IssueComment, error) {
	var comments []db.IssueComment
	err := db.InstanceDB.Preload("AuthorAgent").Where("issue_id = ?", issueID).Order("created_at ASC").Find(&comments).Error
	return comments, err
}
