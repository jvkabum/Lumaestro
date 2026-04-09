# Matriz de Dependencias de Modelo (Gemini x Claude x LM Studio)

Objetivo: listar onde o sistema ainda depende de Gemini e o que pode ser trocado por Claude/LM Studio sem quebrar o fluxo.

## Resumo executivo

- O pipeline de sincronizacao de memoria usa IA em dois pontos criticos: extracao semantica (triplas) e embeddings.
- Hoje esses dois pontos estao acoplados ao servico EmbeddingService (Google GenAI).
- Chat e orquestracao ja conseguem operar com pool de provedores ativos (Gemini/Claude/LM Studio), mas o RAG semantico profundo ainda depende de embeddings compativeis.

## Matriz por funcionalidade

| Area | Funcao | Arquivo | Dependencia atual | Pode trocar agora? | Alternativa recomendada | Esforco |
|---|---|---|---|---|---|---|
| Memoria de chat | Extracao de triplas | internal/rag/memories.go | OntologyService -> GenerateContentWithRetry (Gemini/Gemma via GenAI) | Parcial | LM Studio (modelo instrucional forte) ou Claude via adapter de extracao | Medio |
| Memoria de chat | Embedding de fatos | internal/rag/memories.go | EmbeddingService.GenerateEmbedding (gemini-embedding-2-preview) | Nao direto | Provedor de embeddings compativel (OpenAI local, bge/e5 local) com dimensao padronizada | Alto |
| Sincronizacao Obsidian | Extracao de triplas de notas | internal/obsidian/crawler.go | OntologyService.ExtractTriples | Parcial | Claude/LM Studio para extracao textual | Medio |
| Sincronizacao Obsidian | Processamento de midia (imagem/pdf) | internal/obsidian/crawler.go | OntologyService.ProcessMedia (multimodal GenAI) | Parcial | Claude multimodal (API) ou LM Studio multimodal se modelo suportar | Medio/Alto |
| Sincronizacao Obsidian | Embedding de conteudo | internal/obsidian/crawler.go | GenerateEmbedding / GenerateMultimodalEmbedding | Nao direto | Trocar para backend de embeddings abstraido | Alto |
| Infra vetorial | Dimensao da colecao | internal/obsidian/crawler.go | 3072 (Gemini embedding v2) | Sim, com migracao | Recriar colecoes por provider ou padronizar dimensao unica | Medio |
| Boot dos motores | Inicializacao sem Gemini | internal/core/app.go | Embeddings opcional (modo degradado) | Sim | Manter pool ativo e subir chat mesmo sem RAG vetorial | Ja aplicado |
| Chat/Roteamento | Selecao de provedor | internal/agents/acp/orchestrator.go | Pool configuravel (blend + primary provider) | Sim | Gemini/Claude/LM Studio | Ja aplicado |
| Swarm | Sessao de agente de fundo | internal/core/app_swarm.go | Usava Gemini fixo, agora usa provider pool | Sim | Primary provider + ativos | Ja aplicado |
| Lightning Router | Fallback multi-provedor | internal/lightning/router.go | gemini/openai/claude | Sim | respeitar pool ativo salvo no config | Ja aplicado |

## Onde o Gemini aparece hoje (inventario tecnico)

### 1) Embeddings e geracao semantica (nucleo)

- internal/provider/embeddings.go
  - modelo de embedding: gemini-embedding-2-preview
  - cascata de geracao: gemini-3.1-flash-lite-preview, gemini-2.5-flash, gemini-3-flash-preview, gemini-2.5-flash-lite, gemma-4-31b-it, gemma-4-26b-a4b-it

### 2) Ontologia e memoria estruturada

- internal/provider/ontology.go
  - ExtractTriples
  - ValidateConflict
  - ProcessMedia

### 3) Sync de memoria/conhecimento

- internal/rag/memories.go
  - WeaveChatKnowledge usa extracao de triplas + embeddings
- internal/obsidian/crawler.go
  - processFile usa extracao semantica e embeddings para persistir no Qdrant
- internal/core/app_sync.go
  - ScanVault chama crawler para indexacao continua

### 4) ACP/CLI e operacao de agente

- internal/agents/acp/session.go
- internal/agents/executor.go
- internal/tools/installer.go

## Plano de migracao recomendado (por prioridade)

1. Separar provider de embeddings do provider de geracao.
2. Introduzir roteamento por capacidade:
   - chat_generation
   - semantic_extraction
   - embeddings_text
   - embeddings_multimodal
3. Adicionar fallback de embeddings com normalizacao de dimensao.
4. Implementar reindexacao assistida quando trocar o provider de embeddings.

## Decisao pratica para o problema atual

- Se o objetivo e nao depender de Gemini para iniciar: ja possivel (chat/sistema sobem em modo degradado).
- Se o objetivo e sincronizacao de memoria com IA sem Gemini: precisa migrar embeddings e ontologia para providers alternativos ou adapters dedicados.
