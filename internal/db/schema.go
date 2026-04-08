package db

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Timestamp é um wrapper para time.Time que resolve o erro de binding do Wails v2 (Not found: time.Time)
type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", t.Format(time.RFC3339))), nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	parsed, err := time.Parse(`"`+time.RFC3339+`"`, string(data))
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

func (t Timestamp) Value() (driver.Value, error) {
	return t.Time, nil
}

func (t *Timestamp) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	if v, ok := value.(time.Time); ok {
		t.Time = v
		return nil
	}
	return fmt.Errorf("tipo incompatível para Timestamp: %v", value)
}

// Base model using UUID em vez do uint ID
type Base struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	CreatedAt Timestamp `json:"created_at"`
	UpdatedAt Timestamp `json:"updated_at"`
}

// Antes de criar, gerar UUID se necessário
func (base *Base) BeforeCreate(tx *gorm.DB) (err error) {
	if base.ID == uuid.Nil {
		base.ID = uuid.New()
	}
	return
}

// 7.2 Agent (Funcionário da empresa)
type Agent struct {
	Base
	Name               string     `gorm:"not null" json:"name"`
	Role               string     `gorm:"not null" json:"role"`
	Status             string     `gorm:"default:'idle';index" json:"status"` // active | paused | idle | running | error
	ReportsToID        *uuid.UUID `gorm:"type:uuid;index" json:"reports_to"`
	Capabilities       string     `json:"capabilities"`
	BudgetMonthlyCents int        `gorm:"not null;default:0" json:"budget_monthly_cents"`
	SpentMonthlyCents  int        `gorm:"not null;default:0" json:"spent_monthly_cents"`
	LastHeartbeatAt    Timestamp  `json:"last_heartbeat_at"`
}

// 16.1 AgentSecrets (API Keys e Chaves de Terceiros)
type AgentSecret struct {
	Base
	AgentID uuid.UUID `gorm:"type:uuid;not null;index" json:"agent_id"`
	Key     string    `gorm:"not null" json:"key"`
	Value   string    `gorm:"not null" json:"value"` // Inyectado como ENV na sessão
}

// 7.4 Goal (Objetivos da Empresa)
type Goal struct {
	Base
	Title          string     `gorm:"not null" json:"title"`
	Description    string     `json:"description"`
	Level          string     `gorm:"default:'company'" json:"level"` // company | team | agent
	ParentID       *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`
	OwnerAgentID   *uuid.UUID `gorm:"type:uuid;index" json:"owner_agent_id"`
	Status         string     `gorm:"default:'planned'" json:"status"`
}

// 7.5 Project (Roadmap)
type Project struct {
	Base
	GoalID        *uuid.UUID `gorm:"type:uuid;index" json:"goal_id"`
	Name          string     `gorm:"not null" json:"name"`
	Description   string     `json:"description"`
	Status        string     `gorm:"default:'backlog'" json:"status"`
	LeadAgentID   *uuid.UUID `gorm:"type:uuid;index" json:"lead_agent_id"`
	TargetDate    *Timestamp `json:"target_date"`
}

// 7.6 Issue (Tarefa Unica)
type Issue struct {
	Base
	ProjectID         *uuid.UUID `gorm:"type:uuid;index" json:"project_id"`
	GoalID            *uuid.UUID `gorm:"type:uuid;index" json:"goal_id"`
	ParentID          *uuid.UUID `gorm:"type:uuid;index" json:"parent_id"`
	Title             string     `gorm:"not null" json:"title"`
	Description       string     `json:"description"`
	Status            string     `gorm:"default:'todo';index" json:"status"` // backlog | todo | in_progress | done
	Priority          string     `gorm:"default:'medium'" json:"priority"`
	AssigneeAgentID   *uuid.UUID `gorm:"type:uuid;index" json:"assignee_agent_id"`
	AssigneeAgent     *Agent     `json:"assignee_agent" gorm:"foreignKey:AssigneeAgentID"`
	CreatedByAgentID  *uuid.UUID `gorm:"type:uuid" json:"created_by_agent_id"`
	StartedAt         *Timestamp `json:"started_at"`
	CompletedAt       *Timestamp `json:"completed_at"`
}

// 7.7 IssueComment (Timeline)
type IssueComment struct {
	Base
	IssueID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"issue_id"`
	AuthorAgentID  *uuid.UUID `gorm:"type:uuid;index" json:"author_agent_id"`
	AuthorAgent    *Agent     `json:"author_agent" gorm:"foreignKey:AuthorAgentID"`
	Body           string     `gorm:"not null" json:"body"`
}

