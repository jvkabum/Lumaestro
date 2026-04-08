package orchestration

import (
	"Lumaestro/internal/config"
	"Lumaestro/internal/db"
	"log"
	"time"

	"github.com/google/uuid"
)

// StartHeartbeatDaemon inicia o daemon invísivel que varre o banco em busca de agentes idles para acordar.
func StartHeartbeatDaemon(onWakeUp func(agent db.Agent, runID uuid.UUID)) {
	go func() {
		log.Println("[Heartbeat] Daemon inicializado. Monitores cardíacos ativos.")
		ticker := time.NewTicker(30 * time.Second) // Varre a cada 30 segundos
		defer ticker.Stop()

		for range ticker.C {
			sweepAndWake(onWakeUp)
		}
	}()
}

func sweepAndWake(onWakeUp func(agent db.Agent, runID uuid.UUID)) {
	// Carrega config para pegar o limite de concorrência local
	cfg, errConfig := config.Load()
	maxConcurrent := 3 // Limite de Sobrevivência de Queda (Fallback)
	if errConfig == nil && cfg != nil && cfg.MaxConcurrentAgents > 0 {
		maxConcurrent = cfg.MaxConcurrentAgents
	}

	// 1. Checar Capacidade Atual do Enxame
	var running int64
	db.InstanceDB.Model(&db.Agent{}).Where("status = ?", "running").Count(&running)

	if int(running) >= maxConcurrent {
		// Log.Printf("[Heartbeat] Enxame Ocupado (%d/%d)... Silêncio na rede.", running, maxConcurrent)
		return
	}

	availableSlots := maxConcurrent - int(running)

	var eligibleAgents []db.Agent

	// 2. Busca agentes que estão IDLE e que NÃO estouraram orcamento
	err := db.InstanceDB.
		Where("status = ?", "idle").
		Where("budget_monthly_cents = 0 OR spent_monthly_cents < budget_monthly_cents").
		Order("last_heartbeat_at ASC"). // Acorda quem dorme a mais tempo (Justiça de Fila)
		Limit(availableSlots).
		Find(&eligibleAgents).Error

	if err != nil {
		log.Printf("[Heartbeat] Falha na varredura: %v\n", err)
		return
	}

	for _, agent := range eligibleAgents {
		// Registra formalmente a Batida (HeartbeatRun) no Banco para auditoria
		run := db.HeartbeatRun{
			AgentID:          agent.ID,
			InvocationSource: "scheduler",
			Status:           "running", // Já coloca como running pra travar
		}
		
		now := db.Timestamp{Time: time.Now()}
		run.StartedAt = &now
		db.InstanceDB.Create(&run)

		// Trava ele temporariamente para 'running' (Mutually Exclusive Heartbeat)
		agent.Status = "running"
		agent.LastHeartbeatAt = now
		db.InstanceDB.Save(&agent)

		// Executa a pulsação em paralelo para não travar o loop
		go onWakeUp(agent, run.ID)
	}
}

// FinalizeHeartbeat é exportado agora para que o App mestre posssa chamar ao fim do processamento do Motor
func FinalizeHeartbeat(agentID uuid.UUID, runID uuid.UUID, success bool, executionError string) {
	var run db.HeartbeatRun
	db.InstanceDB.First(&run, "id = ?", runID)
	
	if success {
		run.Status = "succeeded"
	} else {
		run.Status = "failed"
		run.Error = executionError
	}

	now := db.Timestamp{Time: time.Now()}
	run.FinishedAt = &now
	db.InstanceDB.Save(&run)

	var agent db.Agent
	if err := db.InstanceDB.First(&agent, "id = ?", agentID).Error; err == nil {
		agent.Status = "idle"
		db.InstanceDB.Save(&agent)
	}
}
