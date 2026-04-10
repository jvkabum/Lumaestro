# 🐝 Orquestração do Enxame (Swarm Orchestration) 🏗️⚙️

A orquestração no Lumaestro não é apenas uma fila de mensagens; é um sistema de **governança corporativa para agentes**. O enxame opera sob um modelo de delegação de tarefas baseado em tickets, inspirado em metodologias como Agile/Linear.

## 🔄 Fluxo de Trabalho do Agente

O ciclo de vida de uma tarefa no enxame segue um caminho rigoroso de auditoria e responsabilidade.

`mermaid
graph TD
    %% Estilo Dark Mode
    classDef default fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;
    
    A[Usuário/Sistema] -->|Cria Issue| B(Backlog/Todo)
    B -->|Assignee Assigned| C{Agente Disponível?}
    C -->|Sim| D[Agente Inicia Execução]
    C -->|Não| E[Fila de Espera]
    D -->|Precisa de Ajuda| F[DelegateTask]
    F -->|Cria Sub-Issue| B
    D -->|Finaliza| G[Status: DONE]
    G -->|Trigger| H[Notifica Criador/Pai]
    
    subgraph Auditoria
        I[ActivityLog]
        J[CostEvent]
        K[IssueComment]
    end
    
    D -.-> I
    D -.-> J
    D -.-> K
`

## 🤝 O Mecanismo de Handoff

O arquivo internal/orchestration/handoff.go implementa a função DelegateTask, que permite que um agente "passe o bastão".

> [!TIP]
> **Handoff Assíncrono:** No Lumaestro, quando o Agente A delega para o Agente B, o Agente A pode entrar em estado paused ou continuar outras tarefas, enquanto o Agente B recebe uma nova Issue em sua fila.

### Componentes Chave:
- **DelegateTask**: Cria uma nova tarefa vinculada à tarefa pai (ParentID).
- **Timeline de Auditoria**: Cada delegação gera automaticamente um IssueComment e um ActivityLog.
- **Status Heartbeat**: O sistema monitora a saúde do agente durante a execução através de HeartbeatRun.

## 💰 Governança de Custo e Orçamento (Hard Stop)

Diferente de sistemas de chat simples, o Lumaestro impõe limites financeiros aos agentes através do RegistrarCusto.

| Campo | Descrição |
|-------|-----------|
| BudgetMonthlyCents | O limite máximo que o agente pode gastar por mês (em centavos). |
| SpentMonthlyCents | O total acumulado de gastos via CostEvent. |
| Status: PAUSED | Estado automático ("Hard Stop") se o orçamento for excedido. |

`mermaid
sequenceDiagram
    participant A as Agente
    participant O as Orchestrator
    participant DB as DuckDB
    
    A->>O: Executa Chamada LLM
    O->>A: Retorna Resposta + Tokens
    O->>O: RegistrarCusto()
    O->>DB: Update SpentMonthlyCents
    alt Gasto >= Limite
        O->>DB: Status = 'paused'
        O->>DB: Log Activity: out_of_budget
    end
`

## 📂 Arquivos Relacionados
- internal/orchestration/handoff.go: Lógica de delegação.
- internal/orchestration/budget.go: Controle financeiro e Hard Stop.
- internal/db/schema.go: Estrutura de dados das Issues e Agentes.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[DATABASE_SCHEMA|Próximo: Esquema de Dados ➡️]]
