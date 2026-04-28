---
title: "Proveniência e Auditoria (Provenance & Audit)"
type: "component"
status: "active"
tags: ["audit", "provenance", "grounding", "transparency"]
---

# 👁️ Proveniência e Auditoria: O Elo Inquebrável

> [!ABSTRACT]
> A **Proveniência** no Lumaestro é o mecanismo de rastreabilidade total que permite ao Comandante verificar a fonte de cada fato gerado ou armazenado. Este sistema garante que a inteligência artificial nunca se desconecte da realidade documental do seu workspace.

## 🛡️ Rastreabilidade de Linhagem (Grounding)

O Lumaestro estabelece uma conexão direta entre o pensamento da IA e a matéria bruta do conhecimento.

```mermaid
flowchart LR
    %% Estilos
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    classDef source fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef audit fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff

    subgraph Intelligence [Córtex de IA]
        A[fa:fa-robot Resposta da LLM]
        N[fa:fa-star Nó Ativo no Grafo]
    end

    subgraph Registry [Registro de Verdade]
        Q[(fa:fa-database Qdrant: Path/Content)]
    end

    subgraph Origin [Soberania da Fonte]
        O[fa:fa-file-alt Arquivo Obsidian/Código]
        ED[fa:fa-external-link-alt Editor Nativo]
    end

    %% Elo de Proveniência
    A -->|1. Referencia ID| N
    N -->|2. Busca Metadados| Q
    Q -->|3. Aponta para| O
    O -->|4. Auditoria Humana| ED

    %% Estilos
    class A,N ia
    class Q audit
    class O,ED source
```

---

## 🔍 Interface de Auditoria Total

O sistema oferece três camadas de verificação de veracidade:

### 1. Sidebar de Proveniência
Integrada ao HUD 3D, esta lateral de vidro é ativada ao interagir com qualquer nó. Ela exibe:
- **Origem**: O caminho absoluto do arquivo.
- **Timestamp**: Quando o conhecimento foi capturado ou modificado.
- **Snippet**: O fragmento de texto exato usado para fundamentar a afirmação.

### 2. Acesso à Fonte Nativa
Através do método `OpenFileInEditor`, o usuário pode saltar da interface 3D diretamente para o arquivo original no **Obsidian**, **VSCode** ou leitor de **PDF**, eliminando qualquer atrito na validação manual.

### 3. Grounded Reasoning (Chat)
Cada resposta do chat é acompanhada por referências de nós. Ao clicar nestas referências, o grafo navega automaticamente para o nó de origem, permitindo uma inspeção visual da "vizinhança semântica" do fato.

---

## 🔗 Documentos Relacionados

- [[NEURAL_BRAIN]] — Visualização imersiva dos nós auditáveis.
- [[BACKEND_METHODS]] — Detalhes da API `GetNodeDetails`.
- [[RAG_FLOW]] — Como a linhagem é criada durante a ingestão.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Inteligência transparente. Verdade inegociável. 👁️🛡️✨**
