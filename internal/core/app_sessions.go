package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"Lumaestro/internal/agents/acp"
	"Lumaestro/internal/provider"
	"Lumaestro/internal/utils"
)

// ForkSession ramifica uma sessão de chat existente em uma nova linha independente de raciocínio.
func (a *App) ForkSession(agent string, sessionID string) (string, error) {
	if a.executor == nil {
		return "", fmt.Errorf("executor indisponível")
	}

	sessions, err := a.executor.ListSessions(nil)
	if err != nil {
		return "", fmt.Errorf("erro ao listar sessões: %w", err)
	}

	var targetSession *acp.SessionInfo
	if sessionID != "" {
		for i := range sessions {
			if sessions[i].SessionID == sessionID {
				targetSession = &sessions[i]
				break
			}
		}
	} else if len(sessions) > 0 {
		targetSession = &sessions[0]
	}

	newID := uuid.New().String()
	newTitle := "Fork da Sessão"

	if targetSession != nil && targetSession.File != "" {
		origPath := targetSession.File
		if targetSession.Title != "" {
			newTitle = "Fork: " + targetSession.Title
		}

		data, errRead := os.ReadFile(origPath)
		if errRead == nil {
			dir := filepath.Dir(origPath)
			ext := filepath.Ext(origPath)
			if ext == "" {
				ext = ".jsonl"
			}
			newPath := filepath.Join(dir, newID+ext)

			content := string(data)
			if strings.HasSuffix(ext, ".json") {
				content = strings.Replace(content, targetSession.SessionID, newID, 1)
			} else if strings.HasSuffix(ext, ".jsonl") {
				lines := strings.Split(content, "\n")
				if len(lines) > 0 {
					lines[0] = strings.Replace(lines[0], targetSession.SessionID, newID, 1)
					content = strings.Join(lines, "\n")
				}
			}

			_ = os.WriteFile(newPath, []byte(content), 0644)
			fmt.Printf("[Sinfonias] 🌿 Fork criado com sucesso: %s -> %s\n", origPath, newPath)
		}
	}

	// Persiste o título da nova sessão
	_ = acp.SaveSessionTitle(newID, newTitle)

	// Persiste como última sessão do workspace
	ws := a.getActiveWorkspace()
	if ws != "" && ws != "." {
		lastSessionPath := filepath.Join(ws, ".lumaestro", "last_session.json")
		_ = os.WriteFile(lastSessionPath, []byte(fmt.Sprintf(`{"sessionId":"%s"}`, newID)), 0644)
	}

	// Notifica a UI do novo fork criado
	utils.SafeEmit(a.ctx, "session:forked", map[string]string{
		"oldSessionId": sessionID,
		"newSessionId": newID,
		"title":        newTitle,
	})

	return newID, nil
}

// RenameSession permite ao usuário renomear uma Sinfonia manualmente.
func (a *App) RenameSession(sessionID string, newTitle string) error {
	trimmed := strings.TrimSpace(newTitle)
	if trimmed == "" {
		return fmt.Errorf("o título não pode ser vazio")
	}

	if err := acp.SaveSessionTitle(sessionID, trimmed); err != nil {
		return err
	}

	fmt.Printf("[Sinfonias] ✏️ Sessão %s renomeada para: '%s'\n", sessionID, trimmed)

	// Avisa o frontend imediatamente
	utils.SafeEmit(a.ctx, "session:renamed", map[string]string{
		"sessionId": sessionID,
		"title":     trimmed,
	})

	return nil
}

// AutoNameSession gera um título inteligente usando a IA para uma Sinfonia existente.
func (a *App) AutoNameSession(sessionID string) (string, error) {
	if a.executor == nil {
		return "", fmt.Errorf("executor indisponível")
	}

	// 1. Localiza a sessão na lista de arquivos
	sessions, err := a.executor.ListSessions(nil)
	if err != nil {
		return "", err
	}

	var targetSession *acp.SessionInfo
	for i := range sessions {
		if sessions[i].SessionID == sessionID {
			targetSession = &sessions[i]
			break
		}
	}

	// 2. Extrai o primeiro prompt do usuário
	userPrompt := ""
	if targetSession != nil && targetSession.File != "" {
		userPrompt = acp.ExtractFirstUserMessage(targetSession.File)
	}

	if userPrompt == "" {
		// Tenta encontrar o título atual como base
		if targetSession != nil && targetSession.Title != "" && !strings.HasPrefix(targetSession.Title, "session-20") {
			userPrompt = targetSession.Title
		}
	}

	if userPrompt == "" {
		return "", fmt.Errorf("nenhuma mensagem de usuário encontrada para sintetizar título")
	}

	// 3. Gera o título usando a IA
	title, err := a.generateTitleWithAI(userPrompt)
	if err != nil || title == "" {
		title = acp.CleanTitleCandidate(userPrompt)
	}

	if title == "" {
		title = "Sinfonia " + sessionID[:min(8, len(sessionID))]
	}

	// 4. Salva e notifica
	if err := acp.SaveSessionTitle(sessionID, title); err != nil {
		return title, err
	}

	fmt.Printf("[Sinfonias] ✨ IA batizou a sessão %s como: '%s'\n", sessionID, title)

	utils.SafeEmit(a.ctx, "session:renamed", map[string]string{
		"sessionId": sessionID,
		"title":     title,
	})

	return title, nil
}

// generateTitleWithAI utiliza o modelo mais rápido disponível para sintetizar um título.
func (a *App) generateTitleWithAI(userPrompt string) (string, error) {
	baseCtx := a.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, 8*time.Second)
	defer cancel()

	systemInstruction := "Você é um gerador de títulos concisos para conversas. Dado o início de uma conversa, crie um título de 3 a 5 palavras em português que resuma o assunto central. Retorne ESTRITAMENTE o título, sem aspas, sem pontuação final, sem emojis, sem preâmbulo."
	fullPrompt := fmt.Sprintf("%s\n\nConversa do usuário: %s", systemInstruction, userPrompt)

	// 1. Provedor Groq (ultra rápido: ~200-400ms)
	if a.config != nil && a.config.GetActiveGroqKey() != "" {
		groq := provider.NewGroqProvider(a.config.GetActiveGroqKey(), a.config.GroqModel)
		if text, err := groq.GenerateText(ctx, fullPrompt); err == nil && strings.TrimSpace(text) != "" {
			return cleanAITitle(text), nil
		}
	}

	// 2. Provedor Google Gemini (Fast-Track)
	if a.config != nil && a.config.GetActiveGeminiKey() != "" {
		if gemini, err := provider.NewGoogleProvider(ctx, a.config.GetActiveGeminiKey()); err == nil {
			if text, err := gemini.GenerateText(ctx, fullPrompt); err == nil && strings.TrimSpace(text) != "" {
				return cleanAITitle(text), nil
			}
		}
	}

	// 3. Fallback para heurística limpa
	return acp.CleanTitleCandidate(userPrompt), nil
}

func cleanAITitle(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, `"'“”)`+"`")
	cleaned = strings.TrimPrefix(cleaned, "Título:")
	cleaned = strings.TrimPrefix(cleaned, "Titulo:")
	cleaned = strings.TrimPrefix(cleaned, "Title:")
	cleaned = strings.TrimSpace(cleaned)
	cleaned = strings.TrimSuffix(cleaned, ".")
	return acp.CleanTitleCandidate(cleaned)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
