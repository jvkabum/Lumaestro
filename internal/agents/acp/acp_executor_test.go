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
	e := NewACPExecutor(".", ".")

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

func TestAntigravitySendInput(t *testing.T) {
	e := NewACPExecutor(".", ".")
	mockStdin := &MockWriteCloser{}

	session := &ACPSession{
		ID:            "gemini",
		AgentName:     "gemini",
		Stdin:         mockStdin,
		IsAntigravity: true,
		ACPSessID:     "", // O Antigravity não requer ACPSessID prévio!
	}
	e.ActiveSessions["gemini"] = session

	err := e.SendInput("gemini", "Olá Antigravity", nil)
	if err != nil {
		t.Fatalf("SendInput para Antigravity falhou: %v", err)
	}

	output := mockStdin.Bytes()
	if len(output) == 0 || output[len(output)-1] != '\n' {
		t.Errorf("Mensagem Antigravity deve terminar com '\\n'")
	}

	var parsed struct {
		Event   string `json:"event"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(output[:len(output)-1], &parsed); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}
	if parsed.Event != "user" || parsed.Message.Content != "Olá Antigravity" {
		t.Errorf("Conteúdo inesperado do evento: %+v", parsed)
	}
}

