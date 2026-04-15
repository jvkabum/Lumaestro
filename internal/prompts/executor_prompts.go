package prompts

import "fmt"

// GetOneShotRAGPrompt retorna o template de contexto para execução one-shot via CLI.
func GetOneShotRAGPrompt(contextData, query string) string {
	return fmt.Sprintf("CONTEXTO DO OBSIDIAN:\n%s\n\nPERGUNTA DO USUÁRIO:\n%s", contextData, query)
}
