---
tags:
  - architecture
  - multi-agent
  - swarm
  - orchestration
  - lumaestro
---

# 🐝 Sistema de Multi-Agentes (Swarm Orchestration)

> [!ABSTRACT] Visão Geral
> No Lumaestro, os agentes não trabalham isolados. Eles operam em um ecossistema de **Enxame (Swarm)**, onde tarefas complexas são quebradas em sub-tarefas e delegadas dinamicamente. Este sistema é inspirado em metodologias de governança corporativa e gerenciamento de tickets (como Agile e Linear).

---

## 🏗️ A Arquitetura do Enxame

O funcionamento multi-agente baseia-se em três pilares: **Delegação (Handoff)**, **Governança de Custos (Budget)** e **Auditoria Persistente**.

### 1. O Mecanismo de Handoff (Delegação)
Diferente de uma simples chamada de função, a delegação no Lumaestro cria uma nova entidade no banco de dados chamada `Issue` (ou Ticket). Quando o **Agente A** percebe que uma subtarefa foge de sua especialidade, ele utiliza o `DelegateTask`.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    classDef action fill:#455a64,stroke:#fff,stroke-width:1px,color:#fff

    subgraph Command [Comando Central]
        U([fa:fa-user Usuário])
        M{fa:fa-brain Maestro Planner}
    end

    subgraph Swarm [O Enxame de Especialistas]
        direction TB
        T1[fa:fa-ticket-alt Tarefa: Frontend]
        T2[fa:fa-ticket-alt Tarefa: Backend]
        
        VueAg[fa:fa-palette Vue Expert]
        GoAg[fa:fa-terminal Go Gopher]
    end

    subgraph Registry [Orquestração de Handoff]
        H[internal/orchestration/handoff.go]
    end

    %% Fluxo
    U -->|1. Solicita| M
    M -->|2. Divide| T1 & T2
    
    T1 -->|Assignee| VueAg
    T2 -->|Assignee| GoAg
    
    VueAg <-->|3. Handoff| H
    H <-->|4. Delegate| GoAg
    
    GoAg -->|5. Report| M

    %% Estilos
    class U trigger
    class M core
    class VueAg,GoAg ia
    class T1,T2,H action
```

### 2. Governança e "Hard Stop" Financeiro
Para evitar gastos desenfreados com APIs de LLM, cada agente possui um "orçamento mensal". Se o limite for atingido, o sistema impõe um **Hard Stop**, pausando o agente imediatamente.

```mermaid
flowchart TD
    %% Estilos
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef db fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef warning fill:#ff9900,stroke:#333,stroke-width:2px,color:#000

    subgraph Governance [Gestão de Recursos]
        Ag[fa:fa-robot Agente]
        B{fa:fa-wallet Budget Manager}
        DB[(fa:fa-database DuckDB)]
    end

    subgraph Safety [Barreira Hard Stop]
        STOP((fa:fa-hand-paper STOP))
    end

    %% Fluxo
    Ag -->|1. Ação LLM| B
    B -->|2. Log Cost| DB
    B -->|3. Check Budget| DB
    
    DB -- "Limite Excedido" --> STOP
    STOP -->|4. Pause Agent| Ag

    %% Estilos
    class B,DB core
    class STOP warning
```

---

## 🧩 Componentes do Código

### Delegação de Tarefas (`internal/orchestration/handoff.go`)
A função `DelegateTask` é a interface principal para a colaboração entre agentes. Ela vincula uma sub-tarefa a um `ParentID`, permitindo o rastreio da árvore de decisão.

### Controle de Orçamento (`internal/orchestration/budget.go`)
Gerencia o `SpentMonthlyCents`. Cada chamada de modelo (Gemini, Claude, etc.) emite um evento de custo que é processado em tempo real.

### Estados do Agente (`internal/db/schema.go`)
Os agentes transitam entre estados que definem sua disponibilidade:
- `idle`: Aguardando tarefas.
- `running`: Executando uma `Issue`.
- `paused`: Interrompido por segurança (ACP) ou falta de orçamento.

---

## 🕵️ Auditoria e Transparência

Toda interação multi-agente gera uma trilha de evidências:
1.  **ActivityLog:** "O Agente X delegou a tarefa Y para o Agente Z".
2.  **IssueComment:** Justificativas textuais sobre o porquê da delegação.
3.  **Timeline Visual:** No frontend, você vê o progresso da "conversa" entre as IAs.

---

## 💡 Dicas para o Comandante

> [!TIP]
> **Especialização é Chave:** No Lumaestro, é melhor ter 3 agentes pequenos e especialistas (ex: CSS-Expert, SQL-Expert, Doc-Master) do que um único agente gigante. Isso reduz o custo de tokens e aumenta a precisão das respostas.

> [!WARNING]
> **Loop de Delegação:** Evite criar dependências circulares onde o Agente A delega para B, que delega de volta para A sem progresso. O Maestro Planner deve ser usado para quebrar esses loops.

---
[[AGENTS_GUIDE|⬅️ Guia de Agentes]] | [[INDEX|Voltar ao Índice]]
