package core

import (
	"Lumaestro/internal/lightning"
	"Lumaestro/internal/prompts"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// --- ⚡ MÓDULO LIGHTNING (DASHBOARD ANALÍTICO) ---

// GetLightningStats retorna estatísticas calculadas pelo DuckDB para o Dashboard.
func (a *App) GetLightningStats() map[string]interface{} {
	stats := make(map[string]interface{})
	if a.LStore == nil {
		return map[string]interface{}{"status": "offline"}
	}

	// 1. Total de Rollouts
	var totalRollouts int64
	a.LStore.GetDB().QueryRow("SELECT count(*) FROM rollouts").Scan(&totalRollouts)
	stats["total_rollouts"] = totalRollouts

	// 2. Média de Recompensa
	var avgReward float64
	a.LStore.GetDB().QueryRow("SELECT avg(reward) FROM rewards").Scan(&avgReward)
	stats["avg_reward"] = avgReward

	// 💸 3. Métricas Financeiras (Tokens e Custo Estimado)
	var pTokens, cTokens int64
	a.LStore.GetDB().QueryRow("SELECT sum(prompt_tokens), sum(completion_tokens) FROM spans").Scan(&pTokens, &cTokens)
	stats["prompt_tokens"] = pTokens
	stats["completion_tokens"] = cTokens
	
	// Estimativa: $0.15/1M in, $0.60/1M out (Gemini Flash)
	totalUSD := (float64(pTokens)*0.15 + float64(cTokens)*0.60) / 1000000.0
	stats["total_cost_usd"] = totalUSD

	// 3. Status do Proxy
	stats["status"] = "online"
	
	return stats
}

// TriggerReflection destila o conhecimento de um rollout específico no Obsidian.
func (a *App) TriggerReflection(rolloutID string) string {
	if a.LReflector == nil {
		return "Erro: Motor de Reflexão não inicializado."
	}
	err := a.LReflector.DistillLesson(rolloutID)
	if err != nil {
		return "Erro na reflexão: " + err.Error()
	}
	return "Sucesso: Lição destilada no Obsidian Vault!"
}

// startAPOWorker monitora o desempenho do enxame e sugere otimizações (APO).
func (a *App) startAPOWorker() {
	go func() {
		ctx := a.ctx // 🛡️ Ancoragem de segurança
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if a.LStore == nil || a.LOptimizer == nil {
					continue
				}

				// 1. Identificar agentes con "Dopamina Crítica" (Média < -0.2 nos últimos rollouts)
				var agentIDStr string
				var avgReward float64
				err := a.LStore.GetDB().QueryRow(`
				SELECT agent_name, avg(reward) as avg_r 
				FROM rewards r
				JOIN spans s ON r.rollout_id = s.rollout_id
				GROUP BY agent_name 
				HAVING avg_r < -0.2
				LIMIT 1
			`).Scan(&agentIDStr, &avgReward)

				if err == nil && agentIDStr != "" {
					fmt.Printf("[🧠 APO Cortex] Desempenho crítico para %s (RR: %.2f). Iniciando Evolução...\n", agentIDStr, avgReward)

					// 2. Obter o prompt atual (do config ou do DB)
					currentPrompt := prompts.GetAPODefaultPrompt()
					if latest, err := a.LStore.GetLatestPrompt(agentIDStr); err == nil && latest != "" {
						currentPrompt = latest
					}

					// 3. Gerar a Crítica APO
					criticInput, failures, err := a.LOptimizer.RefinePrompt(ctx, agentIDStr, currentPrompt)
					if err != nil || failures == "Nenhuma falha crítica detectada." {
						continue
					}

					// 4. Chamar o LLM para gerar o FEIXE de 3 candidatos (Com Resiliência Automática)
					fmt.Println("[🧠 APO Beam] Gerando 3 variantes de evolução estratégica com Escudo de Resiliência...")
					beamOutput, provider, err := a.LRouter.ExecuteWithFallback(ctx, "", criticInput)
					if err == nil && beamOutput != "" {
						fmt.Printf("[🕵️ RESILIÊNCIA] Variantes geradas via: %s\n", provider)
						// 5. Novo: Loop de Regressão Gold
						goldSamples, _ := a.LStore.GetGoldSamples(agentIDStr)

						re := regexp.MustCompile(`(?s)<variant name="([^"]+)">\s*<critique>(.*?)</critique>\s*<prompt>(.*?)</prompt>\s*</variant>`)
						matches := re.FindAllStringSubmatch(beamOutput, -1)

						for _, m := range matches {
							name, critique, content := m[1], m[2], m[3]

							// Calcular Acurácia contra os "Gold Samples"
							accuracy := 100.0
							if len(goldSamples) > 0 {
								hits := 0
								for _, gs := range goldSamples {
									// Executa o novo prompt contra o input de ouro (Com Fallback)
									fmt.Printf("[🕵️ TEST] Validando variante '%s' contra Caso de Ouro (Manto Ativo)...\n", name)
									testOutput, _, err := a.LRouter.ExecuteWithFallback(ctx, content, gs["input"])
									if err == nil && strings.Contains(strings.ToLower(testOutput), strings.ToLower(gs["output"])) {
										hits++
									}
								}
								accuracy = (float64(hits) / float64(len(goldSamples))) * 100.0
							}

							a.LStore.InsertCandidate(agentIDStr, name, content, critique, accuracy)
						}

						if len(matches) > 0 {
							fmt.Printf("[⭐ BEAM SUCCESS] %d candidatos validados (Gold Check) para %s!\n", len(matches), agentIDStr)
							a.emitEvent("lightning:beam_ready", agentIDStr)
						}
					}
				}
			}
		}
	}()
}

