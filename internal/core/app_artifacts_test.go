package core

import (
	"os"
	"path/filepath"
	"testing"

	"Lumaestro/internal/config"
)

func TestArtifactReviewOperations(t *testing.T) {
	tempDir := t.TempDir()

	// Cria pasta de artefatos no workspace
	artifactsDir := filepath.Join(tempDir, ".lumaestro", "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatalf("falha ao criar pasta de artefatos: %v", err)
	}

	planContent := `# Implementation Plan

## Fase 1: Arquitetura
1. Modificar app_artifacts.go
2. Adicionar testes unitários
`
	planFile := filepath.Join(artifactsDir, "implementation_plan.md")
	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		t.Fatalf("falha ao escrever implementation_plan.md: %v", err)
	}

	app := &App{
		config: &config.Config{
			ActiveWorkspace: tempDir,
		},
	}

	// 1. Teste GetSessionArtifacts
	t.Run("GetSessionArtifacts", func(t *testing.T) {
		arts, err := app.GetSessionArtifacts("")
		if err != nil {
			t.Fatalf("GetSessionArtifacts falhou: %v", err)
		}
		if len(arts) == 0 {
			t.Fatalf("nenhum artefato encontrado")
		}

		foundPlan := false
		for _, a := range arts {
			if a.Name == "implementation_plan.md" {
				foundPlan = true
				if !a.IsPlan {
					t.Errorf("esperava IsPlan=true para implementation_plan.md")
				}
				if a.TotalLines < 5 {
					t.Errorf("total de linhas incorreto: %d", a.TotalLines)
				}
			}
		}
		if !foundPlan {
			t.Errorf("implementation_plan.md não encontrado")
		}
	})

	// 2. Teste GetLatestArtifact
	t.Run("GetLatestArtifact", func(t *testing.T) {
		latest, err := app.GetLatestArtifact()
		if err != nil {
			t.Fatalf("GetLatestArtifact falhou: %v", err)
		}
		if latest == nil || latest.Name != "implementation_plan.md" {
			t.Errorf("artefato mais recente inesperado: %+v", latest)
		}
	})

	// 3. Teste SubmitArtifactReview (Revisão com comentários)
	t.Run("SubmitArtifactReview", func(t *testing.T) {
		sub := ArtifactReviewSubmission{
			ArtifactName:    "implementation_plan.md",
			Approved:        false,
			GeneralFeedback: "Favor revisar os passos da fase 1",
			Comments: []LineComment{
				{
					LineNumber: 4,
					Comment:    "Especificar se o teste é síncrono",
				},
			},
		}

		res, err := app.SubmitArtifactReview(sub)
		if err != nil {
			t.Fatalf("SubmitArtifactReview falhou: %v", err)
		}
		_ = res
	})
}
