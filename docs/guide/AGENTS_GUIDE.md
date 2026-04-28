---
title: "Guia de Agentes (Protocolo ACP)"
type: "guide"
status: "active"
tags: ["agents", "acp", "security", "terminal"]
---

# 🤖 Guia de Agentes (Soberania ACP)

> [!ABSTRACT]
> O Lumaestro utiliza o **Agent Control Protocol (ACP)** para sessões interativas e seguras. Isso permite que a IA execute comandos de terminal, manipule arquivos e realize tarefas complexas no seu workspace sob sua supervisão total.

## 🛡️ Arquitetura de Soberania (Hands Security)

Nenhum agente tem "cheque em branco" no Lumaestro. Toda ação passa por uma camada de validação e aprovação.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef action fill:#2d333b,stroke:#455a64,stroke-width:1px,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef warning fill:#ff9900,stroke:#333,stroke-width:2px,color:#000

    subgraph UserSpace [Soberania do Comandante]
        U([fa:fa-user Usuário])
        AP{fa:fa-shield-alt Approval Gate}
    end

    subgraph AgentSpace [Execução de Inteligência]
        AI[fa:fa-robot Agente Gemini/Claude]
        ACP[fa:fa-terminal ACP Protocol]
    end

    subgraph Security [Filtro de Segurança]
        SF{fa:fa-lock Hands Security}
        LOG[(fa:fa-history Audit Log)]
    end

    %% Fluxo
    AI -->|1. Proposta de Ação| SF
    SF -->|2. Avalia Risco| SF
    
    SF -- "Risco Alto" --> AP
    AP -- "Aprovado" --> ACP
    AP -- "Rejeitado" --> AI
    
    SF -- "Risco Baixo" --> ACP
    
    ACP -->|3. Resultado| LOG
    LOG --> AI

    %% Estilos
    class U trigger
    class AI,ACP core
    class SF,LOG action
    class AP warning
```

---

## 🛠️ Capacidades dos Agentes

Os agentes do Lumaestro não são apenas chatbots; eles possuem "mãos" digitais:

- **📟 Terminal Nativo**: Execução de scripts, builds e testes (com suporte a PowerShell e Bash).
- **📂 Manipulação de Arquivos**: Criação, leitura e edição (patching) de arquivos do projeto.
- **🧠 Consciência de Contexto**: Acesso em tempo real ao grafo neural para fundamentar decisões.
- **🛡️ Hard Stop**: Interrupção automática em caso de detecção de loops infinitos ou gastos excessivos de tokens.

---

## 🔗 Documentos Relacionados

- [[ACP_MODE]] — Detalhamento técnico do protocolo de execução.
- [[NEURAL_BRAIN]] — Como o grafo alimenta a decisão dos agentes.
- [[MULTI_AGENT_SYSTEM]] — Orquestração de múltiplos especialistas (Enxame).
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Inteligência com Soberania. 🐹🛡️🤖⚙️**
