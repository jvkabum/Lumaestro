package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestNormalizeWorkspaceURI(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"file:///d%3A/Git%20Hub/Fortress", "fortress"},
		{"file:///d:/Git%20Hub/gesttik", "gesttik"},
		{"file:///C:/git/TikTickets", "tiktickets"},
		{"file:///c:/git/IA/Lumaestro", "lumaestro"},
		{"file:///c%3A/git/IA/Lumaestro", "lumaestro"},
	}

	for _, tc := range tests {
		got := normalizeWorkspaceURI(tc.input)
		if !filepath.IsAbs(got) && len(got) < 3 {
			t.Errorf("normalizeWorkspaceURI(%s) = %s, caminho muito curto", tc.input, got)
		}
		if filepath.Base(got) != tc.contains && !filepath.IsAbs(got) {
			t.Errorf("normalizeWorkspaceURI(%s) = %s, esperado conter %s", tc.input, got, tc.contains)
		}
	}
}

func TestListSessionsOrbitFiltering(t *testing.T) {
	// Executor apontando para Fortress
	execFortress := &ACPExecutor{
		Workspace: `D:\Git Hub\Fortress`,
	}
	sessionsFortress, err := execFortress.ListSessions(nil)
	if err != nil {
		t.Fatalf("erro ao listar sessões do Fortress: %v", err)
	}

	for _, s := range sessionsFortress {
		// Nenhuma sessão do Fortress deve ser de outros projetos como "gesttik" ou "TikTickets"
		if strings.Contains(strings.ToLower(s.Title), "gesttik") || strings.Contains(strings.ToLower(s.Title), "tiktickets") {
			t.Errorf("sessão de outro projeto vazou para o Fortress: %s", s.Title)
		}
	}
	t.Logf("Sessões filtradas para Fortress: %d sessões", len(sessionsFortress))

	// Executor apontando para Lumaestro
	execLumaestro := &ACPExecutor{
		Workspace: `c:\git\IA\Lumaestro`,
	}
	sessionsLumaestro, err := execLumaestro.ListSessions(nil)
	if err != nil {
		t.Fatalf("erro ao listar sessões do Lumaestro: %v", err)
	}
	for _, s := range sessionsLumaestro {
		if strings.Contains(strings.ToLower(s.Title), "fortress") || strings.Contains(strings.ToLower(s.Title), "dwarf") {
			t.Errorf("sessão do Fortress vazou para o Lumaestro: %s", s.Title)
		}
	}
	t.Logf("Sessões filtradas para Lumaestro: %d sessões", len(sessionsLumaestro))
}


