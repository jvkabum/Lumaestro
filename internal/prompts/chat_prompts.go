package prompts

import "fmt"

// GetLMStudioSystemPrompt retorna o system prompt para o chat via LM Studio.
func GetLMStudioSystemPrompt(language string) string {
	if language == "" {
		language = "Português do Brasil"
	}
	return fmt.Sprintf("You are a powerful AI assistant integrated into Lumaestro. Answer in %s. Be concise and helpful.", language)
}

// GetAPODefaultPrompt retorna o prompt padrão de fallback do córtex APO (Lightning).
func GetAPODefaultPrompt() string {
	return "Você é o Maestro, um assistente técnico de elite."
}
