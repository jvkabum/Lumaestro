---
tags:
  - security
  - agents
  - acp
  - wails
---

# 🛡️ Guia do ACP Mode (Approval Control Protocol)

> [!ABSTRACT] Visão Geral
> O **ACP (Approval Control Protocol)** é o mecanismo de segurança central do Lumaestro. Ele atua como um "Portão Humano" (Human-in-the-Loop), interceptando comandos sensíveis ou ações de alto risco propostas pelos agentes antes que elas sejam executadas no sistema operacional ou na base de dados.

---

## 🏗️ Como Funciona o Fluxo de Aprovação

O ACP não é apenas um "sim ou não", mas uma sincronização de estado entre o Agente, o Banco de Dados e o Usuário.

### Ciclo de Vida de um Pedido ACP

1.  **Solicitação:** Um agente (ex: Gemini ou Claude) decide executar um comando (ex: m -rf ou git push).
2.  **Interceptação:** O internal/orchestration/approvals.go cria um registro de aprovação com status pending.
3.  **Suspensão:** O agente é imediatamente colocado em estado paused no banco de dados.
4.  **Notificação:** O backend emite um evento via Wails para o componente ReviewBlock.vue no frontend.
5.  **Decisão Humana:** O usuário revisa o payload da ação e clica em **Aprovar** ou **Rejeitar**.
6.  **Liberação:** O backend processa a decisão, altera o status do agente para idle (ou executa o comando) e registra o log de auditoria.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef action fill:#455a64,stroke:#fff,stroke-width:1px,color:#fff
    classDef warning fill:#ff9900,stroke:#333,stroke-width:2px,color:#000
    classDef db fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff

    subgraph AgentSpace [Execução de IA]
        A[fa:fa-robot Agente Gemini/Claude]
    end

    subgraph GateKeeper [Portão de Segurança Go]
        B{fa:fa-shield-alt ACP Engine}
        D[(fa:fa-database DB: State)]
    end

    subgraph UserSpace [Soberania Humana]
        F[fa:fa-desktop Frontend ReviewBlock]
        U((fa:fa-user-check DECISÃO))
    end

    %% Fluxo de Interceptação
    A -->|1. Solicita Ação| B
    B -->|2. Status = 'pending'| D
    B -->|3. Agent = 'paused'| D
    B -->|4. approval:needed| F
    
    F -->|5. Análise| U
    U -- "APROVAR" --> B
    U -- "REJEITAR" --> B
    
    B -->|6. Status = 'approved'| D
    B -->|7. Agent = 'idle'| D
    B -->|8. Release Action| A

    %% Estilos
    class A trigger
    class B core
    class D db
    class F action
    class U warning
```

---

## 🧩 Componentes Técnicos

### 1. O Portão de Segurança (pprovals.go)
A função RequestApproval é o ponto de entrada. Ela encapsula o payload da ação em JSON para que o usuário possa ler exatamente o que o agente pretende fazer.

`go
// internal/orchestration/approvals.go
func RequestApproval(agentID uuid.UUID, approvalType string, payload interface{}) (uuid.UUID, error) {
    // 1. Cria o registro no DB
    // 2. Pausa o Agente automaticamente
    // 3. Registra Log de Auditoria
}
`

### 2. O Painel de Revisão (ReviewBlock.vue)
No frontend, este componente é um "Modal Persistente" que bloqueia a interação com o chat até que a decisão seja tomada. Ele exibe:
- **Origem:** Qual agente solicitou.
- **Payload:** O código ou comando bruto.
- **Risco:** Classificação do perigo (Baseado na ontologia do Maestro).

### 3. O Motor de Recompensa (Lightning Engine)
O ACP está integrado ao **Lightning Engine**. Quando você aprova uma ação, o sistema emite uma **Recompensa Positiva (+1.0)** para o Agente no DuckDB. Se você rejeita, ele recebe uma **Punição (-1.0)**. Isso ensina o agente, ao longo do tempo, quais comandos você considera aceitáveis.

---

## ⚙️ Modos de Operação

O Lumaestro suporta dois modos de aprovação, configuráveis via internal/agents/executor.go:

| Modo | Descrição | Risco |
| :--- | :--- | :--- |
| **Protected (Padrão)** | Todas as ações de escrita exigem aprovação manual. | **Mínimo** |
| **YOLO (Autonomous)** | Agentes podem executar comandos livremente. Ativado via --approval-mode=yolo. | **Alto** |

---

## 🕵️ Auditoria e Proveniência

Todas as decisões do ACP são gravadas na tabela  ctivity_logs. Isso permite que você rastreie:
- Quem pediu (Agente).
- Quem aprovou (Usuário).
- Quando aconteceu.
- Qual foi a nota/justificativa da decisão.

```mermaid
flowchart LR
    %% Estilos
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef action fill:#455a64,stroke:#fff,stroke-width:1px,color:#fff

    subgraph Audit [Trilha de Proveniência]
        direction LR
        Log[fa:fa-history Activity Log]
        Agent[fa:fa-fingerprint Agente ID]
        User[fa:fa-user-check Decisão Humana]
        Payload[fa:fa-code Comando/Código]
    end

    %% Conexões
    Log --> Agent
    Log --> User
    Log --> Payload

    %% Estilos
    class Log core
    class Agent,User,Payload action
```

---

## 🔗 Documentos Relacionados
- [[FRONTEND_GUIDE]]: Como o ReviewBlock.vue é renderizado.
- [[LIGHTNING_ENGINE]]: Detalhes sobre o sistema de recompensas.
- [[AGENTS_GUIDE]]: Arquitetura de execução de agentes.
