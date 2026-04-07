package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// JSONRPCHandler define como uma mensagem recebida deve ser tratada.
type JSONRPCHandler interface {
	HandleNotification(method string, params json.RawMessage)
	HandleRequest(id interface{}, method string, params json.RawMessage)
	HandleResponse(id interface{}, result json.RawMessage, err *RPCError)
}

// StartJSONRPCListener lê mensagens no formato ACP oficial (ndJSON).
// Cada mensagem JSON-RPC é uma única linha terminada com '\n'.
func StartJSONRPCListener(r io.Reader, handler JSONRPCHandler) {
	scanner := bufio.NewScanner(r)
	
	// Buffer de 1MB para lidar com respostas RAG/Código muito longas (Original DNA)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Log bruto para diagnóstico (Original DNA)
		fmt.Printf("<< [STDOUT RAW] %s\n", string(line))

		var msg JSONRPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			fmt.Printf("!! Erro ao decodificar JSON-RPC: %v (Linha: %s)\n", err, string(line))
			continue
		}

		// Orquestração das mensagens recebidas
		if msg.Method != "" {
			if msg.ID != nil {
				handler.HandleRequest(msg.ID, msg.Method, msg.Params)
			} else {
				handler.HandleNotification(msg.Method, msg.Params)
			}
		} else if msg.ID != nil {
			handler.HandleResponse(msg.ID, msg.Result, msg.Error)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("!! Erro no Scanner do Listener ACP: %v\n", err)
	}
}
