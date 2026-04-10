# 🌌 Fluxo de Contexto RAG (Cognitive Cosmos) 🧠🚀

O motor RAG do Lumaestro utiliza uma arquitetura de **Duas Fases** baseada em uma metáfora celestial para organizar e recuperar conhecimento.

## 🛰️ Arquitetura Celestial (The Cosmos Model)

O Crawler organiza o vault do Obsidian e repositórios de código em uma hierarquia visual:

| Nível | Entidade | Representação no Grafo |
|-------|----------|------------------------|
| **1** | Galáxia | Raiz do Vault ou Repositório Satélite. |
| **2** | Planeta | Pastas e subpastas (Agrupadores). |
| **3** | Lua | Notas (.md), código e mídias (Arquivos). |
| **4** | Estrela | Links semânticos e Triplas extraídas via IA. |

`mermaid
graph TD
    %% Estilo Dark Mode
    classDef sun fill:#f9d71c,stroke:#6d5dfc,color:#2d333b;
    classDef planet fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;
    classDef moon fill:#2d333b,stroke:#4a4a4a,color:#e6edf3;

    Sun((Galáxia Core)):::sun --> Planet1(Pasta: Backend):::planet
    Sun --> Planet2(Pasta: Docs):::planet
    Planet1 --> Moon1[auth.go]:::moon
    Planet1 --> Moon2[db.go]:::moon
    Planet2 --> Moon3[RAG.md]:::moon
    
    Moon1 -.->|Link Semântico| Moon3
`

## ⚡ O Ciclo de Indexação

### Fase 1: Sincronização de Estrutura (Zero-Cost)
O crawler percorre o sistema de arquivos e emite eventos graph:node e graph:edge para o frontend.
- **Hash SHA-256:** Detecta mudanças reais de conteúdo.
- **Resumo Estático:** Extrai automaticamente títulos e exportações (Go/JS/Py) sem usar LLM.

### Fase 2: Enriquecimento Cognitivo (IA)
Para arquivos novos ou modificados, o enxame realiza:
1.  **Extração de Triplas:** Converte texto em conhecimento estruturado (Sujeito -> Predicado -> Objeto).
2.  **Multimodalidade:** Processa imagens e PDFs via visão computacional.
3.  **Vetorização:** Salva embeddings de 3072 dimensões no **Qdrant**.

## 🔍 Motor de Busca N-Hop

Diferente de um RAG tradicional que busca apenas por similaridade, o Lumaestro faz:
1.  **Busca Vetorial:** Encontra as notas mais próximas semanticamente.
2.  **Exploração de Adjacência:** Puxa notas vizinhas no grafo (links [[ ]]) mesmo que não sejam similares por texto.
3.  **Re-Ranking:** O Agente Reflector valida se o contexto é útil para a tarefa atual.

## 🛠️ Configurações Críticas
- workerCount: Limitado a 2 por padrão para evitar estouro de cota (Rate Limit).
- cachePath: .context/index_cache.json armazena o estado da última indexação.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[DATABASE_SCHEMA|Anterior: Banco de Dados ⬅️]]
