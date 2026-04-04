# Documentação do Projeto Lumaestro

O **Lumaestro** é um orquestrador de IA avançado que integra o seu "Segundo Cérebro" (Obsidian) com agentes de IA poderosos (como Gemini CLI e Claude Code). Ele oferece uma interface moderna para busca semântica, visualização de grafos de conhecimento e um terminal interativo integrado.

## 🏗️ Arquitetura do Sistema

O projeto é construído utilizando o framework **Wails**, que permite criar aplicações desktop nativas usando Go no backend e tecnologias web (Vue.js) no frontend.

### 1. Backend (Go)
O coração da aplicação reside no diretório `internal/`, organizado por responsabilidades:
- **`internal/agents/`**: Gerencia a execução de agentes CLI. Inclui suporte nativo a **Windows ConPTY** (`pty_windows.go`) para permitir um terminal interativo real dentro da aplicação.
- **`internal/rag/`**: Implementa o fluxo de *Retrieval-Augmented Generation*. Realiza a busca semântica, extrai contexto das notas do Obsidian e alimenta os agentes.
- **`internal/obsidian/`**: Contém o `crawler.go` para escanear e indexar o cofre (vault) do Obsidian.
- **`internal/provider/`**: Integrações com APIs externas:
    - **Qdrant**: Banco de dados vetorial para busca semântica.
    - **Google GenAI**: Geração de Embeddings (`gemini-embedding-2-preview`) e extração de ontologias (triplas Sujeito-Predicado-Objeto) usando Gemini 2.0 Flash.
- **`internal/tools/`**: Automatiza a instalação e configuração de dependências externas (como Gemini CLI e Claude Code).

### 2. Frontend (Vue.js 3)
Localizado em `frontend/src/`, utiliza uma interface moderna com efeito *Glassmorphism*:
- **ChatPanel.vue**: Interface de chat para interação com o RAG e agentes.
- **GraphVisualizer.vue**: Visualização dinâmica do grafo de conhecimento usando **D3.js**.
- **TerminalView.vue**: Terminal interativo integrado utilizando **xterm.js**, conectado diretamente ao backend via WebSockets/Wails.
- **Settings.vue**: Painel de configuração de chaves de API e caminhos de diretórios.

## 🚀 Tecnologias Utilizadas

- **Linguagem Principal**: [Go](https://go.dev/) (1.21+)
- **Framework Desktop**: [Wails v2](https://wails.io/)
- **Frontend**: [Vue.js 3](https://vuejs.org/) + [Vite](https://vitejs.dev/)
- **Banco de Dados Vetorial**: [Qdrant](https://qdrant.tech/) (via Docker)
- **IA/LLM**: Google Gemini 2.0 Flash & Embeddings
- **Visualização**: D3.js (Grafos) e xterm.js (Terminal)

## 🛠️ Instalação e Configuração

### Pré-requisitos
- **Go** (v1.21 ou superior)
- **Node.js** (v18 ou superior) & npm
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **Docker** (para rodar o Qdrant)

### Passos para Execução

1. **Subir o Banco de Dados Vetorial (Qdrant):**
   ```bash
   docker-compose up -d
   ```

2. **Instalar Dependências do Frontend:**
   ```bash
   cd frontend
   npm install
   cd ..
   ```

3. **Executar em Modo de Desenvolvimento:**
   ```bash
   wails dev
   ```

4. **Gerar o Executável Final:**
   ```bash
   wails build
   ```

## 📖 Funcionalidades Principais

1. **Terminal Maestro**: Um terminal real integrado que permite rodar agentes como `gemini` ou `claude` com suporte a interatividade completa no Windows.
2. **Busca Semântica (RAG)**: Pergunte qualquer coisa sobre suas notas do Obsidian. O sistema encontrará o contexto relevante automaticamente usando vetores.
3. **Grafo de Conhecimento**: O sistema usa o Gemini 2.0 Flash para "ler" suas notas e extrair relações semânticas, transformando-as em um mapa estelar interativo de conexões.
4. **Auto-Installer**: Facilita a vida do usuário instalando automaticamente as ferramentas de CLI necessárias e configurando o PATH do sistema.

---
*Documentação gerada automaticamente para o projeto Lumaestro.*

---

## 📚 Documentos Relacionados
- [📖 Índice Geral](./DOCS_INDEX.md) — Hub central de documentação
- [🧠 NEURAL_BRAIN](./NEURAL_BRAIN.md) — Grafos, PageRank, Auditoria Lógica
- [🔗 WAILS_BRIDGE](./WAILS_BRIDGE.md) — Ponte Go ↔ Vue.js
- [🎨 FRONTEND_STACK](./FRONTEND_STACK.md) — Vue 3, D3.js, Xterm.js
- [⚡ LIGHTNING_ENGINE](./LIGHTNING_ENGINE.md) — DuckDB e aprendizado
- [🔄 RAG_FLOW](./RAG_FLOW.md) — Pipeline de busca vetorial
