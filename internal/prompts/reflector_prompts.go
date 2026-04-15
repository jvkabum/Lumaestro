package prompts

import "fmt"

// GetReflectorPrompt retorna o prompt de auto-reflexão para análise de qualidade de interações.
func GetReflectorPrompt(query, response string) string {
	return fmt.Sprintf(`Como especialista em qualidade de agentes de IA, analise a interação abaixo.
Pergunta: %s
Resposta: %s

O orquestrador foi preciso? Responda no formato:
SUCCESS: true/false
LEARNING: {descrição de como melhorar o contexto ou a estratégia}
NEW_SKILL: {se necessário, descreva uma nova regra de busca ou comportamento}`, query, response)
}
