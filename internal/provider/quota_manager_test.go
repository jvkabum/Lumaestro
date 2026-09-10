package provider

import (
	"errors"
	"testing"

	"Lumaestro/internal/db"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *db.AiQuotaRepository {
	d, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("falha ao abrir sqlite em memoria: %v", err)
	}

	err = d.AutoMigrate(&db.AiQuota{})
	if err != nil {
		t.Fatalf("falha ao migrar AiQuota: %v", err)
	}

	return db.NewAiQuotaRepository(d)
}

func TestQuotaManager_SuspendedKeys(t *testing.T) {
	repo := setupTestDB(t)
	keys := []string{"AIzaSyTestKey1", "AIzaSyTestKey2"}
	qm := NewQuotaManager(keys, repo)

	if qm.IsSuspended(keys[0]) {
		t.Errorf("chave nao deveria estar suspensa inicialmente")
	}

	// Simula erro 403 / PERMISSION_DENIED
	permErr := errors.New("googleapi: Error 403: The caller does not have permission, PERMISSION_DENIED")
	qm.HandleError(keys[0], 0, "gemini-2.5-flash", permErr)

	if !qm.IsSuspended(keys[0]) {
		t.Errorf("chave deveria ter sido marcada como suspensa")
	}

	if !qm.IsExhausted(keys[0], "gemini-2.5-flash") {
		t.Errorf("chave suspensa deve ser considerada exaurida/bloqueada")
	}

	if qm.IsSuspended(keys[1]) {
		t.Errorf("chave 2 nao deveria ter sido suspensa")
	}
}

func TestQuotaManager_RPM_ExponentialBackoff(t *testing.T) {
	repo := setupTestDB(t)
	keys := []string{"AIzaSyTestKeyRPM"}
	qm := NewQuotaManager(keys, repo)
	model := "gemini-2.5-flash"

	if qm.IsExhausted(keys[0], model) {
		t.Errorf("chave nao deveria estar em cooldown inicialmente")
	}

	// Simula erro 429 RPM
	err429 := errors.New("googleapi: Error 429: Resource has been exhausted (e.g. check quota), RESOURCE_EXHAUSTED")
	qm.HandleError(keys[0], 0, model, err429)

	if !qm.IsExhausted(keys[0], model) {
		t.Errorf("chave deveria estar em cooldown apos erro 429")
	}

	// Simula sucesso posterior que zera o cooldown
	qm.MarkSuccess(keys[0], model)

	if qm.IsExhausted(keys[0], model) {
		t.Errorf("chave deveria estar liberada apos MarkSuccess")
	}
}

func TestQuotaManager_RPD_DailyExhaustion(t *testing.T) {
	repo := setupTestDB(t)
	keys := []string{"AIzaSyTestKeyRPD"}
	qm := NewQuotaManager(keys, repo)
	model := "gemini-2.5-flash"

	// Simula erro de limite diario
	dailyErr := errors.New("ResourceExhausted: Quota exceeded for quota metric 'Requests per day'")
	qm.HandleError(keys[0], 0, model, dailyErr)

	if !qm.IsExhausted(keys[0], model) {
		t.Errorf("chave deveria estar bloqueada por esgotamento diario")
	}
}

func TestQuotaManager_CircuitBreaker(t *testing.T) {
	repo := setupTestDB(t)
	keys := []string{"AIzaSyTestKeyCircuit"}
	qm := NewQuotaManager(keys, repo)
	model := "gemini-2.5-flash"

	genericErr := errors.New("500 internal server error")

	// 1 falha
	qm.HandleError(keys[0], 0, model, genericErr)
	if qm.IsExhausted(keys[0], model) {
		t.Errorf("nao deveria bloquear com apenas 1 falha")
	}

	// 2 falhas
	qm.HandleError(keys[0], 0, model, genericErr)
	if qm.IsExhausted(keys[0], model) {
		t.Errorf("nao deveria bloquear com 2 falhas")
	}

	// 3 falhas -> Dispara Circuit Breaker (quarentena de 15 min)
	qm.HandleError(keys[0], 0, model, genericErr)
	if !qm.IsExhausted(keys[0], model) {
		t.Errorf("deveria bloquear com 3 falhas consecutivas (Circuit Breaker)")
	}

	// Sucesso reseta o contador
	qm.MarkSuccess(keys[0], model)
	if qm.IsExhausted(keys[0], model) {
		t.Errorf("deveria ter sido desbloqueado apos MarkSuccess")
	}
}

func TestQuotaManager_RequestLogs(t *testing.T) {
	qm := NewQuotaManager([]string{"AIzaSy1234567890"}, nil)

	for i := 0; i < 110; i++ {
		qm.LogRequest("gemini-2.5-flash", 0, "AIzaSy1234567890", "SUCCESS", "")
	}

	logs := qm.GetRecentLogs()
	if len(logs) != 100 {
		t.Errorf("esperado 100 logs no buffer deslizante, obtido %d", len(logs))
	}

	if logs[0].KeyID != "AIza...7890" {
		t.Errorf("esperado mascaramento da chave 'AIza...7890', obtido '%s'", logs[0].KeyID)
	}
}
