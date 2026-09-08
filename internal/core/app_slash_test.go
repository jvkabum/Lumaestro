package core

import (
	"os"
	"path/filepath"
	"testing"

	"Lumaestro/internal/config"
)

func TestAppSlashOperations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lumaestro_core_test_*")
	if err != nil {
		t.Fatalf("Erro ao criar tempDir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Criar arquivos de teste para busca
	goFile := filepath.Join(tempDir, "main.go")
	goContent := "package main\n\nfunc SuperSpecialFunction() {\n\tprintln(\"hello\")\n}\n"
	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Erro ao escrever main.go: %v", err)
	}

	txtFile := filepath.Join(tempDir, "notes.txt")
	txtContent := "Documenting SuperSpecialFunction logic here.\nEnd of notes.\n"
	if err := os.WriteFile(txtFile, []byte(txtContent), 0644); err != nil {
		t.Fatalf("Erro ao escrever notes.txt: %v", err)
	}

	// 2. Criar agente customizado em .agents/agents/superagent/agent.md
	agentDir := filepath.Join(tempDir, ".agents", "agents", "superagent")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatalf("Erro ao criar agentDir: %v", err)
	}
	agentMd := "---\nname: SuperAgent\ndescription: Especialista em testes unitarios\n---\nVoce e um agente de testes."
	if err := os.WriteFile(filepath.Join(agentDir, "agent.md"), []byte(agentMd), 0644); err != nil {
		t.Fatalf("Erro ao escrever agent.md: %v", err)
	}

	// 3. Inicializar App com o workspace temporario
	app := &App{
		config: &config.Config{
			ActiveWorkspace: tempDir,
			Security: config.SecurityConfig{
				AllowRead:         true,
				AllowWrite:        true,
				AllowRunCommands:  false,
				FullMachineAccess: false,
				Workspaces:        []string{tempDir},
			},
		},
	}

	// Teste RunCodeSearch (Regex / Smart-case)
	t.Run("RunCodeSearch - Regex", func(t *testing.T) {
		res, err := app.RunCodeSearch("SuperSpecial.*", "", false)
		if err != nil {
			t.Fatalf("RunCodeSearch falhou: %v", err)
		}
		if res.TotalCount < 2 {
			t.Errorf("Esperava pelo menos 2 ocorrencias, obteve %d", res.TotalCount)
		}
		if len(res.Files) < 2 {
			t.Errorf("Esperava 2 arquivos com correspondencias, obteve %d", len(res.Files))
		}
	})

	// Teste RunCodeSearch (Literal com Path Filter)
	t.Run("RunCodeSearch - Literal com filtro", func(t *testing.T) {
		res, err := app.RunCodeSearch("SuperSpecialFunction", "main.go", true)
		if err != nil {
			t.Fatalf("RunCodeSearch com filtro falhou: %v", err)
		}
		if res.TotalCount != 1 {
			t.Errorf("Esperava 1 ocorrencia no main.go, obteve %d", res.TotalCount)
		}
	})

	// Teste GetCustomAgents
	t.Run("GetCustomAgents", func(t *testing.T) {
		agents, err := app.GetCustomAgents()
		if err != nil {
			t.Fatalf("GetCustomAgents falhou: %v", err)
		}
		found := false
		for _, a := range agents {
			if a.Name == "SuperAgent" {
				found = true
				if a.Description != "Especialista em testes unitarios" {
					t.Errorf("Descricao inesperada: %s", a.Description)
				}
				if a.Scope != "workspace" {
					t.Errorf("Escopo esperado 'workspace', obteve %s", a.Scope)
				}
				break
			}
		}
		if !found {
			t.Errorf("SuperAgent nao encontrado na lista: %+v", agents)
		}
	})

	// Teste GetSecurityPermissions
	t.Run("GetSecurityPermissions", func(t *testing.T) {
		sec, err := app.GetSecurityPermissions()
		if err != nil {
			t.Fatalf("GetSecurityPermissions falhou: %v", err)
		}
		if !sec.AllowRead {
			t.Errorf("Esperava AllowRead=true")
		}
		if sec.AllowRunCommands {
			t.Errorf("Esperava AllowRunCommands=false")
		}
		if len(sec.Workspaces) != 1 {
			t.Errorf("Esperava 1 workspace autorizado, obteve %d", len(sec.Workspaces))
		}
	})

	// Teste GetWorkspaceDiff
	t.Run("GetWorkspaceDiff", func(t *testing.T) {
		diff, err := app.GetWorkspaceDiff()
		if err != nil {
			t.Fatalf("GetWorkspaceDiff falhou: %v", err)
		}
		if diff == nil {
			t.Fatalf("Esperava resultado nao nulo para diff")
		}
	})
}
