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

	// Teste GetAvailableSlashCommands e Skills Dinâmicas
	t.Run("GetAvailableSlashCommands e Skills", func(t *testing.T) {
		// Criar uma skill temporária no workspace: .agents/skills/data-quality/SKILL.md
		skillDir := filepath.Join(tempDir, ".agents", "skills", "data-quality")
		_ = os.MkdirAll(skillDir, 0755)
		skillMd := "---\nname: data-quality\ndescription: Validador de dados automatizado\n---\nRegras de qualidade de dados."
		_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMd), 0644)

		cmds, err := app.GetAvailableSlashCommands()
		if err != nil {
			t.Fatalf("GetAvailableSlashCommands falhou: %v", err)
		}

		foundBoost := false
		foundAgent := false
		foundSkill := false

		for _, c := range cmds {
			if c.Command == "/boost" {
				foundBoost = true
			}
			if c.Command == "/SuperAgent" {
				foundAgent = true
			}
			if c.Command == "/data-quality" {
				foundSkill = true
				if c.Type != "skill" {
					t.Errorf("Tipo esperado 'skill', obteve %s", c.Type)
				}
			}
		}

		if !foundBoost {
			t.Errorf("Comando built-in /boost nao encontrado")
		}
		if !foundAgent {
			t.Errorf("Custom Agent /SuperAgent nao encontrado nos slash commands")
		}
		if !foundSkill {
			t.Errorf("Dynamic Skill /data-quality nao encontrada nos slash commands")
		}
	})

	// Teste MCP Configuração Padronizada (Workspace & Global)
	t.Run("MCP Configuration", func(t *testing.T) {
		// Salva servidor MCP no workspace
		msg, err := app.SaveMCPServerConfig("my-postgres", "npx -y @modelcontextprotocol/server-postgres", false)
		if err != nil {
			t.Fatalf("SaveMCPServerConfig falhou: %v", err)
		}
		if msg == "" {
			t.Errorf("Mensagem vazia retornada")
		}

		// Salva servidor MCP SSE (URL) no workspace
		_, errUrl := app.SaveMCPServerConfig("remote-mcp", "https://api.example.com/sse", false)
		if errUrl != nil {
			t.Fatalf("SaveMCPServerConfig para URL falhou: %v", errUrl)
		}

		list, errList := app.GetMCPServersList()
		if errList != nil {
			t.Fatalf("GetMCPServersList falhou: %v", errList)
		}

		foundStdio := false
		foundSSE := false
		for _, s := range list {
			if s.Name == "my-postgres" {
				foundStdio = true
				if s.Type != "stdio" {
					t.Errorf("Esperava tipo stdio, obteve %s", s.Type)
				}
			}
			if s.Name == "remote-mcp" {
				foundSSE = true
				if s.Type != "sse" {
					t.Errorf("Esperava tipo sse, obteve %s", s.Type)
				}
				if s.ServerURL != "https://api.example.com/sse" {
					t.Errorf("URL incorreta: %s", s.ServerURL)
				}
			}
		}

		if !foundStdio {
			t.Errorf("Servidor MCP stdio nao encontrado na lista")
		}
		if !foundSSE {
			t.Errorf("Servidor MCP SSE nao encontrado na lista")
		}

		// Remove servidor MCP
		delMsg, errDel := app.RemoveMCPServerConfig("remote-mcp", false)
		if errDel != nil {
			t.Fatalf("RemoveMCPServerConfig falhou: %v", errDel)
		}
		if delMsg == "" {
			t.Errorf("Mensagem vazia ao deletar")
		}
	})

	// Teste Permissões Finas do Antigravity (action(target) com Deny > Ask > Allow)
	t.Run("Antigravity FineGrained Permissions", func(t *testing.T) {
		settings, err := app.GetAntigravitySettings()
		if err != nil {
			t.Fatalf("GetAntigravitySettings falhou: %v", err)
		}
		if settings == nil {
			t.Fatalf("settings retornou nil")
		}

		// Adiciona regra de Allow
		if err := app.SaveAntigravityPermissionRule("allow", "command", "git status"); err != nil {
			t.Fatalf("SaveAntigravityPermissionRule falhou: %v", err)
		}

		// Adiciona regra de Deny para comando perigoso
		if err := app.SaveAntigravityPermissionRule("deny", "command", "rm -rf /"); err != nil {
			t.Fatalf("SaveAntigravityPermissionRule deny falhou: %v", err)
		}

		// Avalia ação
		lvlAllow := app.EvaluateAntigravityAction("command", "git status")
		if lvlAllow != "allow" {
			t.Errorf("esperava 'allow' para git status, obteve '%s'", lvlAllow)
		}

		lvlDeny := app.EvaluateAntigravityAction("command", "rm -rf /")
		if lvlDeny != "deny" {
			t.Errorf("esperava 'deny' para rm -rf /, obteve '%s'", lvlDeny)
		}

		// Remove regra
		if err := app.RemoveAntigravityPermissionRule("command(rm -rf /)"); err != nil {
			t.Fatalf("RemoveAntigravityPermissionRule falhou: %v", err)
		}
	})
}