// GetLatestSpans retorna os últimos traces analíticos do DuckDB para o Dashboard.
func (a *App) GetLatestSpans() []map[string]interface{} {
	if a.LStore == nil { return []map[string]interface{}{} }
	
	rows, err := a.LStore.GetDB().Query(`
		SELECT rollout_id, attempt_id, name, prompt_tokens + completion_tokens as tokens, start_time,
		       attributes->>'$.image_path' as media
		FROM spans 
		ORDER BY start_time DESC 
		LIMIT 10
	`)
	if err != nil {
		fmt.Printf("[App] Erro ao buscar spans: %v\n", err)
		return []map[string]interface{}{}
	}
	defer rows.Close()

	var spans []map[string]interface{}
	for rows.Next() {
		var rid, aid, name string
		var tokens int
		var startTime float64
		var media sql.NullString
		rows.Scan(&rid, &aid, &name, &tokens, &startTime, &media)
		
		spans = append(spans, map[string]interface{}{
			"id": rid,
			"agent": aid,
			"op": name,
			"usage": tokens,
			"media": media.String,
			"time": time.Unix(int64(startTime), 0).Format("15:04:05"),
		})
	}
	return spans
}

// ExportTelemetry exporta os traces do DuckDB para um arquivo JSON estruturado.
func (a *App) ExportTelemetry() string {
	if a.LStore == nil { return "⚠️ Motor analítico offline." }
	path := "lumaestro_telemetry_export.json"
	err := lightning.ExportSpansToJSON(a.LStore, path)
	if err != nil { return "🔴 Erro na exportação: " + err.Error() }
	return "✅ Telemetria de Elite exportada para: " + path
}

// GetPromptHistory retorna o histórico de evolução de prompts de um agente.
func (a *App) GetPromptHistory(agentName string) []map[string]interface{} {
	if a.LStore == nil { return nil }
	rows, err := a.LStore.GetDB().Query(`
		SELECT content, avg_reward, created_at 
		FROM prompts 
		WHERE agent_name = ? 
		ORDER BY created_at DESC 
		LIMIT 10`, agentName)
	if err != nil { return nil }
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var content string
		var reward, createdAt float64
		if err := rows.Scan(&content, &reward, &createdAt); err == nil {
			history = append(history, map[string]interface{}{
				"content": content,
				"reward": reward,
				"date": time.Unix(int64(createdAt), 0).Format("02/01 15:04"),
			})
		}
	}
	return history
}

// GetPromptCandidates retorna os candidatos aguardando aprovação.
func (a *App) GetPromptCandidates() []map[string]interface{} {
	if a.LStore == nil { return nil }
	cands, _ := a.LStore.GetPendingCandidates()
	return cands
}

// ApprovePromptVariant aprova um candidato e o torna o prompt oficial.
func (a *App) ApprovePromptVariant(candidateID string) string {
	if a.LStore == nil { return "🔴 Motor analítico offline." }
	err := a.LStore.ApproveCandidate(candidateID)
	if err != nil { return "🔴 Erro na aprovação: " + err.Error() }
	return "✅ Variante aprovada com sucesso! Evolução concluída."
}

// AddGoldSample registra manualmente uma interação perfeita como referência de regressão.
func (a *App) AddGoldSample(agentName, input, output string) string {
	if a.LStore == nil { return "🔴 Motor analítico offline." }
	err := a.LStore.InsertGoldSample(agentName, input, output)
	if err != nil { return "🔴 Erro ao salvar: " + err.Error() }
	return "💎 Caso de Ouro registrado! O enxame usará isso para validar futuras evoluções."
}

// ExportRLHFDataset gera um arquivo JSONL com conversas perfeitas para Fine-tuning.
func (a *App) ExportRLHFDataset() string {
	if a.LStore == nil { return "⚠️ Motor analítico offline." }
	path := "dataset_lumaestro_rlhf.jsonl"
	
	file, err := os.Create(path)
	if err != nil { return "🔴 Erro ao criar arquivo: " + err.Error() }
	defer file.Close()

	// 1. Exportar Gold Samples como SFT (Conversational)
	rows, _ := a.LStore.GetDB().Query(`SELECT agent_name, input, output FROM gold_samples`)
	if rows != nil {
		defer rows.Close()

		count := 0
		for rows.Next() {
			var agent, in, out string
			rows.Scan(&agent, &in, &out)

			entry := map[string]interface{}{
				"messages": []map[string]string{
					{"role": "system", "content": prompts.GetSwarmAgentSystemPrompt(agent)},
					{"role": "user", "content": in},
					{"role": "assistant", "content": out},
				},
			}
			data, _ := json.Marshal(entry)
			file.Write(data)
			file.Write([]byte("\n"))
			count++
		}
		return fmt.Sprintf("✅ Fábrica RLHF: %d exemplos de elite exportados para: %s", count, path)
	}

	return "⚠️ Nenhum Caso de Ouro encontrado para exportação."
}

// SendMessageToSwarm permite ao Comandante intervir diretamente no enxame via Dashboard.
func (a *App) SendMessageToSwarm(agentName, message string) string {
	if a.LRouter == nil { return "🔴 Roteador offline." }
	
	fmt.Printf("[🕵️‍♂️ COMANDO] Enviando ordem para %s: %s\n", agentName, message)
	
	// Executa a ordem com resiliência total
	response, provider, err := a.LRouter.ExecuteWithFallback(a.ctx, prompts.GetSwarmCommandPrompt(), message)
	if err != nil { return "🔴 Falha no comando: " + err.Error() }
	
	return fmt.Sprintf("💎 [%s] Resposta do Enxame: %s", provider, response)
}
