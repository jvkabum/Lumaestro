---
title: "Fluxo de Conhecimento (RAG Flow)"
type: "architecture"
status: "active"
tags: ["rag", "embeddings", "qdrant", "obsidian"]
---

# 🧠 RAG Flow: A Jornada da Matéria Cognitiva

> [!ABSTRACT]
> O sistema de **Retrieval-Augmented Generation (RAG)** do Lumaestro é o que diferencia o projeto de um simples chatbot. Ele transforma seu repositório local (Obsidian/Código) em um "Córtex Neural" vivo, onde a IA fundamenta cada resposta em dados reais e atualizados do seu workspace.

## 🏗️ Pipeline de Inteligência

A transformação do dado bruto em sabedoria ocorre em três fases distintas e orquestradas.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef action fill:#2d333b,stroke:#455a64,stroke-width:1px,color:#fff
    classDef db fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff

    subgraph Ingestion [Fase 1: Ingestão de Matéria]
        direction TB
        OBS[fa:fa-file-alt Obsidian Vault]
        CRAWL[fa:fa-spider Crawler]
        EMB[fa:fa-brain Embedding Model]
        QDR[(fa:fa-database Qdrant: Vetores)]
    end

    subgraph Retrieval [Fase 2: Recuperação de Contexto]
        direction TB
        Q[fa:fa-comment Pergunta do Usuário]
        SIM{fa:fa-search Similaridade Semântica}
        CTX[fa:fa-microchip Contexto Injetado]
    end

    subgraph Generation [Fase 3: Geração de Resposta]
        LLM[fa:fa-robot LLM Engine]
        ANS[fa:fa-check-circle Resposta Fundamentada]
    end

    %% Fluxo
    OBS --> CRAWL
    CRAWL --> EMB
    EMB --> QDR

    Q --> SIM
    SIM <--> QDR
    SIM --> CTX
    CTX --> LLM
    LLM --> ANS

    %% Estilos
    class OBS,Q trigger
    class CRAWL,CTX action
    class QDR db
    class EMB,LLM,ANS ia
```

---

## 🔬 Detalhes das Fases

### 1. Ingestão (Knowledge Weaving)
O **Crawler** monitora mudanças no sistema de arquivos em tempo real. Cada nota modificada é fragmentada (chunking) e enviada para o modelo de **Embeddings** do Gemini, que gera uma representação vetorial matemática da ideia.

### 2. Recuperação (Semantic Search)
Quando o usuário faz uma pergunta, o sistema não busca por palavras-chave (como o Google antigo), mas por **sentido**. O Qdrant retorna os fragmentos de conhecimento que têm a maior similaridade matemática com a intenção do usuário.

### 3. Geração (Grounded Response)
O contexto recuperado é injetado no prompt do sistema como uma "verdade absoluta". A LLM então sintetiza a resposta, citando fontes e garantindo que não haja alucinações.

---

## 🛠️ Tecnologias Utilizadas

- **Qdrant**: Banco de dados vetorial de alta performance para armazenamento de embeddings.
- **Gemini Embeddings**: Modelo de última geração para tradução de texto em vetores.
- **DuckDB**: Utilizado para metadados e busca textual rápida (Fuzzy Search).

---

## 🔗 Documentos Relacionados

- [[SEMANTIC_NAVIGATOR]] — Como o GPS semântico navega por estas fases.
- [[CODE_RAG_GUIDE]] — Guia específico para RAG aplicado a código fonte.
- [[NEURAL_BRAIN]] — Visualização 3D do conhecimento indexado.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Sua realidade. Inteligência artificial. 🧠🕸️✨**
