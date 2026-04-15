package prompts

import "fmt"

// GetConflictValidatorPrompt retorna o prompt do Agente Validador de Verdade (resolver conflitos no grafo).
func GetConflictValidatorPrompt(oldFact, newFact, contextStr string) string {
	return fmt.Sprintf(`Você é o Agente Validador de Verdade.
Detectamos um conflito no Grafo de Conhecimento.

FATO ANTIGO: %s
FATO NOVO: %s
CONTEXTO RECENTE: %s

Sua tarefa:
Responda APENAS "UPDATE" se o Fato Novo for claramente uma atualização ou correção válida.
Responda APENAS "CONFLICT" se houver dúvida real.

Decisão:`, oldFact, newFact, contextStr)
}

// GetBeamCritiquePrompt retorna o template Beam Search do motor APO de Elite.
func GetBeamCritiquePrompt(failures, currentPrompt string) string {
	return fmt.Sprintf(`
Você é o Arquiteto Metacognitivo do Enxame Lumaestro.
Sua tarefa é analisar falhas de um agente e propor 3 VARIANTES de System Prompt diferentes (Beam Search).

---
FALHAS ANALISADAS:
%s
---
PROMPT ATUAL:
%s
---

INSTRUÇÕES:
Gere 3 propostas distintas, cada uma com uma "Personalidade" clara:
1. "O Rigoroso": Focado em regras estritas, tipos e validações.
2. "O Eficiente": Focado em concisão, velocidade e economia de tokens.
3. "O Criativo": Focado em resolução de problemas complexos e pensamento lateral.

FORMATO DE RESPOSTA (OBRIGATÓRIO):
<variants>
  <variant name="O Rigoroso">
    <critique>Por que esta versão é melhor para este erro...</critique>
    <prompt>O texto completo do novo prompt...</prompt>
  </variant>
  ... (repetir para as outras 2)
</variants>
`, failures, currentPrompt)
}

// GetSwarmAgentSystemPrompt retorna o system prompt para dados de fine-tuning RLHF.
func GetSwarmAgentSystemPrompt(agentName string) string {
	return "Você é o agente " + agentName + " do enxame Lumaestro."
}

// GetSwarmCommandPrompt retorna o system prompt para comandos diretos ao enxame.
func GetSwarmCommandPrompt() string {
	return "Você é o Maestro do enxame Lumaestro. Responda à ordem do Comandante de forma executiva."
}

// GetLMStudioTestSystemPrompt retorna o system prompt para teste de capacidade do modelo LM Studio.
func GetLMStudioTestSystemPrompt() string {
	return `You are a JSON API. You MUST respond ONLY with a valid JSON object, no prose, no markdown code blocks, no extra text. The JSON must have exactly these keys: "status" (string "ok"), "capability" (string describing what you can do in one sentence), "language" (string, the language of this prompt).`
}

// GetLMStudioTestUserPrompt retorna a mensagem de usuário para teste de capacidade.
func GetLMStudioTestUserPrompt() string {
	return "Respond in JSON format as instructed by the system prompt."
}
