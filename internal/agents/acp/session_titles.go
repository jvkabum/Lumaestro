package acp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	titlesMu       sync.RWMutex
	cleanTagRegex  = regexp.MustCompile(`\[(IDIOMA|CPI|AMNÉSIA|AUTONOMIA|NAV-3D|CENSURA-ID)[^\]]*\]`)
	targetGoalRegex = regexp.MustCompile(`(?i)OBJETIVO:\s*`)
)

func getTitlesFilePath() string {
	cwd, _ := os.Getwd()
	dir := filepath.Join(cwd, ".lumaestro", "cache")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "session_titles.json")
}

// LoadSessionTitles carrega o mapa de títulos personalizados gravados.
func LoadSessionTitles() map[string]string {
	titlesMu.RLock()
	defer titlesMu.RUnlock()

	titles := make(map[string]string)
	data, err := os.ReadFile(getTitlesFilePath())
	if err == nil {
		_ = json.Unmarshal(data, &titles)
	}
	return titles
}

// SaveSessionTitle persiste um título personalizado para uma sessão.
func SaveSessionTitle(sessionID string, title string) error {
	titlesMu.Lock()
	defer titlesMu.Unlock()

	titles := make(map[string]string)
	path := getTitlesFilePath()
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &titles)
	}

	trimmed := strings.TrimSpace(title)
	if trimmed != "" {
		titles[sessionID] = trimmed
	} else {
		delete(titles, sessionID)
	}

	out, err := json.MarshalIndent(titles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// CleanTitleCandidate higieniza um texto bruto de prompt para virar título legível.
func CleanTitleCandidate(text string) string {
	cleaned := cleanTagRegex.ReplaceAllString(text, "")
	cleaned = targetGoalRegex.ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, "Analista de tarefas complexas e geração de planos técnicos.", "")
	cleaned = strings.ReplaceAll(cleaned, "Especialista em código, arquitetura e diagnósticos.", "")
	cleaned = strings.TrimSpace(cleaned)

	// Se começar com aspas, remove
	cleaned = strings.Trim(cleaned, `"'“”)`)
	cleaned = strings.TrimSpace(cleaned)

	if cleaned == "" {
		return ""
	}

	// Limita tamanho
	runes := []rune(cleaned)
	if len(runes) > 42 {
		cleaned = string(runes[:39]) + "..."
	}

	// Capitaliza primeira letra
	if len(cleaned) > 0 {
		cleaned = strings.ToUpper(cleaned[:1]) + cleaned[1:]
	}

	return cleaned
}

// ExtractFirstUserMessage extrai a primeira mensagem real do usuário dentro do arquivo de sessão.
func ExtractFirstUserMessage(filePath string) string {
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Limite de buffer para linhas longas
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lineCount := 0
	for scanner.Scan() {
		lineCount++
		if lineCount > 40 {
			break // Não lê arquivos gigantes inteiros
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Tenta descompactar como objeto genérico
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		// 1. Formato direto {"type":"user", "content": [...]}
		if rawType, ok := raw["type"].(string); ok && (rawType == "user" || rawType == "human") {
			if txt := parseContentField(raw["content"]); txt != "" {
				if cleaned := CleanTitleCandidate(txt); cleaned != "" {
					return cleaned
				}
			}
		}

		// 2. Formato de delta {"$set":{"messages":[...]}}
		if setObj, ok := raw["$set"].(map[string]interface{}); ok {
			if msgs, ok := setObj["messages"].([]interface{}); ok {
				for _, m := range msgs {
					if msgMap, ok := m.(map[string]interface{}); ok {
						if mType, _ := msgMap["type"].(string); mType == "user" || mType == "human" {
							if txt := parseContentField(msgMap["content"]); txt != "" {
								if cleaned := CleanTitleCandidate(txt); cleaned != "" {
									return cleaned
								}
							}
						}
					}
				}
			}
		}

		// 3. Formato array direto {"messages":[...]}
		if msgs, ok := raw["messages"].([]interface{}); ok {
			for _, m := range msgs {
				if msgMap, ok := m.(map[string]interface{}); ok {
					role, _ := msgMap["role"].(string)
					mType, _ := msgMap["type"].(string)
					if role == "user" || mType == "user" {
						if txt := parseContentField(msgMap["content"]); txt != "" {
							if cleaned := CleanTitleCandidate(txt); cleaned != "" {
								return cleaned
							}
						}
					}
				}
			}
		}
	}

	return ""
}

func parseContentField(content interface{}) string {
	if content == nil {
		return ""
	}
	if str, ok := content.(string); ok {
		if strings.Contains(str, "<session_context>") {
			return ""
		}
		return str
	}
	if arr, ok := content.([]interface{}); ok {
		for _, item := range arr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if text, ok := itemMap["text"].(string); ok {
					if strings.Contains(text, "<session_context>") {
						continue
					}
					return text
				}
			}
		}
	}
	return ""
}

// ResolveSessionTitle define o melhor título legível para uma sessão.
func ResolveSessionTitle(sessionID string, filename string, filePath string, modTime time.Time) string {
	titles := LoadSessionTitles()

	// 1. Título customizado explicitamente salvo no cache
	if t, ok := titles[sessionID]; ok && t != "" {
		return t
	}
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	if t, ok := titles[baseName]; ok && t != "" {
		return t
	}

	// 2. Extrair primeira mensagem do usuário do arquivo físico
	if filePath != "" {
		if firstMsg := ExtractFirstUserMessage(filePath); firstMsg != "" {
			return firstMsg
		}
	}

	// 3. Fallback amigável com data/hora em vez de hash cru
	if !modTime.IsZero() {
		return "Sinfonia de " + modTime.Format("02/01 às 15:04")
	}

	return "Sinfonia sem título"
}
