---
tags: [frontend, vue, vite, wails, ui]
type: technical-spec
status: active
---

# 🎨 Frontend Stack: A Interface Neural

A interface do Lumaestro é uma Single Page Application (SPA) moderna construída com **Vue 3** e **Vite**, integrada ao backend Go através do **Wails**. Ela foca em reatividade, visualização de dados complexos e controle de fluxo de agentes.

## 🛠️ Tecnologias Utilizadas

- **Framework**: Vue 3 (Composition API).
- **Build Tool**: Vite.
- **Estado**: Pinia (para gerenciamento de agentes e sessões).
- **Estilização**: CSS Nativo / Variáveis customizadas para modo Dark.
- **Gráficos**: D3.js / Mermaid.js (para visualização do Graph-RAG e Trajetórias).

---

## 🏗️ Estrutura de Pastas (rontend/src/)

- **components/**: Peças reutilizáveis da UI (Chat, ReviewBlock, Sidebar).
- **stores/**: Lógica de estado (ex: useAgentStore.js).
- **wailsjs/**: Bindings gerados automaticamente pelo Wails (Ponte Go -> JS).
- **engines/**: Lógica de processamento no lado do cliente (ex: processamento de Markdown).

## 🔌 A Ponte Wails (WailsJS)

O Frontend não faz requisições HTTP tradicionais. Ele chama funções do Go como se fossem funções assíncronas locais do JavaScript.

`javascript
// Exemplo de chamada no Vue
import { SendMessage } from "../../wailsjs/go/core/App";

async function handleSend() {
  try {
    await SendMessage(sessionId, text);
    // O backend processa e emite eventos via EventsEmit
  } catch (err) {
    console.error("Falha no Core:", err);
  }
}
`

---

## 🔄 Fluxo de Eventos (Event-Driven UI)

O Lumaestro utiliza um padrão de eventos para atualizar a interface em tempo real sem a necessidade de polling.

`mermaid
sequenceDiagram
    participant B as Backend (Go)
    participant E as Wails Events
    participant F as Frontend (Vue)

    B->>E: EventsEmit("agent_thought", chunk)
    E->>F: On("agent_thought")
    F->>F: Atualiza Store do Chat
    F->>F: Renderiza nova linha na UI
`

---

## 🛡️ Componentes Críticos

### 1. ReviewBlock.vue (O Guardião ACP)
Este componente é acionado sempre que um agente solicita uma aprovação. Ele bloqueia o chat e exige uma decisão binária (Aprovar/Rejeitar), exibindo o payload técnico da ação.

### 2. GraphCanvas.vue
Utiliza WebGL ou Canvas para renderizar a ontologia do sistema e as conexões entre documentos capturados pelo RAG.

---

## 🔗 Veja Também
- [[LUMAESTRO_CORE]]: Como os métodos JS chegam no Go.
- [[VISUAL_TRAJECTORIES]]: Como os dados são transformados em gráficos.
- [[ACP_MODE]]: Detalhes sobre o componente de revisão.

> [!TIP]
> Para testar mudanças no frontend rapidamente, use o comando 
pm run dev na pasta rontend/, que permite hot-reload enquanto o binário Wails está rodando.
