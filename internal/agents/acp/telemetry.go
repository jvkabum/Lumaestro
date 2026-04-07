package acp

import (
	"strings"

	"Lumaestro/internal/orchestration"
	"github.com/google/uuid"
)

// emitReward emite um bônus ou penalidade técnica autônoma (Sistema Lightning).
func (h *ACPRpcHandler) emitReward(details string, err error) {
	go func() {
		if h.Executor.LStore == nil || h.Executor.RewardEngine == nil { return }
		
		lowerCmd := strings.ToLower(details)
		isTest := strings.Contains(lowerCmd, "test") || strings.Contains(lowerCmd, "build") || strings.Contains(lowerCmd, "compile")
		
		if err == nil {
			if isTest {
				h.Executor.RewardEngine.EmitReward(h.Session.RolloutID, h.Session.AttemptID, 0.5, "technical_success_auto", map[string]interface{}{ "cmd": details })
			}
		} else {
			h.Executor.RewardEngine.EmitReward(h.Session.RolloutID, h.Session.AttemptID, -0.5, "technical_failure_auto", map[string]interface{}{ "cmd": details, "err": err.Error() })
		}
	}()
}

// logNetworkActivity registra silenciosamente a atividade de rede silenciosa (NetworkLogger).
func (h *ACPRpcHandler) logNetworkActivity() {
	if h.Executor.NetLog != nil {
		h.Executor.NetLog.LogRequest()
	}
}

// reportTurnCost calcula e registra o custo fixo por turno enquanto o CLI não expõe usage.usage.
func (h *ACPRpcHandler) reportTurnCost() {
	if h.Session.AgentID != uuid.Nil {
		_ = orchestration.RegistrarCusto(h.Session.AgentID, h.Session.CurrentIssueID, "google", "gemini-1.5-flash", 800, 400, 2)
	}
}
