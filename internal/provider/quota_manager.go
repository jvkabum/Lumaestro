package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"Lumaestro/internal/db"
	"Lumaestro/internal/utils"
)

// RequestLog armazena o log de uma requisição de IA para auditoria em tempo real
type RequestLog struct {
	Timestamp    time.Time `json:"timestamp"`
	ModelName    string    `json:"model_name"`
	KeyIndex     int       `json:"key_index"`
	KeyID        string    `json:"key_id"` // masked key
	Status       string    `json:"status"` // "SUCCESS" or "FAILED"
	ErrorMessage string    `json:"error_message,omitempty"`
}

// QuotaManager gerencia o controle de tráfego centralizado das API Keys (SQLite + RAM)
type QuotaManager struct {
	apiKeys       []string
	repo          *db.AiQuotaRepository
	requestLogs   []RequestLog
	logMu         sync.Mutex
	suspendedKeys map[string]bool
	suspMu        sync.RWMutex
}

// NewQuotaManager inicializa um gerenciador de cotas e resiliência
func NewQuotaManager(apiKeys []string, repo *db.AiQuotaRepository) *QuotaManager {
	return &QuotaManager{
		apiKeys:       apiKeys,
		repo:          repo,
		requestLogs:   make([]RequestLog, 0),
		suspendedKeys: make(map[string]bool),
	}
}

func (qm *QuotaManager) SetApiKeys(keys []string) {
	qm.apiKeys = keys
}

func (qm *QuotaManager) GetApiKeys() []string {
	return qm.apiKeys
}

func (qm *QuotaManager) MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
}

func (qm *QuotaManager) cooldownKey(apiKey string, modelName string) string {
	h := sha256.Sum256([]byte(apiKey))
	shortHash := hex.EncodeToString(h[:8])
	return fmt.Sprintf("%s_%s", shortHash, modelName)
}

// IsSuspended verifica se uma chave foi marcada como suspensa/inválida
func (qm *QuotaManager) IsSuspended(apiKey string) bool {
	qm.suspMu.RLock()
	defer qm.suspMu.RUnlock()
	return qm.suspendedKeys[apiKey]
}

// MarkSuspended isola permanentemente uma chave suspensa na memória
func (qm *QuotaManager) MarkSuspended(apiKey string, keyIdx int) {
	qm.suspMu.Lock()
	qm.suspendedKeys[apiKey] = true
	qm.suspMu.Unlock()

	masked := qm.MaskAPIKey(apiKey)
	fmt.Printf("🚫 [QuotaManager] Chave [%d (%s)] SUSPENSA / DESATIVADA (403/PERMISSION_DENIED). Bloqueando chave.\n", keyIdx+1, masked)
}

// IsExhausted verifica se uma chave+modelo está bloqueada por cota, backoff ou instabilidade
func (qm *QuotaManager) IsExhausted(apiKey string, modelName string) bool {
	// 1. Verificação instantânea de chave suspensa
	if qm.IsSuspended(apiKey) {
		return true
	}

	if qm.repo == nil {
		return false
	}

	key := qm.cooldownKey(apiKey, modelName)
	q, err := qm.repo.GetQuota(key)
	if err != nil {
		return false
	}

	// 2. Esgotamento Diário (RPD)
	if q.ExhaustedAt != nil {
		return true
	}

	// 3. Cooldown Temporário (Backoff Exponencial RPM)
	if q.TempExhaustedUntil != nil {
		if time.Now().Before(*q.TempExhaustedUntil) {
			return true
		}
		// Expirou, limpa no banco
		_ = qm.repo.ClearTemporaryExhaustion(key)
	}

	// 4. Circuit Breaker de Instabilidade (3 falhas consecutivas)
	if q.FailureCount >= 3 {
		if q.LastFailure != nil && time.Since(*q.LastFailure) < 15*time.Minute {
			return true
		}
	}

	return false
}

// MarkExhausted aplica backoff exponencial progressivo (RPM) ou bloqueio diário (RPD)
func (qm *QuotaManager) MarkExhausted(apiKey string, keyIdx int, modelName string, isDaily bool) {
	key := qm.cooldownKey(apiKey, modelName)
	masked := qm.MaskAPIKey(apiKey)

	if isDaily {
		if qm.repo != nil {
			_ = qm.repo.MarkDailyExhaustion(key)
		}
		fmt.Printf("☠️ [QuotaManager] Chave [%d (%s)] esgotou o limite DIÁRIO (RPD) para o modelo %s. Desativada hoje.\n", keyIdx+1, masked, modelName)
		return
	}

	// Calcula backoff exponencial: 30s * 2^(tentativas-1) até 960s
	backoffCount := 1
	if qm.repo != nil {
		if q, err := qm.repo.GetQuota(key); err == nil && q != nil {
			backoffCount = q.BackoffCount + 1
		}
	}

	multiplier := 1
	for i := 1; i < backoffCount && i < 6; i++ {
		multiplier *= 2
	}

	backoffSeconds := 30 * multiplier
	if backoffSeconds > 960 {
		backoffSeconds = 960
	}

	if qm.repo != nil {
		_ = qm.repo.MarkTemporaryExhaustion(key, time.Duration(backoffSeconds)*time.Second)
	}

	fmt.Printf("💸 [QuotaManager] Chave [%d (%s)] esgotou RPM para %s. Cooldown exponencial de %ds (Tentativa %d)\n", keyIdx+1, masked, modelName, backoffSeconds, backoffCount)
}

