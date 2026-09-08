package acp

import (
	"strings"

	"Lumaestro/internal/config"
	"Lumaestro/internal/orchestration"
	"github.com/google/uuid"
	"Lumaestro/internal/utils"
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
	h.Session.ReportTurnCost(h.Executor)
}

// ReportTurnCost calcula e registra o custo fixo por turno e telemetria de tokens.
func (s *ACPSession) ReportTurnCost(exec *ACPExecutor) {
	if s.AgentID != uuid.Nil {
		pt := s.LastPromptTokens
		ct := s.LastCandidatesTokens
		
		// Fallback para valores padrão caso a telemetria tenha falhado
		if pt == 0 { pt = 500 }
		if ct == 0 { ct = 200 }

		cfg, _ := config.Load()
		modelName := "gemini-3-flash-preview"
		if cfg != nil && cfg.GeminiModel != "" {
			modelName = cfg.GeminiModel
		}

		// Preços (Estimativas): 
		// Flash: in ~$0.000000075 / out ~$0.0000003
		// Pro: in ~$0.0000035 / out ~$0.0000105
		var costUSD float64
		if strings.Contains(strings.ToLower(modelName), "pro") {
			costUSD = float64(pt)*0.0000035 + float64(ct)*0.0000105
		} else {
			costUSD = float64(pt)*0.000000075 + float64(ct)*0.0000003
		}

		// RegistrarCusto espera costCents em INT
		costCents := int(costUSD * 100)
		
		_ = orchestration.RegistrarCusto(s.AgentID, s.CurrentIssueID, "google", modelName, pt, ct, costCents)
		
		// 📈 Acumular economia de cache
		s.TotalCacheTokens += s.LastCacheTokens

		// Emitir evento de telemetria para o Dashboard (Wails)
		if exec != nil && exec.Ctx != nil {
			utils.SafeEmit(exec.Ctx, "agent:tokens", map[string]interface{}{
				"agent":          s.AgentName,
				"prompt":         s.LastPromptTokens,
				"candidates":     s.LastCandidatesTokens,
				"cacheCurrent":   s.LastCacheTokens,
				"cacheTotal":     s.TotalCacheTokens,
				"costCentsTotal": costCents,
			})
		}

		// Reseta para o próximo turno
		s.LastPromptTokens = 0
		s.LastCandidatesTokens = 0
		s.LastCacheTokens = 0
	}
}
