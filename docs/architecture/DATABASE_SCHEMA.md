# 🗄️ Esquema de Dados (Database Schema) 🏛️📊

O Lumaestro utiliza **DuckDB** para persistência, aproveitando sua capacidade analítica para processar telemetria de agentes em tempo real. O esquema é gerenciado via GORM em internal/db/schema.go.

## 🗺️ Mapa de Entidades (ERD)

`mermaid
erDiagram
    %% Estilo Dark Mode
    AGENT ||--o{ ISSUE : assignee
    AGENT ||--o{ COST_EVENT : incurs
    AGENT ||--o{ ACTIVITY_LOG : performs
    ISSUE ||--o{ ISSUE_COMMENT : timeline
    ISSUE ||--o{ ISSUE_ATTACHMENT : contains
    DOCUMENT ||--o{ DOCUMENT_REVISION : history
    PROJECT ||--o{ ISSUE : contains
    GOAL ||--o{ PROJECT : drives
    
    AGENT {
        uuid id
        string name
        string role
        string status
        int budget_monthly_cents
        int spent_monthly_cents
    }
    
    ISSUE {
        uuid id
        uuid project_id
        string title
        string status
        string priority
    }
    
    COST_EVENT {
        uuid id
        uuid agent_id
        string model
        int cost_cents
        timestamp occurred_at
    }
`

## 🧩 Tabelas Principais

### 1. Núcleo de Agentes (Agent)
Armazena a identidade e o estado financeiro de cada trabalhador do enxame.
- **UUID:** Identificação única global.
- **Status:** ctive, paused, idle, unning, error.
- **LastHeartbeatAt:** Usado para detectar agentes que travaram.

### 2. Gestão de Trabalho (Issue, Project, Goal)
Hierarquia de produtividade do enxame:
- **Goal:** Visão estratégica (ex: "Lançar versão 1.0").
- **Project:** Roadmap tático.
- **Issue:** Unidade de trabalho atômica.

### 3. Memória e Conhecimento (Document, DocumentRevision)
Onde o RAG e o enxame salvam resultados de longo prazo.
- Suporta **Versionamento Automático** via DocumentRevision.
- O corpo do documento é armazenado como 	ext.

### 4. Telemetria e Auditoria (CostEvent, ActivityLog, HeartbeatRun)
Essencial para auditoria técnica e financeira.
- **CostEvent:** Detalha tokens de entrada/saída e custo em centavos.
- **ActivityLog:** Rastro de migalhas (quem fez o quê e quando).

## 🛠️ Tipos Customizados
- **Timestamp**: Um wrapper sobre 	ime.Time para garantir compatibilidade com o Wails v2 (JSON binding).
- **Base**: Modelo base que utiliza UUID em vez de IDs incrementais para evitar colisões em sistemas distribuídos.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[SWARM_ORCHESTRATION|⬅️ Voltar: Orquestração]]
