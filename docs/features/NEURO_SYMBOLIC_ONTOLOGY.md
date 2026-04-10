# 🔍 Ontologia Neuro-Simbólica: O Truth Engine 🧠💎

A **Ontologia Neuro-Simbólica** é o filtro de verdade do Lumaestro. Em vez de confiar apenas na "intuição" estatística do LLM, o sistema força a extração de conhecimento para um esquema rígido de triplas semânticas.

## 🏗️ O Blueprint de Conhecimento

Sempre que o Lumaestro lê uma nota do Obsidian ou um arquivo de código, ele tenta "atomizar" a informação em triplas:
**[Sujeito] --(Predicado)--> [Objeto]**

### Classes Obrigatórias (Entidades)
Para manter o grafo limpo, o motor de ontologia (internal/provider/ontology.go) utiliza classes pré-definidas:
- Person, Project, Task, Concept, Technology, Milestone, Bug, Decision.

### Relações Suportadas (Predicados)
- is_part_of, works_on, uses, defines, explains, mentions, created, esolved, depends_on.

## 🔄 Ciclo de Extração e Validação

`mermaid
graph LR
    %% Estilo Dark Mode
    classDef default fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;
    
    A[Texto Bruto] --> B[Extração LLM]
    B --> C{Validação de Blueprint}
    C -->|Conforme| D[Persistência Qdrant]
    C -->|Inconsistente| E[Descarte/Refinamento]
    
    D --> F[Nó no Grafo 3D]
`

## 🛡️ Resolução de Conflitos (Truth Validation)

O arquivo ontology.go implementa o método ValidateConflict. Quando o sistema encontra uma informação que contradiz o que já está no grafo:
1.  O sistema apresenta o **Fato Antigo** e o **Fato Novo**.
2.  Um agente validador decide entre UPDATE (o novo fato é uma atualização válida) ou CONFLICT (requer intervenção humana ou mais contexto).

## 📸 Multimodalidade e Visão
O motor de ontologia também processa imagens e PDFs (ProcessMedia). Ele extrai descrições textuais e triplas estruturadas diretamente de fluxos visuais, permitindo que o RAG "enxergue" diagramas e capturas de tela.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[CONTEXT_FLOW_RAG|Anterior: RAG ⬅️]]