// MarkFailure registra uma falha não ligada à cota para detecção de instabilidade
func (qm *QuotaManager) MarkFailure(apiKey string, keyIdx int, modelName string, err error) {
	key := qm.cooldownKey(apiKey, modelName)
	masked := qm.MaskAPIKey(apiKey)

	if qm.repo != nil {
		_ = qm.repo.MarkFailure(key)
		if q, dbErr := qm.repo.GetQuota(key); dbErr == nil && q != nil && q.FailureCount >= 3 {
			fmt.Printf("💥 [QuotaManager] Circuit Breaker ativado! Chave [%d (%s)] instável (%d falhas seguidas no modelo %s). Quarentena de 15m.\n", keyIdx+1, masked, q.FailureCount, modelName)
			return
		}
	}

	fmt.Printf("⚠️ [QuotaManager] Falha na chave [%d (%s)] para %s: %v\n", keyIdx+1, masked, modelName, err)
}

// MarkSuccess reseta as falhas e zera cooldowns do par chave+modelo
func (qm *QuotaManager) MarkSuccess(apiKey string, modelName string) {
	key := qm.cooldownKey(apiKey, modelName)
	if qm.repo != nil {
		_ = qm.repo.MarkSuccess(key)
	}
}

// HandleError processa e roteia o erro para o tratamento adequado de cota, suspensão ou falha
func (qm *QuotaManager) HandleError(apiKey string, keyIdx int, modelName string, err error) {
	if err == nil {
		return
	}

	if utils.IsSuspendedError(err) {
		qm.MarkSuspended(apiKey, keyIdx)
		key := qm.cooldownKey(apiKey, modelName)
		if qm.repo != nil {
			_ = qm.repo.MarkDailyExhaustion(key)
		}
		return
	}

	if utils.IsQuotaError(err) {
		errStr := strings.ToLower(err.Error())
		isDaily := strings.Contains(errStr, "per day") || strings.Contains(errStr, "daily") || strings.Contains(errStr, "rpd")
		qm.MarkExhausted(apiKey, keyIdx, modelName, isDaily)
		return
	}

	qm.MarkFailure(apiKey, keyIdx, modelName, err)
}

// GetAvailableKeys retorna as chaves prontas para uso imediato em um modelo
func (qm *QuotaManager) GetAvailableKeys(modelName string) (availableKeys []string, availableIdxs []int) {
	for idx, key := range qm.apiKeys {
		if !qm.IsExhausted(key, modelName) {
			availableKeys = append(availableKeys, key)
			availableIdxs = append(availableIdxs, idx)
		}
	}
	return availableKeys, availableIdxs
}

// GetShortestCooldown calcula o menor tempo restante entre os cooldowns pendentes da frota
func (qm *QuotaManager) GetShortestCooldown(models []string) time.Duration {
	if qm.repo == nil || len(qm.apiKeys) == 0 {
		return 30 * time.Second
	}

	var keyIDs []string
	for _, m := range models {
		for _, k := range qm.apiKeys {
			keyIDs = append(keyIDs, qm.cooldownKey(k, m))
		}
	}

	return qm.repo.GetShortestPendingCooldown(keyIDs)
}

// LogRequest armazena a requisição no buffer circular de auditoria (últimos 100 requests)
func (qm *QuotaManager) LogRequest(modelName string, keyIdx int, apiKey string, status string, errMsg string) {
	qm.logMu.Lock()
	defer qm.logMu.Unlock()

	qm.requestLogs = append(qm.requestLogs, RequestLog{
		Timestamp:    time.Now(),
		ModelName:    modelName,
		KeyIndex:     keyIdx,
		KeyID:        qm.MaskAPIKey(apiKey),
		Status:       status,
		ErrorMessage: errMsg,
	})

	if len(qm.requestLogs) > 100 {
		qm.requestLogs = qm.requestLogs[1:]
	}
}

// GetRecentLogs retorna uma cópia dos logs recentes em ordem cronológica inversa
func (qm *QuotaManager) GetRecentLogs() []RequestLog {
	qm.logMu.Lock()
	defer qm.logMu.Unlock()

	logs := make([]RequestLog, len(qm.requestLogs))
	copy(logs, qm.requestLogs)

	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs
}

var (
	globalQuotaManager *QuotaManager
	globalQuotaMu      sync.Mutex
)

// GetGlobalQuotaManager retorna ou inicializa o QuotaManager global compartilhado
func GetGlobalQuotaManager(apiKeys []string) *QuotaManager {
	globalQuotaMu.Lock()
	defer globalQuotaMu.Unlock()

	if globalQuotaManager != nil {
		if len(apiKeys) > 0 {
			globalQuotaManager.SetApiKeys(apiKeys)
		}
		return globalQuotaManager
	}

	var repo *db.AiQuotaRepository
	if db.InstanceDB != nil {
		repo = db.NewAiQuotaRepository(db.InstanceDB)
	}

	globalQuotaManager = NewQuotaManager(apiKeys, repo)
	return globalQuotaManager
}
