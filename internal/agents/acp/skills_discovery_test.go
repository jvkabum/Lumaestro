package acp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverSkills(t *testing.T) {
	tempDir := t.TempDir()

	// Cria estrutura de skills de workspace: .agents/skills/my-skill/SKILL.md
	skillDir := filepath.Join(tempDir, ".agents", "skills", "data-analyzer")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("falha ao criar pasta: %v", err)
	}

	skillContent := `---
name: data-analyzer
description: Executa analise de dados automatizada e relatorios
---

# Data Analyzer Instructions
Siga estas etapas para analisar dados:
1. Verifique o schema
2. Valide as tabelas
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillContent), 0644); err != nil {
		t.Fatalf("falha ao escrever SKILL.md: %v", err)
	}

	skills, err := DiscoverSkills(tempDir)
	if err != nil {
		t.Fatalf("erro ao descobrir skills: %v", err)
	}

	found := false
	for _, s := range skills {
		if s.Name == "data-analyzer" {
			found = true
			if s.Command != "/data-analyzer" {
				t.Errorf("comando esperado /data-analyzer, obteve %s", s.Command)
			}
			if s.Scope != "workspace" {
				t.Errorf("escopo esperado workspace, obteve %s", s.Scope)
			}
			if s.Description != "Executa analise de dados automatizada e relatorios" {
				t.Errorf("descricao incorreta: %s", s.Description)
			}
			break
		}
	}

	if !found {
		t.Errorf("skill data-analyzer nao foi descoberta no workspace")
	}
}
