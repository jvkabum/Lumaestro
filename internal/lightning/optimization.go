package lightning

import (
	"context"
	"fmt"

	"Lumaestro/internal/prompts"
)

// Optimizer é o motor que refina os prompts dos agentes.
type Optimizer struct {
	Store        *DuckDBStore
	RewardEngine *RewardEngine
}

// NewOptimizer inicializa o motor de refinamento APO.
func NewOptimizer(store *DuckDBStore, re *RewardEngine) *Optimizer {
	return &Optimizer{Store: store, RewardEngine: re}
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

	return prompts.GetBeamCritiquePrompt(failures, currentPrompt), failures, nil
}
