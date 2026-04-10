# 🏛️ Arquitetura do Hub (The Central Cortex) 🐹🏢

O arquivo internal/core/app.go é o coração do Lumaestro. Ele implementa a struct App, que funciona como o **Hub Central** (Orquestrador Soberano) de todos os serviços.

## 🏗️ Estrutura Hub-and-Spoke

O Lumaestro não é um monólito desorganizado; ele segue uma arquitetura onde o Hub centraliza a comunicação entre módulos especialistas.

`mermaid
graph TD
    %% Estilo Dark Mode
    classDef hub fill:#6d5dfc,stroke:#2d333b,color:#ffffff;
    classDef spoke fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;

    Hub((App Struct)):::hub
    
    Hub --> S1[Lightning Engine: APO/Optimization]:::spoke
    Hub --> S2[RAG Motor: Qdrant/Embeddings]:::spoke
    Hub --> S3[Swarm: Orchestrator/Executor]:::spoke
    Hub --> S4[Obsidian: Crawler/Parser]:::spoke
    Hub --> S5[DB: DuckDB/GORM]:::spoke
`

## 🚀 Ciclo de Vida (Lifecycle)

### 1. Startup
Ao iniciar, o Hub executa o ootSequence:
- Carrega o config.json.
- Inicializa conexões (Qdrant, DuckDB).
- Acorda os Agentes em AutoStart.
- Inicia o APOWorker (Otimização em background).

### 2. Comunicação Wails (Bridge)
O Hub expõe métodos Go para o Frontend Vue 3. 
- **Events**: O Hub emite eventos assíncronos (untime.EventsEmit) como gent:log e oot:stage.
- **Bindings**: Métodos como StartAgentSession e ScanVault são chamados diretamente pela interface.

### 3. Segurança (checkRogueMainFiles)
O Hub possui um mecanismo de defesa que impede que arquivos Go órfãos com package main em subpastas quebrem o hot-reload do Wails.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[BACKEND_METHODS|Próximo: Métodos RPC ➡️]]
