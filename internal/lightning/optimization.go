package lightning

import (
	"context"
	"fmt"
)

// Optimizer é o motor que refina os prompts dos agentes.
type Optimizer struct {
	Store        *DuckDBStore
	RewardEngine *RewardEngine
}

// PromptCandidate representa uma proposta de evolução do prompt.
type PromptCandidate struct {
	Name     string
	Content  string
	Critique string
	Accuracy float64
}

// GoldSample representa um caso de sucesso histórico para regressão.
type GoldSample struct {
	Input  string
	Output string
}

// BeamCritiquePrompt é o template para o motor APO de Elite (Beam Search).
const BeamCritiquePrompt = `
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
`

// RefinePrompt analisa as falhas e prepara os dados para o APO (Elite Version).
func (o *Optimizer) RefinePrompt(ctx context.Context, agentName string, currentPrompt string) (string, string, error) {
    // [Modificado para suportar Beam Search — Retorna o Prompt de Instrução para o LLM]
	query := `
		SELECT attributes->>'$.request_body' as prompt, 
		       attributes->>'$.response_body' as response,
			   r.reward
		FROM spans s
		JOIN rewards r ON s.rollout_id = r.rollout_id
		WHERE s.name = 'llm_call' AND r.reward < 0.3
		ORDER BY r.timestamp DESC
		LIMIT 5
	`
	rows, err := o.Store.db.Query(query)
	if err != nil { return "", "", err }
	defer rows.Close()

	var failures string
	for rows.Next() {
		var p, r string
		var reward float64
		if err := rows.Scan(&p, &r, &reward); err == nil {
			failures += fmt.Sprintf("\n[ERRO %.2f]:\nIN: %s\nOUT: %s\n", reward, p, r)
		}
	}

	if failures == "" {
		return currentPrompt, "Nenhuma falha crítica detectada.", nil
	}

	return fmt.Sprintf(BeamCritiquePrompt, failures, currentPrompt), failures, nil
}
