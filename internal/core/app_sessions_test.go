package core

import (
	"os"
	"path/filepath"
	"testing"

	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/config"
)

func TestSessionBranchingAndRenaming(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Cria diretório de sessões simulado
	historyDir := filepath.Join(tempDir, ".lumaestro", "sinfonias", "gemini", "history", "lumaestro")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		t.Fatalf("falha ao criar pasta de histórico: %v", err)
	}

	sessionFile := filepath.Join(historyDir, "orig-sess.jsonl")
	sessData := `{"id":"orig-sess","title":"Sessão Original","createdAt":"2026-09-08T20:00:00Z"}
{"role":"user","text":"olá mundo"}
`
	if err := os.WriteFile(sessionFile, []byte(sessData), 0644); err != nil {
		t.Fatalf("falha ao escrever arquivo de sessão: %v", err)
	}

	executor := acp.NewACPExecutor(tempDir, "")
	executor.Workspace = tempDir

	app := &App{
		executor: executor,
		config: &config.Config{
			ActiveWorkspace: tempDir,
		},
	}

	// 2. Teste RenameSession
	t.Run("RenameSession", func(t *testing.T) {
		err := app.RenameSession("orig-sess", "Meu Novo Titulo")
		if err != nil {
			t.Fatalf("RenameSession falhou: %v", err)
		}
		titles := acp.LoadSessionTitles()
		if titles["orig-sess"] != "Meu Novo Titulo" {
			t.Errorf("Título esperado 'Meu Novo Titulo', obteve '%s'", titles["orig-sess"])
		}
	})

	// 3. Teste ForkSession
	t.Run("ForkSession", func(t *testing.T) {
		newID, err := app.ForkSession("antigravity", "orig-sess")
		if err != nil {
			t.Fatalf("ForkSession falhou: %v", err)
		}
		if newID == "" || newID == "orig-sess" {
			t.Fatalf("ID do fork inválido: %s", newID)
		}

		// Verifica se o novo arquivo de sessão foi criado no disco
		newFile := filepath.Join(historyDir, newID+".jsonl")
		if _, errStat := os.Stat(newFile); os.IsNotExist(errStat) {
			t.Errorf("Arquivo do fork não foi criado no disco: %s", newFile)
		}

		// Verifica título do fork
		titles := acp.LoadSessionTitles()
		if titles[newID] == "" {
			t.Errorf("Título do fork não foi persistido")
		}
	})
}
