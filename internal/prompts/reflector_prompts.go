package prompts

import "fmt"

// GetReflectorPrompt retorna o prompt de auto-reflexão para análise de qualidade de interações.
// Token-Optimized: formato estruturado compacto, sem prosa decorativa.
func GetReflectorPrompt(query, response string) string {
	return fmt.Sprintf(`Analise a qualidade desta interação de agente IA.
Q: %s
R: %s
Formato de resposta:
SUCCESS: true/false
LEARNING: {como melhorar contexto/estratégia}
NEW_SKILL: {nova regra se necessário}`, query, response)
}
