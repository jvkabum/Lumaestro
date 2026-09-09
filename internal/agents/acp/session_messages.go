package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cleanUserRequestPrompt limpa os wrappers de injeção de sistema (como <USER_REQUEST>, OBJETIVO:, etc.)
// retornando estritamente a pergunta original digitada pelo usuário.
func cleanUserRequestPrompt(raw string) string {
	s := strings.TrimSpace(raw)

	// Corta metadados adicionais se presentes
	if idx := strings.Index(s, "<ADDITIONAL_METADATA>"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	if idx := strings.Index(s, "<USER_SETTINGS_CHANGE>"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}

	// 1. Se contiver OBJETIVO:, extrai o texto do objetivo real
	if idx := strings.Index(s, "OBJETIVO:"); idx != -1 {
		after := s[idx+len("OBJETIVO:"):]
		if end := strings.Index(after, "</USER_REQUEST>"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
		return strings.TrimSpace(after)
	}

	// 2. Se for uma tag <USER_REQUEST> direta
	if strings.Contains(s, "<USER_REQUEST>") {
		s = strings.ReplaceAll(s, "<USER_REQUEST>", "")
		if end := strings.Index(s, "</USER_REQUEST>"); end != -1 {
			s = s[:end]
		}
		return strings.TrimSpace(s)
	}

	return s
}

// parseAntigravityTranscript lê o arquivo transcript.jsonl do Antigravity CLI e reconstrói as mensagens cronológicas.
func parseAntigravityTranscript(filePath string) ([]ChatMessage, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 20*1024*1024)

	var messages []ChatMessage
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry struct {
			StepIndex int    `json:"step_index"`
			Source    string `json:"source"`
			Type      string `json:"type"`
			Content   string `json:"content"`
			Thinking  string `json:"thinking"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Type == "USER_INPUT" {
			text := cleanUserRequestPrompt(entry.Content)
			if text != "" {
				messages = append(messages, ChatMessage{
					Role:  "user",
					Text:  text,
					Agent: "user",
				})
			}
		} else if entry.Type == "PLANNER_RESPONSE" {
			if strings.TrimSpace(entry.Content) != "" {
				messages = append(messages, ChatMessage{
					Role:       "assistant",
					Text:       entry.Content,
					Thought:    entry.Thinking,
					Agent:      "Antigravity",
					IsPlanning: true,
				})
			}
		}
	}

	return messages, nil
}

// parseLegacySessionFile processa arquivos .jsonl/.json do Gemini CLI legado.
func parseLegacySessionFile(filePath string) ([]ChatMessage, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var messages []ChatMessage
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}

		role, _ := m["role"].(string)
		text, _ := m["text"].(string)
		if role == "" {
			if t, ok := m["type"].(string); ok {
				if t == "human" || t == "user" {
					role = "user"
				} else if t == "assistant" || t == "model" {
					role = "assistant"
				}
			}
		}
		if text == "" {
			if c, ok := m["content"].(string); ok {
				text = c
			}
		}

		if (role == "user" || role == "assistant") && text != "" {
			messages = append(messages, ChatMessage{
				Role:  role,
				Text:  text,
				Agent: role,
			})
		}
	}
	return messages, nil
}

// GetSessionMessages recupera o histórico cronológico de mensagens de uma sessão/sinfonia.
func (e *ACPExecutor) GetSessionMessages(sessionID string) ([]ChatMessage, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID não pode ser vazio")
	}

	userHome, _ := os.UserHomeDir()
	candidateTranscripts := []string{
		filepath.Join(userHome, ".gemini", "antigravity-cli", "brain", sessionID, ".system_generated", "logs", "transcript.jsonl"),
		filepath.Join(userHome, ".gemini", "antigravity", "brain", sessionID, ".system_generated", "logs", "transcript.jsonl"),
		filepath.Join(e.Workspace, ".lumaestro", "brain", sessionID, ".system_generated", "logs", "transcript.jsonl"),
		filepath.Join(e.Workspace, ".lumaestro", "sinfonias", "gemini", "brain", sessionID, ".system_generated", "logs", "transcript.jsonl"),
	}

	for _, p := range candidateTranscripts {
		if _, err := os.Stat(p); err == nil {
			msgs, errParse := parseAntigravityTranscript(p)
			if errParse == nil && len(msgs) > 0 {
				fmt.Printf("[Sinfonias] 📜 Restauradas %d mensagens da sessão %s a partir de: %s\n", len(msgs), sessionID, p)
				return msgs, nil
			}
		}
	}

	// Fallback para arquivos de checkpoints legacy (.jsonl/.json)
	sessions, _ := e.ListSessions(nil)
	for _, s := range sessions {
		if s.SessionID == sessionID && s.File != "" && !strings.HasSuffix(s.File, ".db") {
			if _, err := os.Stat(s.File); err == nil {
				msgs, errParse := parseLegacySessionFile(s.File)
				if errParse == nil && len(msgs) > 0 {
					fmt.Printf("[Sinfonias] 📜 Restauradas %d mensagens da sessão legacy %s a partir de: %s\n", len(msgs), sessionID, s.File)
					return msgs, nil
				}
			}
		}
	}

	return []ChatMessage{}, nil
}
