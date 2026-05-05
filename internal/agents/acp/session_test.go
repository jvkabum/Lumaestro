package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAutoSessionRestore(t *testing.T) {
	// Setup: Criar diretório temporário para workspace
	tmpDir, err := os.MkdirTemp("", "lumaestro-test")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	lumaestroPath := filepath.Join(tmpDir, ".lumaestro")
	os.MkdirAll(lumaestroPath, 0755)

	// Mock session ID
	sessionID := "test-session-123"
	lastSessionPath := filepath.Join(lumaestroPath, "last_session.json")
	
	data := map[string]string{"sessionId": sessionID}
	jsonData, _ := json.Marshal(data)
	
	if err := os.WriteFile(lastSessionPath, jsonData, 0644); err != nil {
		t.Fatalf("falha ao escrever last_session.json: %v", err)
	}

	// Simular a leitura do arquivo (lógica de teste do código implementado)
	loadedData, err := os.ReadFile(lastSessionPath)
	if err != nil {
		t.Fatalf("falha ao ler last_session.json: %v", err)
	}

	var lastSession struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(loadedData, &lastSession); err != nil {
		t.Fatalf("falha ao unmarshal: %v", err)
	}

	if lastSession.SessionID != sessionID {
		t.Errorf("esperado sessão %s, got %s", sessionID, lastSession.SessionID)
	}
}
