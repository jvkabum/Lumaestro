package acp

import (
	"bytes"
	"encoding/json"
	"testing"
)

// MockWriteCloser simula o stdin do processo para o teste
type MockWriteCloser struct {
	bytes.Buffer
}

func (m *MockWriteCloser) Close() error {
	return nil
}

func TestSendRPC(t *testing.T) {
	e := NewACPExecutor()

	// Criamos um mock de stdin
	mockStdin := &MockWriteCloser{}

	session := &ACPSession{
		ID:    "test-session",
		Stdin: mockStdin,
	}

	testMsg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      123,
		Method:  "testMethod",
		Params:  json.RawMessage(`{"foo":"bar"}`),
	}

	// Executa o SendRPC
	err := e.SendRPC(session, testMsg)
	if err != nil {
		t.Fatalf("SendRPC falhou: %v", err)
	}

	// Verifica o output
	output := mockStdin.Bytes()

	// 1. Deve terminar com \n (ndJSON requirement)
	if len(output) == 0 || output[len(output)-1] != '\n' {
		t.Errorf("Mensagem ndJSON deve terminar com '\\n'")
	}

	// 2. Deve ser um JSON válido (removendo o \n final)
	var received JSONRPCMessage
	err = json.Unmarshal(output[:len(output)-1], &received)
	if err != nil {
		t.Fatalf("Output não é um JSON válido: %v. Output: %s", err, string(output))
	}

	// 3. Verifica integridade dos dados
	if received.Method != "testMethod" || received.JSONRPC != "2.0" {
		t.Errorf("Dados corrompidos no SendRPC. Recebido: %+v", received)
	}

	// 4. Garante que NÃO tem Content-Length (o erro que estávamos corrigindo)
	if bytes.Contains(output, []byte("Content-Length")) {
		t.Errorf("A mensagem NÃO deve conter headers 'Content-Length' no protocolo ACP do Gemini")
	}
}
