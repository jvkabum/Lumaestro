package acp

import (
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"sync"

	"Lumaestro/internal/lightning"
	"Lumaestro/internal/utils"
	"github.com/google/uuid"
)

// Constantes do Protocolo
const (
	JSONRPCVersion = "2.0"
)

// --- Estruturas do Executor Central ---

// ACPExecutor gerencia a execução do Gemini CLI em modo --acp (JSON-RPC)
type ACPExecutor struct {
	Mu             sync.Mutex
	msgIDCounter   uint64
	ActiveSessions map[string]*ACPSession
	LogChan        chan ExecutionLog
	TerminalOutput chan TerminalData
	Proxy          *FSProxy // As "mãos" do backend

	// Modo Autônomo (--approval-mode=yolo) ou as flags de segurança finas
	AutonomousMode bool
	Ctx            context.Context

	pendingReviews map[string]chan bool
	reviewMu       sync.Mutex

	// pendingRequests mapeia o ID da mensagem para um canal que receberá o resultado.
	pendingRequests   map[int]chan JSONRPCMessage
	requestsMu        sync.Mutex
	
	Tools             *ToolRegistry // 🛠️ Biblioteca de ferramentas do Obsidian
	
	// Fila de execução para ferramentas (Semáforo)
	execLock chan struct{}

	// 📡 Agregador de logs de rede
	NetLog *utils.NetworkLogger
	
	// Turnos Ativos para AskSync
	turnChannels map[string]chan string
	turnMu       sync.Mutex

	// ✨ Motores de Elite (Lightning)
	LStore         *lightning.DuckDBStore
	RewardEngine   *lightning.RewardEngine
}

// ACPSession representa a conexão JSON-RPC ativa com um Agent Server.
type ACPSession struct {
	ID        string // Lumaestro Session ID
	ACPSessID string // ACP Internal Session ID
	AgentName string
	Cmd       *exec.Cmd
	Stdin     io.WriteCloser
	Cancel    context.CancelFunc
	// initDone sinaliza eventos de inicialização.
	// Usamos buffer de 1 para evitar bloqueios ao sinalizar.
	initDone chan struct{}
	
	// Governança e Orquestração Swarm
	AgentID        uuid.UUID
	CurrentIssueID *uuid.UUID

	// Trava de escrita para garantir integridade do JSON no stdin
	WriteMu sync.Mutex

	// Estados de log para evitar flooding no terminal
	isLoggingThought bool
	isLoggingMessage bool

	// 🧬 Telemetria Lightning (Rastreamento de Elite)
	RolloutID string
	AttemptID string

	// 📊 Telemetria de Cotas (ACP /stats)
	ModelRequestsUsed  int
	ModelRequestsLimit int
	ModelRequestsInfo  string
}

// ACPRpcHandler lida com o despacho de mensagens do protocolo JSON-RPC.
type ACPRpcHandler struct {
	Executor *ACPExecutor
	Session  *ACPSession
}

// ToolRegistry gerencia a biblioteca de funções disponíveis para a IA.
type ToolRegistry struct {
	Ctx     interface{}
	Tools   map[string]AgentTool
	Indexer interface{} // 🧩 Elo com o Crawler/RAG
}

// --- Estruturas de Protocolo e Mensagens ---

// JSONRPCMessage define o formato base para mensagens do protocolo ndJSON.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError define erros no formato JSON-RPC 2.0.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ExecutionLog define o formato das mensagens de log enviadas para a UI.
type ExecutionLog struct {
	Source  string `json:"source"`
	Content string `json:"content"`
	Type    string `json:"type,omitempty"` // message, thought, system, error
}

// TerminalData define chunks de texto brutos do processo para compatibilidade PTY.
type TerminalData struct {
	Agent string `json:"agent"`
	Data  []byte `json:"data"`
}

// --- Estruturas de Gestão de Sessão (Checkpoints) ---

// SessionInfo representa metadados de uma sessão ACP (Checkpoint).
type SessionInfo struct {
	SessionID        string `json:"sessionId"`
	Title            string `json:"title"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
	File             string `json:"file"`
	IsCurrentSession bool   `json:"isCurrentSession"`
}

// ListSessionsResponse é a resposta estruturada para o método listSessions.
type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}
