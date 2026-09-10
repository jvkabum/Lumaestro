package db

import (
	"time"

	"gorm.io/gorm"
)

// AiQuotaRepository lida com a persistência de estados das chaves de IA (SQLite / GORM)
type AiQuotaRepository struct {
	db *gorm.DB
}

// NewAiQuotaRepository inicializa o repositório de cotas
func NewAiQuotaRepository(db *gorm.DB) *AiQuotaRepository {
	return &AiQuotaRepository{db: db}
}

// GetQuota retorna a cota de uma chave+modelo específica (cria registro se não existir)
func (r *AiQuotaRepository) GetQuota(keyID string) (*AiQuota, error) {
	if r == nil || r.db == nil {
		return nil, gorm.ErrInvalidDB
	}
	var quota AiQuota
	err := r.db.Where("key_id = ?", keyID).FirstOrCreate(&quota, AiQuota{
		KeyID:        keyID,
		FailureCount: 0,
		BackoffCount: 0,
	}).Error
	return &quota, err
}

// MarkTemporaryExhaustion define um cooldown temporário (RPM / 429) e incrementa o backoff
func (r *AiQuotaRepository) MarkTemporaryExhaustion(keyID string, delay time.Duration) error {
	if r == nil || r.db == nil {
		return nil
	}
	var quota AiQuota
	err := r.db.Where("key_id = ?", keyID).FirstOrCreate(&quota, AiQuota{KeyID: keyID}).Error
	if err != nil {
		return err
	}

	until := time.Now().Add(delay)
	newCount := quota.BackoffCount + 1

	return r.db.Model(&quota).Updates(map[string]interface{}{
		"temp_exhausted_until": until,
		"backoff_count":        newCount,
	}).Error
}

// MarkDailyExhaustion define que a cota da chave estourou para o dia todo (RPD)
func (r *AiQuotaRepository) MarkDailyExhaustion(keyID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	var quota AiQuota
	err := r.db.Where("key_id = ?", keyID).FirstOrCreate(&quota, AiQuota{KeyID: keyID}).Error
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.Model(&quota).Updates(map[string]interface{}{
		"exhausted_at": now,
	}).Error
}

// ClearTemporaryExhaustion limpa o cooldown temporário quando o tempo expira
func (r *AiQuotaRepository) ClearTemporaryExhaustion(keyID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Model(&AiQuota{}).Where("key_id = ?", keyID).Updates(map[string]interface{}{
		"temp_exhausted_until": nil,
	}).Error
}

// MarkFailure registra uma falha para detecção de instabilidade (Circuit Breaker)
func (r *AiQuotaRepository) MarkFailure(keyID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	var quota AiQuota
	err := r.db.Where("key_id = ?", keyID).FirstOrCreate(&quota, AiQuota{KeyID: keyID}).Error
	if err != nil {
		return err
	}

	now := time.Now()
	return r.db.Model(&quota).Updates(map[string]interface{}{
		"failure_count": quota.FailureCount + 1,
		"last_failure":  now,
	}).Error
}

// MarkSuccess reseta falhas e zera cooldowns após execução bem-sucedida
func (r *AiQuotaRepository) MarkSuccess(keyID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Model(&AiQuota{}).Where("key_id = ?", keyID).Updates(map[string]interface{}{
		"failure_count":        0,
		"last_failure":         nil,
		"temp_exhausted_until": nil,
	}).Error
}

// ResetDailyQuotas limpa o esgotamento diário de todas as chaves
func (r *AiQuotaRepository) ResetDailyQuotas() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Model(&AiQuota{}).Where("1 = 1").Updates(map[string]interface{}{
		"exhausted_at":  nil,
		"failure_count": 0,
		"backoff_count": 0,
	}).Error
}

// GetShortestPendingCooldown calcula o menor tempo restante entre os cooldowns das chaves informadas
func (r *AiQuotaRepository) GetShortestPendingCooldown(keyIDs []string) time.Duration {
	if r == nil || r.db == nil || len(keyIDs) == 0 {
		return 30 * time.Second
	}

	var quotas []AiQuota
	err := r.db.Where("key_id IN ? AND temp_exhausted_until > ?", keyIDs, time.Now()).Find(&quotas).Error
	if err != nil || len(quotas) == 0 {
		return 15 * time.Second
	}

	now := time.Now()
	var minDuration time.Duration = 0

	for _, q := range quotas {
		if q.TempExhaustedUntil != nil && q.TempExhaustedUntil.After(now) {
			diff := q.TempExhaustedUntil.Sub(now)
			if minDuration == 0 || diff < minDuration {
				minDuration = diff
			}
		}
	}

	if minDuration <= 0 {
		return 10 * time.Second
	}
	return minDuration
}
