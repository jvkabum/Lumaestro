# ⚡ Lumaestro-Lightning: O Cérebro Analítico Nativo 🐹⚙️💰📈

Este documento descreve o motor de aprendizado por reforço e telemetria analítica do Lumaestro, portado e otimizado a partir do framework **Agent-Lightning** (Microsoft).

## 🏛️ Arquitetura de "Pulmão Duplo"

O Lumaestro utiliza uma infraestrutura de dados híbrida para garantir integridade e performance:

1.  **SQLite (O Coração)**: Gerencia o estado transacional, governança de agentes, tarefas e segredos.
2.  **DuckDB (O Cérebro Analítico)**: Um banco de dados colunar embutido que processa telemetria massiva, rastros de pensamento (Spans) e cálculos financeiros em tempo real.

---

## 🚀 Componentes do Motor

### 1. Interceptor Proxy ([proxy.go](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/internal/lightning/proxy.go))
Um interceptor HTTP nativo que atua como um túnel entre os agentes e os provedores de IA (Gemini/OpenAI).
- **Telemetria Automática**: Captura cada requisição e resposta sem necessidade de alterar o código do agente.
- **Rastreamento de Custos**: Extrai automaticamente o bloco `usage` das respostas para registrar o consumo de tokens.

### 2. Motor de Recompensas ([reward_engine.go](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/internal/lightning/reward_engine.go))
Implementa o sistema de "Dopamina Digital" do enxame.
- **Feedback Humano**: Cada aprovação ou rejeição no Dashboard do Lumaestro emite uma recompensa (+1.0 ou -1.0).
- **Aprendizado por Reforço**: Os scores são persistidos no [store_duckdb.go](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/internal/lightning/store_duckdb.go) para análise de trajetórias de sucesso.

### 3. Otimizador APO ([optimization.go](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/internal/lightning/optimization.go))
Motor de Otimização Automática de Prompts (APO).
- **Análise de Falhas**: Examina rollouts com recompensas negativas para identificar padrões de erro.
- **Refinamento**: Sugere melhorias no System Prompt baseadas no histórico de aprendizado.

---

## 💰 Consciência Financeira (Cost Tuning)

O sistema monitora o investimento em inteligência em tempo real:
- **Tabela de Custos**: Baseado nas tarifas do Gemini 1.5 Flash ($0.15/1M in, $0.60/1M out).
- **KPIs no Dashboard**: Exibe o custo total acumulado (USD) e a eficiência por rollout.

## 📊 Visualização Executiva

Os dados do DuckDB são expostos via Bindings Wails para o [SwarmDashboard.vue](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/frontend/src/components/SwarmDashboard.vue):
- **GetLightningStats**: Retorna Rollouts totais, Média de Recompensa e Investimento Total implementado no [app.go](file:///c:/git/projeto%20sem%20nome%20ia/Lumaestro/app.go).
- **Gráficos de Telemetria**: Visualização de "Dopamina Digital" (Satisfação do Comandante).

---

## 🛠️ Como Operar

### Ativação
O motor Lightning é iniciado automaticamente no boot do aplicativo se habilitado nas configurações.
- **Porta Padrão**: 8001 (Proxy).
- **Arquivo de Dados**: `.lumaestro/analytics.db`.

### Emitindo Recompensas Manuais
Você pode emitir recompensas programaticamente ou via interface:
```go
re := lightning.NewRewardEngine(lStore)
re.EmitReward(rolloutID, attemptID, 1.0, "manual_feedback", nil)
```

---

> [!IMPORTANT]
> **Mente Colmeia:** O conhecimento aprendido é destilado pelo motor e pode ser sincronizado com o **Obsidian Vault** (RAG), garantindo que as lições de um agente sirvam para todo o enxame.

**Lumaestro: Inteligência que aprende. Economia que escala. 🐹⚡🤖💰**

---

##  Documentos Relacionados
- [ �ndice Geral](./DOCS_INDEX.md)  Hub central de documenta��o
- [ NEURAL_BRAIN](./NEURAL_BRAIN.md)  Grafos, PageRank, Auditoria
- [ DOCUMENTATION](./DOCUMENTATION.md)  Arquitetura geral
- [ RAG_FLOW](./RAG_FLOW.md)  Pipeline de busca vetorial
- [ LIGHTNING_ENGINE](./LIGHTNING_ENGINE.md)  DuckDB e aprendizado
