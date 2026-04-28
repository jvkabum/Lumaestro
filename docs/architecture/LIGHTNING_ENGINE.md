---
title: "⚡ Lumaestro-Lightning: O Cérebro Analítico Nativo"
tags: ["core", "analytics", "duckdb", "rlhf", "telemetry"]
status: "active"
version: "1.2"
---

# ⚡ Lumaestro-Lightning: O Cérebro Analítico Nativo 🐹⚙️💰📈

Este documento descreve o motor de aprendizado por reforço e telemetria analítica do Lumaestro, portado e otimizado a partir do framework **Agent-Lightning** (Microsoft). Ele atua como a camada de observabilidade e inteligência econômica do enxame.

## 🏛️ Arquitetura de "Pulmão Duplo"

O Lumaestro utiliza uma infraestrutura de dados híbrida para garantir integridade e performance:

1.  **SQLite (O Coração)**: Gerencia o estado transacional, governança de agentes, tarefas e segredos. (OLTP)
2.  **DuckDB (O Cérebro Analítico)**: Um banco de dados colunar embutido que processa telemetria massiva, rastros de pensamento (Spans) e cálculos financeiros em tempo real. (OLAP)

> [!TIP]
> **Por que DuckDB?** Enquanto o SQLite é excelente para escritas rápidas de estado, o DuckDB permite realizar agregações complexas (ex: "Qual a média de custo por token nos últimos 1000 rollouts?") em milissegundos, sem travar a UI.

### 📊 Fluxo de Dados e Telemetria

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef action fill:#2d333b,stroke:#455a64,stroke-width:1px,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef db fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff

    subgraph AgentsLayer [Camada de Execução]
        A[fa:fa-robot Agente / Executor]
        B{fa:fa-shield-alt Lightning Proxy}
    end

    subgraph DataEngine [Motor de Pulmão Duplo]
        direction TB
        C{fa:fa-filter Interceptor}
        D[(fa:fa-database DuckDB: Analytics)]
        E[(fa:fa-database SQLite: State)]
    end

    subgraph Intelligence [Ciclo de Inteligência]
        G[fa:fa-chart-line Dashboard / UI]
        H[fa:fa-brain Reward Engine]
    end

    %% Fluxo
    A -->|Request| B
    B -->|Capture| C
    C -->|Spans/Tokens| D
    C -->|Metadados| E
    B -->|Forward| F[fa:fa-cloud LLM Provider]
    F -->|Usage| C

    D -->|Telemetry| G
    G -->|Human Feedback| H
    H -->|Dopamina Digital| D

    %% Estilos
    class A trigger
    class B,C,H action
    class D,E db
    class G core
```

---

## 🚀 Componentes do Motor

### 1. Interceptor Proxy ([proxy.go](../../internal/lightning/proxy.go))
Um interceptor HTTP nativo que atua como um túnel entre os agentes e os provedores de IA (Gemini/OpenAI).
- **Telemetria Automática**: Captura cada requisição e resposta sem necessidade de alterar o código do agente.
- **Rastreamento de Custos**: Extrai automaticamente o bloco `usage` das respostas para registrar o consumo de tokens.

### 2. Motor de Recompensas ([reward_engine.go](../../internal/lightning/reward_engine.go))
Implementa o sistema de "Dopamina Digital" do enxame.
- **Feedback Humano**: Cada aprovação ou rejeição no Dashboard emite uma recompensa (+1.0 ou -1.0).
- **Aprendizado por Reforço**: Os scores são persistidos no [store_duckdb.go](../../internal/lightning/store_duckdb.go) para análise de trajetórias de sucesso.

### 3. Otimizador APO ([optimization.go](../../internal/lightning/optimization.go))
Motor de **Automatic Prompt Optimization (APO)**.
- **Análise de Falhas**: Examina rollouts com recompensas negativas para identificar padrões de erro.
- **Refinamento**: Sugere melhorias no System Prompt baseadas no histórico de aprendizado.

---

## 💰 Consciência Financeira (Cost Tuning)

O sistema monitora o investimento em inteligência em tempo real:
- **Tabela de Custos**: Baseado nas tarifas do Gemini 1.5 Flash ($0.15/1M in, $0.60/1M out).
- **KPIs no Dashboard**: Exibe o custo total acumulado (USD) e a eficiência por rollout através do componente [SwarmDashboard.vue](../../frontend/src/components/SwarmDashboard.vue).

### 🔄 Ciclo de Reforço (Digital Dopamine)

```mermaid
flowchart TD
    %% Estilos
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    classDef action fill:#455a64,stroke:#fff,stroke-width:1px,color:#fff

    subgraph Loop [Ciclo de Reforço]
        direction TB
        A[fa:fa-robot Agente]
        P{fa:fa-shield-alt Proxy}
        D[(fa:fa-database DuckDB)]
        H[fa:fa-user-check Comandante]
        O[fa:fa-magic APO Optimizer]
    end

    %% Fluxo Circular
    A -->|1. Executa Prompt| P
    P -->|2. Registra Spans| D
    D -->|3. Apresenta Dados| H
    H -->|4. Atribui Recompensa| D
    D -->|5. Treina Dataset| O
    O -->|6. Refina Prompt| A

    %% Nota Estilizada
    Note[O erro de hoje é a sabedoria de amanhã]
    O -.-> Note
    Note -.-> A

    %% Estilos
    class A,O ia
    class P,H action
    class D core
```

---

## 🛠️ Como Operar

### Ativação
O motor Lightning é iniciado automaticamente no boot do aplicativo se habilitado nas configurações ([config.go](../../internal/config/config.go)).
- **Porta Padrão**: `8001` (Proxy).
- **Arquivo de Dados**: `.lumaestro/analytics.db`.

### Emitindo Recompensas Manuais
Você pode emitir recompensas programaticamente ou via interface:
```go
// Exemplo de emissão manual de recompensa no backend
re := lightning.NewRewardEngine(lStore)
re.EmitReward(rolloutID, attemptID, 1.0, "manual_feedback", nil)
```

---

> [!IMPORTANT]
> **Mente Colmeia:** O conhecimento aprendido é destilado pelo motor e pode ser sincronizado com o **Obsidian Vault** (RAG), garantindo que as lições de um agente sirvam para todo o enxame.

**Lumaestro: Inteligência que aprende. Economia que escala. 🐹⚡🤖💰**

---

## 🔗 Documentos Relacionados
- [[INDEX|Índice Geral]]: Hub central de documentação.
- [[NEURAL_BRAIN|NEURAL_BRAIN]]: Grafos, PageRank e Auditoria.
- [[RAG_FLOW|RAG_FLOW]]: Pipeline de busca vetorial.
- [[API/BACKEND_METHODS|BACKEND_METHODS]]: Referência técnica dos bindings Wails.
