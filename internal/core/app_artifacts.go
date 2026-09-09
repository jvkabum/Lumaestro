package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArtifactItem representa um artefato de documentação, plano ou código gerado por um agente.
type ArtifactItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Content    string    `json:"content"`
	ModTime    time.Time `json:"modTime"`
	IsPlan     bool      `json:"isPlan"`
	TotalLines int       `json:"totalLines"`
}

// LineComment representa uma anotação ou crítica do usuário em uma linha específica do artefato.
type LineComment struct {
	LineNumber int    `json:"lineNumber"`
	Comment    string `json:"comment"`
}

// ArtifactReviewSubmission representa a decisão do usuário (aprovação ou pedido de alterações) com anotações.
type ArtifactReviewSubmission struct {
	ArtifactName    string        `json:"artifactName"`
	Approved        bool          `json:"approved"`
	GeneralFeedback string        `json:"generalFeedback"`
	Comments        []LineComment `json:"comments"`
}

// GetSessionArtifacts retorna os artefatos encontrados no diretório brain da sessão e no workspace.
func (a *App) GetSessionArtifacts(sessionID string) ([]ArtifactItem, error) {
	var results []ArtifactItem
	seen := make(map[string]bool)

	// Diretórios candidatos a conter artefatos:
	var candidateDirs []string

	// 1. Antigravity Brain da sessão (~/.gemini/antigravity/brain/<sessionID>)
	userHome, errHome := os.UserHomeDir()
	if errHome == nil && sessionID != "" {
		candidateDirs = append(candidateDirs, filepath.Join(userHome, ".gemini", "antigravity", "brain", sessionID))
	}

	// 2. Todos os subdiretórios de brain se sessionID for vazio ou "latest"
	if errHome == nil && (sessionID == "" || sessionID == "latest") {
		brainBase := filepath.Join(userHome, ".gemini", "antigravity", "brain")
		if entries, err := os.ReadDir(brainBase); err == nil {
			// Ordena pelos mais recentes
			for _, e := range entries {
				if e.IsDir() {
					candidateDirs = append(candidateDirs, filepath.Join(brainBase, e.Name()))
				}
			}
		}
	}

	// 3. Workspace artifacts ({workspace}/.lumaestro/artifacts, {workspace}/.agents/artifacts, {workspace}/.context)
	ws := a.getActiveWorkspace()
	if ws != "" && ws != "." {
		candidateDirs = append(candidateDirs,
			filepath.Join(ws, ".lumaestro", "artifacts"),
			filepath.Join(ws, ".agents", "artifacts"),
			filepath.Join(ws, ".context"),
		)
	}

	for _, dir := range candidateDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasSuffix(strings.ToLower(name), ".md") {
				fullPath := filepath.Join(dir, name)
				if seen[fullPath] {
					continue
				}
				seen[fullPath] = true

				info, errStat := e.Info()
				if errStat != nil {
					continue
				}

				data, errRead := os.ReadFile(fullPath)
				if errRead != nil {
					continue
				}

				content := string(data)
				lines := strings.Split(content, "\n")

				isPlan := strings.Contains(strings.ToLower(name), "plan") ||
					strings.Contains(strings.ToLower(content), "implementation plan") ||
					strings.Contains(strings.ToLower(content), "plano de execução")

				results = append(results, ArtifactItem{
					ID:         filepath.Base(dir) + "_" + name,
					Name:       name,
					Path:       fullPath,
					Content:    content,
					ModTime:    info.ModTime(),
					IsPlan:     isPlan,
					TotalLines: len(lines),
				})
			}
		}
	}

	// Ordena do mais recente para o mais antigo
	sort.Slice(results, func(i, j int) bool {
		return results[i].ModTime.After(results[j].ModTime)
	})

	return results, nil
}

// GetLatestArtifact retorna o artefato mais recentemente gerado ou modificado.
func (a *App) GetLatestArtifact() (*ArtifactItem, error) {
	artifacts, err := a.GetSessionArtifacts("")
	if err != nil {
		return nil, err
	}
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("nenhum artefato encontrado")
	}
	return &artifacts[0], nil
}

// SubmitArtifactReview injeta os comentários de co-steering no motor ACP / agy ativo.
func (a *App) SubmitArtifactReview(sub ArtifactReviewSubmission) (string, error) {
	agent := "antigravity"
	if a.config != nil && a.config.GeminiModel != "" && strings.HasPrefix(a.config.GeminiModel, "claude-") {
		agent = "claude"
	}

	var sb strings.Builder
	if sub.Approved {
		sb.WriteString(fmt.Sprintf("✅ [CO-STEERING REVIEW — APROVADO: %s]\n", sub.ArtifactName))
		sb.WriteString("O usuário aprovou o plano/artefato. Prossiga imediatamente com a execução das etapas delineadas.\n")
		if sub.GeneralFeedback != "" {
			sb.WriteString(fmt.Sprintf("\nObservação adicional: %s\n", sub.GeneralFeedback))
		}
	} else {
		sb.WriteString(fmt.Sprintf("❌ [CO-STEERING REVIEW — REVISÃO SOLICITADA: %s]\n", sub.ArtifactName))
		sb.WriteString("O usuário solicitou alterações e ajustes antes de prosseguir com a execução:\n\n")

		if len(sub.Comments) > 0 {
			sb.WriteString("COMENTÁRIOS POR LINHA:\n")
			for _, c := range sub.Comments {
				sb.WriteString(fmt.Sprintf("• Linha %d: \"%s\"\n", c.LineNumber, c.Comment))
			}
			sb.WriteString("\n")
		}

		if sub.GeneralFeedback != "" {
			sb.WriteString(fmt.Sprintf("DIRETRIZ GERAL DO USUÁRIO: %s\n\n", sub.GeneralFeedback))
		}
		sb.WriteString("Por favor, atualize o artefato/plano de acordo com essas diretrizes e aguarde nova validação antes de alterar arquivos de produção.")
	}

	feedbackMsg := sb.String()

	// Envia como Steering Hint de alta prioridade para o agente ativo
	res := a.SendSteeringHint(agent, feedbackMsg)

	a.emitEvent("artifact:review_submitted", map[string]interface{}{
		"artifact": sub.ArtifactName,
		"approved": sub.Approved,
	})

	return res, nil
}
