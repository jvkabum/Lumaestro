package lightning

import (
	"time"
)

// RolloutStatus representa o estado atual de um rollout.
type RolloutStatus string

const (
	StatusQueuing    RolloutStatus = "queuing"
	StatusPreparing  RolloutStatus = "preparing"
	StatusRunning    RolloutStatus = "running"
	StatusFailed     RolloutStatus = "failed"
	StatusSucceeded  RolloutStatus = "succeeded"
	StatusCancelled  RolloutStatus = "cancelled"
	StatusRequeuing  RolloutStatus = "requeuing"
)

// AttemptStatus representa o estado de uma tentativa específica.
type AttemptStatus string

const (
	AttemptPreparing   AttemptStatus = "preparing"
	AttemptRunning     AttemptStatus = "running"
	AttemptFailed      AttemptStatus = "failed"
	AttemptSucceeded   AttemptStatus = "succeeded"
	AttemptUnresponsive AttemptStatus = "unresponsive"
	AttemptTimeout     AttemptStatus = "timeout"
)

// RolloutMode define se é treino, validação ou teste.
type RolloutMode string

const (
	ModeTrain RolloutMode = "train"
	ModeVal   RolloutMode = "val"
	ModeTest  RolloutMode = "test"
)

// Triplet captura um turno de interação (pergunta, resposta, recompensa).
type Triplet struct {
	Prompt   interface{}            `json:"prompt"`
	Response interface{}            `json:"response"`
	Reward   *float64               `json:"reward,omitempty"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Attempt representa uma tentativa de execução de um rollout.
type Attempt struct {
	RolloutID         string                 `json:"rollout_id"`
	AttemptID         string                 `json:"attempt_id"`
	SequenceID        int                    `json:"sequence_id"`
	StartTime         float64                `json:"start_time"`
	EndTime           *float64               `json:"end_time,omitempty"`
	Status            AttemptStatus          `json:"status"`
	WorkerID          *string                 `json:"worker_id,omitempty"`
	LastHeartbeatTime *float64               `json:"last_heartbeat_time,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// RolloutConfig controla retentativas e timeouts.
type RolloutConfig struct {
	TimeoutSeconds      *float64        `json:"timeout_seconds,omitempty"`
	UnresponsiveSeconds *float64        `json:"unresponsive_seconds,omitempty"`
	MaxAttempts         int             `json:"max_attempts"`
	RetryCondition      []AttemptStatus `json:"retry_condition"`
}

// Rollout é o modelo canônico de uma execução de agente.
type Rollout struct {
	RolloutID   string                 `json:"rollout_id"`
	Input       interface{}            `json:"input"`
	StartTime   float64                `json:"start_time"`
	EndTime     *float64               `json:"end_time,omitempty"`
	Mode        *RolloutMode           `json:"mode,omitempty"`
	ResourcesID *string                 `json:"resources_id,omitempty"`
	Status      RolloutStatus          `json:"status"`
	Config      RolloutConfig          `json:"config"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// --- Telemetria (Spans) ---

// SpanContext representa o contexto OTel.
type SpanContext struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	IsRemote   bool              `json:"is_remote"`
	TraceState map[string]string `json:"trace_state"`
}

// TraceStatus representa o status de um Span.
type TraceStatus struct {
	StatusCode  string  `json:"status_code"` // UNSET, OK, ERROR
	Description *string `json:"description,omitempty"`
}

// Event representa um evento dentro de um Span.
type Event struct {
	Name       string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes"`
	Timestamp  float64                `json:"timestamp"`
}

// OtelResource representa o recurso OTel.
type OtelResource struct {
	Attributes map[string]interface{} `json:"attributes"`
	SchemaURL  string                 `json:"schema_url"`
}

// Span é o modelo de persistência analítica do Lightning.
type Span struct {
	RolloutID  string `json:"rollout_id"`
	AttemptID  string `json:"attempt_id"`
	SequenceID int    `json:"sequence_id"`

	TraceID  string  `json:"trace_id"`
	SpanID   string  `json:"span_id"`
	ParentID *string `json:"parent_id,omitempty"`

	Name       string                 `json:"name"`
	Status     TraceStatus            `json:"status"`
	Attributes map[string]interface{} `json:"attributes"`
	Events     []Event                `json:"events"`
	
	StartTime float64  `json:"start_time"`
	EndTime   *float64 `json:"end_time,omitempty"`

	// 💰 Métricas de Custo (Tokens)
	PromptTokens     int `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`

	Resource OtelResource `json:"resource"`
}

// GetNowTimestamp retorna o timestamp atual em segundos (float64) para compatibilidade com o ecossistema Python.
func GetNowTimestamp() float64 {
	return float64(time.Now().UnixNano()) / 1e9
}
