package lightning

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RewardDimension representa uma única dimensão de uma recompensa multidimensional.
type RewardDimension struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// RewardEngine gerencia a atribuição de dopamina digital aos agentes.
type RewardEngine struct {
	Store *DuckDBStore
}

// NewRewardEngine cria um novo motor de recompensas.
func NewRewardEngine(store *DuckDBStore) *RewardEngine {
	return &RewardEngine{Store: store}
}

// EmitReward gera um span de recompensa no DuckDB.
func (e *RewardEngine) EmitReward(rolloutID string, attemptID string, value float64, name string, metadata map[string]interface{}) error {
	now := GetNowTimestamp()
	
	// Criar o Span de Recompensa (Seguindo semântica AGL)
	span := Span{
		RolloutID:  rolloutID,
		AttemptID:  attemptID,
		SequenceID: 999, // Recompensas geralmente são o fim ou eventos especiais
		TraceID:    uuid.NewString(),
		SpanID:     uuid.NewString(),
		Name:       "agentlightning.reward",
		StartTime:  now,
		Status:     TraceStatus{StatusCode: "OK"},
		Attributes: map[string]interface{}{
			"reward_name":  name,
			"reward_value": value,
			"metadata":     metadata,
		},
	}

	// Salvar no DuckDB
	if err := e.Store.InsertSpan(span); err != nil {
		return fmt.Errorf("erro ao persistir recompensa: %w", err)
	}

	// Também registramos na tabela de resumo de rewards para consultas analíticas rápidas
	query := `INSERT INTO rewards (rollout_id, reward, timestamp, source) VALUES (?, ?, ?, ?)`
	_, err := e.Store.db.Exec(query, rolloutID, value, now, name)
	
	return err
}

// CalculateSessionSuccess analisa o histórico de uma sessão e retorna um score de 0 a 1.
func (e *RewardEngine) CalculateSessionSuccess(rolloutID string) (float64, error) {
	var total float64
	var count int
	
	query := `SELECT reward FROM rewards WHERE rollout_id = ?`
	rows, err := e.Store.db.Query(query, rolloutID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var val float64
		if err := rows.Scan(&val); err != nil {
			return 0, err
		}
		total += val
		count++
	}

	if count == 0 {
		return 0, nil
	}

	return total / float64(count), nil
}
