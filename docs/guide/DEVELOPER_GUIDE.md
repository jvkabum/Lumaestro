---
title: "Guia do Desenvolvedor (Developer Guide)"
type: "guide"
status: "active"
tags: ["development", "setup", "wails", "go", "vue"]
---

# 💻 Guia do Desenvolvedor: Manual de Engenharia de Elite

> [!ABSTRACT]
> Este guia é o mapa de operações para os engenheiros do enxame. Ele detalha o setup do ambiente, o fluxo de desenvolvimento bi-direcional (Go ↔ Vue) e as diretrizes para manter a soberania técnica do projeto Lumaestro.

## 🏗️ Workflow de Desenvolvimento Sincronizado

O Lumaestro utiliza um ciclo de desenvolvimento ágil onde o backend e o frontend co-evoluem em tempo real.

```mermaid
flowchart TD
    %% Estilos
    classDef step fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef tool fill:#455a64,stroke:#fff,stroke-width:1px,color:#fff
    classDef finish fill:#2e7d32,stroke:#fff,stroke-width:2px,color:#fff

    subgraph Dev_Cycle [Ciclo de Desenvolvimento]
        direction TB
        G1[fa:fa-file-code Edit Go: internal/]
        G2[fa:fa-terminal wails dev]
        G3[fa:fa-magic Auto-Bindings JS]
        G4[fa:fa-code Edit Vue: frontend/]
    end

    subgraph Build_Process [Pipeline de Entrega]
        direction TB
        B1[fa:fa-hammer build.ps1]
        B2[fa:fa-box-open Single Binary EXE]
    end

    %% Conexões
    G1 --> G2
    G2 --> G3
    G3 --> G4
    G4 --> G1
    
    G4 --> B1
    B1 --> B2

    %% Estilos
    class G1,G2,G3,G4 step
    class B1 tool
    class B2 finish
```

---

## 🚀 Setup do Ambiente (Grounding)

### Pré-requisitos Mandatórios
- **Go 1.21+**: Motor de performance.
- **Node.js 20+**: Ecossistema de interface.
- **Wails CLI**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`.
- **DuckDB**: O binário analítico deve estar acessível na pasta `deps/` ou no PATH do sistema.

### Comandos de Poder (PowerShell)
| Comando | Efeito |
| :--- | :--- |
| `./dev.ps1` | Inicia o Hot Reload total (Go + Vite). Ideal para iterações rápidas. |
| `./build.ps1` | Compila a versão de produção, embutindo todos os assets e dependências. |
| `go run scripts/setup_build_env.ps1` | Prepara o ambiente isolado para compilação limpa. |

---

## 🛠️ Extensibilidade: Criando Novas Conexões

### 1. Adicionando Métodos RPC (Go → JS)
- Localize o arquivo apropriado em `internal/core/` (ex: `app_tools.go`).
- Defina o método na struct `App` com a primeira letra maiúscula.
- O Wails detectará a mudança e gerará automaticamente o binding em `frontend/wailsjs/go/`.

### 2. Ciclo de Testes
Sempre valide as alterações no motor de grafos e RAG antes de comitar:
```powershell
go test ./internal/rag/...
```

---

## 🔗 Documentos Relacionados

- [[BACKEND_METHODS]] — Lista completa de funções disponíveis para o frontend.
- [[LUMAESTRO_CORE]] — Entenda a raiz do orquestrador.
- [[FRONTEND_GUIDE]] — Convenções de UI e State Management (Pinia).
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Código que constrói o futuro da inteligência. 💻⚙️💎**
