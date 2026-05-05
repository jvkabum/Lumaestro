package prompts

import "fmt"

// GetConflictValidatorPrompt retorna o prompt do Agente Validador de Verdade (resolver conflitos no grafo).
// Token-Optimized: formato estruturado compacto, instruções telegráficas.
func GetConflictValidatorPrompt(oldFact, newFact, contextStr string) string {
	return fmt.Sprintf(`Conflito no Grafo de Conhecimento.
ANTIGO: %s
NOVO: %s
CONTEXTO: %s
Responda APENAS "UPDATE" (fato novo é correção válida) ou "CONFLICT" (dúvida real).`, oldFact, newFact, contextStr)
}

// GetBeamCritiquePrompt retorna o template Beam Search do motor APO de Elite.
// Token-Optimized: instruções compactas, formato XML preservado (parsing depende dele).
func GetBeamCritiquePrompt(failures, currentPrompt string) string {
	return fmt.Sprintf(`Arquiteto Metacognitivo: analise falhas e proponha 3 variantes de System Prompt (Beam Search).

FALHAS:
%s
PROMPT ATUAL:
%s

Gere 3 propostas com personalidades distintas:
1. "O Rigoroso": regras estritas, validações.
2. "O Eficiente": concisão, economia de tokens.
3. "O Criativo": pensamento lateral, resolução complexa.

FORMATO (OBRIGATÓRIO):
<variants>
  <variant name="O Rigoroso">
    <critique>Por que esta versão é melhor...</critique>
    <prompt>Texto do novo prompt...</prompt>
  </variant>
  ... (repetir para as outras 2)
</variants>`, failures, currentPrompt)
}

// GetSwarmAgentSystemPrompt retorna o system prompt para dados de fine-tuning RLHF.
func GetSwarmAgentSystemPrompt(agentName string) string {
	return "Agente " + agentName + " do enxame Lumaestro."
}

// GetSwarmCommandPrompt retorna o system prompt para comandos diretos ao enxame.
func GetSwarmCommandPrompt() string {
	return "Maestro do enxame Lumaestro. Responda à ordem do Comandante de forma executiva."
}

// GetLMStudioTestSystemPrompt retorna o system prompt para teste de capacidade do modelo LM Studio.
// NOTA: Este prompt DEVE permanecer em inglês — é usado para testar compliance JSON do modelo.
func GetLMStudioTestSystemPrompt() string {
	return `JSON API. Respond ONLY with valid JSON, no prose/markdown. Keys: "status" ("ok"), "capability" (one sentence), "language" (language of this prompt).`
}

// GetLMStudioTestUserPrompt retorna a mensagem de usuário para teste de capacidade.
func GetLMStudioTestUserPrompt() string {
	return "Respond in JSON as instructed."
}
