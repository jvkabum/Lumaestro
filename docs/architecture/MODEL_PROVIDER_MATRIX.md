---
title: "Matriz de Provedores de Modelo (AI Matrix)"
type: "technical-spec"
status: "active"
tags: ["llm", "gemini", "claude", "lmstudio", "orchestration"]
---

# 🧠 Matriz de Provedores de Modelo (AI Matrix)

> [!ABSTRACT]
> O Lumaestro é agnóstico a modelos de linguagem. Ele opera através de um **Pool de Provedores** que permite alternar dinamicamente entre Gemini, Claude e modelos locais (LM Studio), garantindo que cada tarefa seja executada pelo "cérebro" mais eficiente para aquela carga de trabalho.

## 🏗️ Arquitetura de Roteamento de Capacidades

O sistema não escolhe apenas um modelo; ele orquestra capacidades baseadas em latência, custo e especialização.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef action fill:#2d333b,stroke:#455a64,stroke-width:1px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff

    subgraph UserIntent [Intenção do Comandante]
        Q[fa:fa-comment Pergunta / Tarefa]
    end

    subgraph Router [Roteador de Capacidades]
        R{fa:fa-route AI Provider Pool}
    end

    subgraph Providers [Ecossistema de Cérebro]
        G[fa:fa-google Gemini v2]
        C[fa:fa-robot Claude 3.5]
        L[fa:fa-desktop LM Studio / Local]
    end

    subgraph Tasks [Especialização de Carga]
        T1[fa:fa-brain Embeddings]
        T2[fa:fa-project-diagram Ontologia]
        T3[fa:fa-terminal ACP Execution]
    end

    %% Fluxo
    Q --> R
    R -->|Chat/Orquestração| G & C & L
    
    G -->|Embeddings| T1
    C -->|Complex Logic| T2
    L -->|Local/Offline| T3

    %% Estilos
    class Q trigger
    class R core
    class G,C,L ia
    class T1,T2,T3 action
```

---

## 📊 Matriz de Dependências e Substituição

| Área | Função | Dependência Atual | Status de Migração | Alternativa Recomendada |
| :--- | :--- | :--- | :--- | :--- |
| **RAG** | Embeddings | Gemini Embedding v2 | **Dependente** | Local OpenAI / BGE |
| **Ontologia** | Extração Semântica | Gemini Flash 1.5 | **Parcial** | Claude 3.5 Sonnet |
| **Chat** | Orquestração | Provedor Pool | **Agnóstico** | Gemini / Claude / LM Studio |
| **ACP** | Execução de Código | Provedor Pool | **Agnóstico** | LM Studio (Offline) |

---

## 🚀 Plano de Evolução (Migração)

1.  **Desacoplamento de Embeddings**: Introduzir uma camada de abstração para permitir que o Qdrant use dimensões diferentes de outros provedores (OpenAI/Local).
2.  **Roteamento por Capacidade**: Definir tags como `multimodal`, `long-context` ou `reasoning` para que o Maestro escolha o provedor automaticamente.
3.  **Modo Offline Total**: Priorizar o uso de LM Studio para todas as tarefas de processamento de arquivos sensíveis, eliminando a dependência de nuvem.

---

## 🔗 Documentos Relacionados

- [[AGENTS_GUIDE]] — Como os agentes usam este pool para executar tarefas.
- [[RAG_FLOW]] — A dependência de modelos de embedding.
- [[LIGHTNING_ENGINE]] — Otimização de custos baseada na escolha do modelo.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Inteligência Híbrida. Soberania Local. 🧠🤖🛡️**