// 7.15 Documents (Documentação Gerada por IA / RAG Base)
type Document struct {
	Base
	Title                string     `gorm:"not null" json:"title"`
	Format               string     `gorm:"default:'markdown'" json:"format"`
	LatestBody           string     `gorm:"type:text" json:"latest_body"`
	LatestRevisionNumber int        `gorm:"default:1" json:"latest_revision_number"`
	IssueID              *uuid.UUID `gorm:"type:uuid;index" json:"issue_id"`
	CreatedByAgentID     *uuid.UUID `gorm:"type:uuid" json:"created_by_agent_id"`
}

type DocumentRevision struct {
	Base
	DocumentID     uuid.UUID `gorm:"type:uuid;not null;index" json:"document_id"`
	RevisionNumber int       `gorm:"not null" json:"revision_number"`
	Body           string    `gorm:"type:text" json:"body"`
	ChangeSummary  string    `json:"change_summary"`
}

// 7.14 Assets (Arquivos Binários/Anexos)
type Asset struct {
	Base
	Provider         string     `gorm:"default:'local_disk'" json:"provider"`
	ObjectKey        string     `gorm:"not null;index" json:"object_key"`
	ContentType      string     `json:"content_type"`
	ByteSize         int64      `json:"byte_size"`
	OriginalFilename string     `json:"original_filename"`
	CreatedByAgentID *uuid.UUID `gorm:"type:uuid" json:"created_by_agent_id"`
}

type IssueAttachment struct {
	Base
	IssueID    uuid.UUID `gorm:"type:uuid;not null;index" json:"issue_id"`
	AssetID    uuid.UUID `gorm:"type:uuid;not null;index" json:"asset_id"`
	DocumentID *uuid.UUID `gorm:"type:uuid;index" json:"document_id"`
}

// Logs Técnicos e de Custo
type HeartbeatRun struct {
	Base
	AgentID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"agent_id"`
	InvocationSource string     `json:"invocation_source"` // scheduler | manual
	Status           string     `gorm:"index" json:"status"` // succeeded | failed | running
	Error            string     `json:"error"`
	StartedAt        *Timestamp `json:"started_at"`
	FinishedAt       *Timestamp `json:"finished_at"`
}

type CostEvent struct {
	Base
	AgentID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"agent_id"`
	IssueID      *uuid.UUID `gorm:"type:uuid;index" json:"issue_id"`
	Provider     string     `json:"provider"`
	Model        string     `json:"model"`
	InputTokens  int        `json:"input_tokens"`
	OutputTokens int        `json:"output_tokens"`
	CostCents    int        `json:"cost_cents"`
	OccurredAt   Timestamp  `json:"occurred_at"`
}

// 7.10 Approvals (Portões Humanos)
type Approval struct {
	Base
	Type                string     `gorm:"not null" json:"type"` // hire_agent | approve_strategy | agent_request
	RequestedByAgentID  *uuid.UUID `gorm:"type:uuid" json:"requested_by_agent_id"`
	Status              string     `gorm:"default:'pending';index" json:"status"`
	Payload             string     `gorm:"type:text" json:"payload"`
	DecisionNote        string     `json:"decision_note"`
	DecidedAt           *Timestamp `json:"decided_at"`
}

type ActivityLog struct {
	Base
	ActorType  string `json:"actor_type"` // agent | user | system
	ActorID    string `json:"actor_id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Details    string `json:"details"`
}
