package acp

import (
	"encoding/json"
	"time"
)

// Constantes fundamentais do protocolo ACP
const JSONRPCVersion = "2.0"

// JSONRPCMessage representa a estrutura de um pacote JSON-RPC (ndJSON).
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// RPCError é a estrutura oficial para erros do protocolo.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ExecutionLog representa uma linha de log enviada para a UI.
type ExecutionLog struct {
	Source    string    `json:"source"`
	Content   string    `json:"content"`
	Type      string    `json:"type,omitempty"` // message, thought, error, system
	Timestamp time.Time `json:"timestamp"`
}

// TerminalData representa chunks de output bruto do processo.
type TerminalData struct {
	Agent string
	Data  []byte
}
