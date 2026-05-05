package acp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// findLatestSessionID vasculha as pastas autorizadas em busca do chat JSON mais recente.
func (e *ACPExecutor) findLatestSessionID(sessionHome string) string {
	var latestFile string
	var latestTime time.Time

	cwd, _ := os.Getwd()
	userHome, _ := os.UserHomeDir()

	// 🕵️ Lista de pastas para buscar a última sessão
	searchDirs := []string{
		sessionHome, // 🎼 Prioriza o novo Hangar de Sinfonias passado pelo orchestrator
		filepath.Join(userHome, ".gemini", "tmp", "lumaestro", "chats"),
		filepath.Join(cwd, ".gemini", "tmp", "lumaestro", "chats"),
		filepath.Join(cwd, ".lumaestro"), // Busca recursiva no Hangar local
	}

	for _, root := range searchDirs {
		if _, err := os.Stat(root); err != nil {
			continue
		}

		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil { return nil }
			
			// Procuramos por arquivos .json ou .jsonl que contenham chats/history
			isChat := (strings.HasSuffix(path, ".json") || strings.HasSuffix(path, ".jsonl")) && 
				(strings.Contains(path, "chats") || strings.Contains(path, "history"))

			if !info.IsDir() && isChat {
				if info.ModTime().After(latestTime) {
					latestTime = info.ModTime()
					latestFile = path
				}
			}
			return nil
		})
	}

	if latestFile != "" {
		// Tenta ler o ID real de dentro do arquivo JSON para ser mais preciso
		data, err := os.ReadFile(latestFile)
		if err == nil {
			var meta struct {
				SessionID string `json:"sessionId"`
			}
			// Tenta decodificar a primeira linha (formato JSONL)
			firstLine := strings.Split(string(data), "\n")[0]
			if json.Unmarshal([]byte(firstLine), &meta) == nil && meta.SessionID != "" {
				return meta.SessionID
			}
		}

		// Fallback: Tenta extrair do nome do arquivo
		base := filepath.Base(latestFile)
		parts := strings.Split(strings.TrimSuffix(strings.TrimSuffix(base, ".jsonl"), ".json"), "-")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	return ""
}
