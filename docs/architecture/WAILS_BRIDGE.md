---
title: "Wails Bridge - A Ponte do Maestro"
type: "architecture"
status: "active"
tags: ["wails", "golang", "vuejs", "ipc", "bindings"]
---

# 🌉 Wails Bridge: A Sinfonia de Dados

> [!ABSTRACT]
> O **Wails** é o framework de orquestração que une o motor de alta performance em **Go** à interface imersiva em **Vue.js**. Ele é responsável por transformar métodos de sistema em APIs de frontend e garantir que o fluxo de eventos da IA seja entregue em tempo real.

## 🏗️ Arquitetura da Ponte (IPC)

A comunicação entre o cérebro (Backend) e os olhos (Frontend) do Lumaestro ocorre através de uma camada de **Inter-Process Communication (IPC)** ultra-veloz.

```mermaid
flowchart LR
    %% Estilos
    classDef be fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef fe fill:#9c27b0,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef bridge fill:#455a64,stroke:#fff,stroke-dasharray: 5 5,color:#fff

    subgraph GoCore [Córtex Backend: Go]
        direction TB
        M[fa:fa-server Methods/Bindings]
        E[fa:fa-broadcast-tower Event Bus]
    end

    subgraph WailsBridge [A Ponte de Wails]
        direction TB
        IPC{fa:fa-exchange-alt IPC Layer}
    end

    subgraph VueFront [Córtex Frontend: Vue 3]
        direction TB
        C[fa:fa-code API Calls]
        L[fa:fa-stream Event Listeners]
    end

    %% Fluxo Bi-direcional
    M <--> IPC <--> C
    E --> IPC --> L

    %% Estilos
    class M,E be
    class IPC bridge
    class C,L fe
```

---

## 🚀 Componentes da Sinfonia

### 1. Bindings de Funções (Go ↔ JS)
As funções de **IA**, **RAG** e **Gestão de Workspace** definidas em Go são exportadas automaticamente para o JavaScript. 
- **Exemplo**: `window.go.main.App.SendAgentInput(msg)` permite que o Chat dispare processos complexos de backend com uma única linha de código.

### 2. Barramento de Eventos (Real-time)
O Wails fornece um sistema de `EventsEmit` (Backend) e `EventsOn` (Frontend) que alimenta:
- **📊 Grafo 3D**: Atualização de posições de nós e sinapses sem travar a UI.
- **📟 Logs de Agente**: Transmissão bit-a-bit das respostas da IA.
- **🛡️ Alertas de Segurança**: Notificações instantâneas do protocolo ACP.

---

## 🛠️ Detalhes de Implementação

- **Wails Runtime**: Localizado no pacote `github.com/wailsapp/wails/v2/pkg/runtime`.
- **Assets**: A pasta `frontend/dist` é embutida no binário final do Go para portabilidade total.
- **Desenvolvimento**: O comando `wails dev` ativa o Hot-Reload em ambos os lados da ponte simultaneamente.

---

## 🔗 Documentos Relacionados

- [[LUMAESTRO_CORE]] — Como o App.go inicializa a ponte.
- [[FRONTEND_GUIDE]] — Como o Vue 3 consome os bindings.
- [[AGENTS_GUIDE]] — O uso da ponte para controle de terminais.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Conexão Nativa. Performance Digital. 🌉🐹⚙️**
