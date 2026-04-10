# 🛠️ Guia do Desenvolvedor Lumaestro 🐹💻

Este guia contém as instruções para configurar, desenvolver e manter o ecossistema Lumaestro.

## 🚀 Setup Rápido

### Pré-requisitos
- **Go 1.21+**
- **Node.js 20+** (npm ou pnpm)
- **Wails CLI** (go install github.com/wailsapp/wails/v2/cmd/wails@latest)
- **DuckDB** (Binário deve estar no PATH ou pasta deps/)

### Comandos Úteis (PowerShell)

| Comando | Descrição |
|---------|-----------|
| ./dev.ps1 | Inicia o modo de desenvolvimento (Hot Reload Go + Vite). |
| ./build.ps1 | Compila o executável final para Windows. |
| go run scripts/setup_build_env.ps1 | Configura o ambiente de compilação. |

## 📁 Estrutura do Projeto

- /internal/core: O coração da aplicação (App struct e binding Wails).
- /internal/lightning: Motor de otimização de prompts e telemetria.
- /internal/orchestration: Lógica de enxame, orçamentos e handoff.
- /frontend/src: Interface Vue 3 com Tailwind e Glassmorphism.
- /docs: Base de conhecimento (Grounding do RAG).

## 🛠️ Adicionando Novos Métodos (Go -> Frontend)

1. Adicione o método no arquivo apropriado em internal/core/ (ex: pp_tools.go).
2. O método deve ser exportado (letra maiúscula) e pertencer à struct App.
3. Execute wails dev para gerar os bindings automáticos em rontend/wailsjs/go/.

## 🧪 Testes e Validação

Para rodar testes do motor de grafos:
`powershell
go test ./internal/rag/graph_engine_test.go
`

## 📝 Documentação
Sempre que adicionar uma nova funcionalidade no backend:
1. Atualize o docs/api/BACKEND_METHODS.md.
2. Verifique se há necessidade de uma nova página na pasta docs/architecture/.

---
[[INDEX|⬅️ Voltar ao Índice]]
