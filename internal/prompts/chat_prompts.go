package prompts

// GetLMStudioSystemPrompt retorna o system prompt para o chat via LM Studio.
// Token-Optimized: sem fmt.Sprintf quando o idioma é o default.
func GetLMStudioSystemPrompt(language string) string {
	if language == "" || language == "Português do Brasil" {
		return "Assistente Lumaestro. Responda em PT-BR. Seja conciso e útil."
	}
	return "Assistente Lumaestro. Responda em " + language + ". Seja conciso e útil."
}

// GetAPODefaultPrompt retorna o prompt padrão de fallback do córtex APO (Lightning).
func GetAPODefaultPrompt() string {
	return "Maestro: assistente técnico de elite do enxame Lumaestro."
}
