package orchestration

import (
	"Lumaestro/internal/db"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// UpsertDocument cria ou atualiza um documento atrelado a uma tarefa e gera uma revisão.
func UpsertDocument(agentID uuid.UUID, issueID uuid.UUID, title, body, changeSummary string) (*db.Document, error) {
	var doc db.Document
	err := db.InstanceDB.Where("issue_id = ? AND title = ?", issueID, title).First(&doc).Error

	if err != nil {
		// Criar Novo
		doc = db.Document{
			Title:                title,
			LatestBody:           body,
			LatestRevisionNumber: 1,
			IssueID:              &issueID,
			CreatedByAgentID:     &agentID,
		}
		if err := db.InstanceDB.Create(&doc).Error; err != nil {
			return nil, err
		}
	} else {
		// Atualizar Existente
		doc.LatestBody = body
		doc.LatestRevisionNumber++
		if err := db.InstanceDB.Save(&doc).Error; err != nil {
			return nil, err
		}
	}

	// Criar Revisão (Histórico / Git-style)
	rev := db.DocumentRevision{
		DocumentID:     doc.ID,
		RevisionNumber: doc.LatestRevisionNumber,
		Body:           body,
		ChangeSummary:  changeSummary,
	}
	db.InstanceDB.Create(&rev)

	// Log da Atividade
	db.InstanceDB.Create(&db.ActivityLog{
		ActorType:  "agent",
		ActorID:    agentID.String(),
		Action:     "document_updated",
		EntityType: "document",
		EntityID:   doc.ID.String(),
		Details:    fmt.Sprintf("Documento '%s' atualizado para versão %v", title, doc.LatestRevisionNumber),
	})

	// 🚀 Sincronizar com RAG (Exportar para disco)
	ExportToMarkdown(doc)

	return &doc, nil
}

// ExportToMarkdown salva o documento no disco para que o Crawler de RAG o indexe automaticamente.
func ExportToMarkdown(doc db.Document) error {
	cwd, _ := os.Getwd()
	docDir := filepath.Join(cwd, "paperclip", "knowledge", "swarm_documents")
	os.MkdirAll(docDir, 0755)

	filename := fmt.Sprintf("%s.md", doc.ID.String())
	filePath := filepath.Join(docDir, filename)

	content := fmt.Sprintf("# %s\n\n> Documento gerado pelo Swarm Lumaestro\n> ID: %s\n> Versão: %v\n> Data: %s\n\n---\n\n%s",
		doc.Title, doc.ID.String(), doc.LatestRevisionNumber, time.Now().Format("2006-01-02 15:04:05"), doc.LatestBody)

	return os.WriteFile(filePath, []byte(content), 0644)
}

// GetExecutiveSummary retorna estatísticas vitais para o Dashboard de Comando.
type ExecSummary struct {
	TotalSpentCents int `json:"total_spent_cents"`
	ActiveAgents    int `json:"active_agents"`
	PausedAgents    int `json:"paused_agents"`
	OpenIssues      int `json:"open_issues"`
	DoneIssues      int `json:"done_issues"`
	PendingApprovals int `json:"pending_approvals"`
}

func GetExecutiveSummary() (ExecSummary, error) {
	var summary ExecSummary
	
	// Custo Total
	db.InstanceDB.Model(&db.Agent{}).Select("SUM(spent_monthly_cents)").Scan(&summary.TotalSpentCents)
	
	// Agentes
	db.InstanceDB.Model(&db.Agent{}).Where("status = ?", "running").Count(nil).Scan(&summary.ActiveAgents)
	db.InstanceDB.Model(&db.Agent{}).Where("status = ?", "paused").Count(nil).Scan(&summary.PausedAgents)
	
	// Tarefas
	db.InstanceDB.Model(&db.Issue{}).Where("status != ?", "done").Count(nil).Scan(&summary.OpenIssues)
	db.InstanceDB.Model(&db.Issue{}).Where("status = ?", "done").Count(nil).Scan(&summary.DoneIssues)
	
	// Aprovações
	db.InstanceDB.Model(&db.Approval{}).Where("status = ?", "pending").Count(nil).Scan(&summary.PendingApprovals)
	
	return summary, nil
}
