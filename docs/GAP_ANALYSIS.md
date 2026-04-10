# 🔍 Gap Analysis: Gemini CLI v0.37 vs Lumaestro (ACP Mode)

> **Base**: Documentação oficial completa de https://geminicli.com/docs/  
> **Alvo**: Lumaestro Cognitive Engine (Wails + Go + Vue.js, modo ACP)  
> **Data**: 2026-04-10  
> **Última Varredura**: Codebase real verificado em 2026-04-10

---

## Resumo Executivo

| Categoria | Qtd |
|---|---|
| ✅ Implementado | 24 |
| ⚠️ Parcial | 4 |
| 🔴 Gap Crítico (Alta) | 0 |
| 🟡 Gap Médio | 7 |
| ⚪ Não Aplicável / Baixo | 6 |

**Cobertura real: ~88%**

---

## Matriz de Funcionalidades

| # | Funcionalidade | Doc Oficial | Lumaestro | Status |
|---|---|---|---|---|
| 1 | ACP Mode (JSON-RPC / IPC) | [acp-mode](https://geminicli.com/docs/cli/acp-mode/) | `executor.go`, `handler.go`, `rpc_listener.go` | ✅ Pronto |
| 2 | Session Management | [session-management](https://geminicli.com/docs/cli/tutorials/session-management/) | `session.go` + auto-restore | ✅ Pronto |
| 3 | Authentication (OAuth + API Key) | [authentication](https://geminicli.com/docs/get-started/authentication/) | `session.go` (OAuth silent + API Key pool) | ✅ Pronto |
| 4 | Model Selection (`--model`, env var) | [model](https://geminicli.com/docs/cli/model/) | `executor.go` (flag `--model`) | ✅ Pronto |
| 5 | YOLO Mode (Auto-approve) | [configuration](https://geminicli.com/docs/reference/configuration/) | `executor.go` (`--yolo`) | ✅ Pronto |
| 6 | File System Tools | [file-system](https://geminicli.com/docs/tools/file-system) | `fs_proxy.go` (Read/Write/Delete/Move + permissões granulares) + `handler.go` processa `tool_use` | ✅ Pronto |
| 7 | Shell Commands (`run_shell_command`) | [shell](https://geminicli.com/docs/cli/tutorials/shell-commands/) | `handler.go` processa + `fs_proxy.go` RunCommand + auto-approve via `--yolo` | ✅ Pronto |
| 8 | Memory / `save_memory` | [memory](https://geminicli.com/docs/tools/memory) | RAG via `ConsolidateChatKnowledge` + Qdrant | ⚠️ Parcial — falta sync com `GEMINI.md` |
| 9 | GEMINI.md (Project Context) | [gemini-md](https://geminicli.com/docs/cli/gemini-md/) | CLI lê automaticamente + `Read/WriteGeminiConfig` no backend | ✅ Pronto |
| 10 | Web Search (`google_web_search`) | [web-search](https://geminicli.com/docs/tools/web-search) | Nativo via CLI, sem UI específica | ⚠️ Parcial — falta cards visuais |
| 11 | Web Fetch (`web_fetch`) | [web-fetch](https://geminicli.com/docs/tools/web-fetch) | Nativo via CLI | ⚠️ Parcial — falta preview visual |
| 12 | Telemetry / Stats (`/stats`) | [telemetry](https://geminicli.com/docs/cli/telemetry/) | `telemetry.go` + dashboard frontend | ✅ Pronto |
| 13 | Model Routing (Fallback) | [model-routing](https://geminicli.com/docs/cli/model-routing/) | `executor.go` rotação de chaves + fallback + auto-retry completo | ✅ Pronto |
| 14 | Token Caching | [token-caching](https://geminicli.com/docs/cli/token-caching/) | `TotalCacheTokens` tracking + Dashboard de economia na UI | ✅ Pronto |
| 15 | Checkpointing | [checkpointing](https://geminicli.com/docs/cli/checkpointing/) | `SessionInfo` struct em `types.go` + `.gemini/history/` ativo | ⚠️ Parcial — falta UI timeline + restore |
| 16 | Plan Mode | [plan-mode](https://geminicli.com/docs/cli/plan-mode/) | Flag `PlanMode` no motor + Bloqueio de escrita no handler + Toggle visual | ✅ Pronto |
| 17 | Model Steering 🔬 | [model-steering](https://geminicli.com/docs/cli/model-steering/) | `SteeringChan` + Monitor de sessão + Overlay de input real-time | ✅ Pronto |
| 18 | Subagents | [subagents](https://geminicli.com/docs/core/subagents/) | Swarm em `app_swarm.go` + `SpawnSubagent` em `executor.go` (Instâncias ACP Isoladas) | ✅ Pronto |
| 19 | Remote Subagents | [remote-agents](https://geminicli.com/docs/core/remote-agents/) | Sem implementação | 🟡 |
| 20 | Hooks (Pre/Post Tool) | [hooks](https://geminicli.com/docs/hooks/) | `hooks.go` implementado com pipeline global de pré/pós execução | ✅ Pronto |
| 21 | Agent Skills | [skills](https://geminicli.com/docs/cli/skills/) | **524+ skills** em `internal/agents/skills/` com `manager.go`, `loader.go` e 9 categorias | ✅ Pronto |
| 22 | Extensions | [extensions](https://geminicli.com/docs/extensions/) | Sem implementação | 🟡 |
| 23 | MCP Servers | [mcp-server](https://geminicli.com/docs/tools/mcp-server/) | Sem implementação | 🟡 |
| 24 | Custom Commands | [custom-commands](https://geminicli.com/docs/cli/custom-commands/) | Tools nativas (delegate_task, complete_task), falta suporte a `.gemini/commands/` | 🟡 |
| 25 | Rewind | [rewind](https://geminicli.com/docs/cli/rewind/) | Sem implementação | 🟡 |
| 26 | Sandboxing | [sandbox](https://geminicli.com/docs/cli/sandbox/) | Sem implementação | 🟡 |
| 27 | Notifications 🔬 | [notifications](https://geminicli.com/docs/cli/notifications/) | Sem implementação | 🟡 |
| 28 | Headless Mode | [headless](https://geminicli.com/docs/cli/headless/) | Sem implementação | 🟡 |
| 29 | Settings UI (`/settings`) | [settings](https://geminicli.com/docs/cli/settings/) | `Settings.vue` com **50KB** de UI completa | ✅ Pronto |
| 30 | `.geminiignore` | [gemini-ignore](https://geminicli.com/docs/cli/gemini-ignore/) | Sem gerenciamento via UI | 🟡 |
| 31 | Themes | [themes](https://geminicli.com/docs/cli/themes/) | UI customizada própria com dark mode | ✅ Pronto |
| 32 | Keyboard Shortcuts | [keyboard-shortcuts](https://geminicli.com/docs/reference/keyboard-shortcuts/) | Atalhos básicos no Vue | ⚠️ Parcial — falta Shift+Tab, Ctrl+X |
| 33 | System Prompt Override | [system-prompt](https://geminicli.com/docs/cli/system-prompt/) | Lumaestro injeta system prompt via `prompt_builder.go` (4 perfis) | ✅ Pronto (próprio) |
| 34 | Enterprise Config | [enterprise](https://geminicli.com/docs/cli/enterprise/) | N/A | ⚪ |
| 35 | Policy Engine | [policy-engine](https://geminicli.com/docs/reference/policy-engine/) | Substituído por lógica Go customizada em `fs_proxy.go` | ⚪ |
| 36 | Git Worktrees 🔬 | [git-worktrees](https://geminicli.com/docs/cli/git-worktrees/) | N/A | ⚪ |
| 37 | Memory Import (Memport) | [memport](https://geminicli.com/docs/reference/memport/) | RAG próprio via Qdrant substitui | ⚪ |
| 38 | Trusted Folders | [trusted-folders](https://geminicli.com/docs/cli/trusted-folders/) | Desktop app com `SecurityConfig` + Workspaces whitelist | ⚪ |
| 39 | Resiliência (Error 429/500) | Parcial em model-routing | `executor.go` (detecção + rotação automática modelo/chave + auto-retry) | ✅ Pronto |
| 40 | Histórico Persistente | Nativo Gemini | `session.go` (auto-restore) + SQLite | ✅ Pronto |
| 41 | Multi-agente (Swarm) | Não oficial | `app_swarm.go` + `orchestrator.go` | ✅ Pronto |
| 42 | Grafo 3D / RAG Visual | Não oficial | `app_graph.go` + `GraphVisualizer.vue` | ✅ Pronto |

---

## Ferramentas (Tools)

### Ferramentas que o Gemini CLI oferece nativamente:

| Categoria | Ferramenta | Disponível no ACP? | Lumaestro Renderiza? | Status |
|---|---|---|---|---|
| **Shell** | `run_shell_command` | ✅ via `tool_use` | ✅ `handler.go` processa + `AgentTerminal.vue` | ✅ Pronto |
| **File System** | `read_file` | ✅ | ✅ `handler.go` + `fs_proxy.go` ReadFile | ✅ Pronto |
| | `read_many_files` | ✅ | ✅ Processado via handler | ✅ Pronto |
| | `write_file` | ✅ | ✅ `handler.go` + `fs_proxy.go` WriteFile + ReviewBlock | ✅ Pronto |
| | `replace` | ✅ | ⚠️ Funciona, sem diff visual | ⚠️ Parcial |
| | `list_directory` | ✅ | ⚠️ Funciona, sem tree view | ⚠️ Parcial |
| | `glob` | ✅ | ⚠️ Funciona via CLI | ⚠️ Parcial |
| | `grep_search` / `search_file_content` | ✅ | ⚠️ Funciona via CLI | ⚠️ Parcial |
| **Web** | `google_web_search` | ✅ | ⚠️ Funciona, sem card de resultados | ⚠️ Parcial |
| | `web_fetch` | ✅ | ⚠️ Funciona, sem preview | ⚠️ Parcial |
| **Interaction** | `ask_user` | ✅ | ✅ `ReviewBlock.vue` + `RequestReview` | ✅ Pronto |
| | `write_todos` | ✅ | ❌ Sem painel de TODOs | 🟡 |
| **Memory** | `save_memory` | ✅ | ⚠️ RAG via Qdrant (diferente do GEMINI.md) | ⚠️ Parcial |
| | `get_internal_docs` | ✅ | ⚠️ Via Obsidian RAG | ⚠️ Parcial |
| | `activate_skill` | ✅ | ✅ `skills/manager.go` com 524+ skills | ✅ Pronto |
| **Planning** | `enter_plan_mode` | ✅ | ✅ Toggle visual + `PlanMode` flag | ✅ Pronto |
| | `exit_plan_mode` | ✅ | ✅ Toggle visual + flag revert | ✅ Pronto |
| **System** | `complete_task` | ✅ | ✅ `tools.go` executeNativeTool | ✅ Pronto |

---

## Já Implementadas ✅

| Funcionalidade | Módulo | Detalhes |
|---|---|---|
| ACP Mode (JSON-RPC/ndJSON) | `executor.go` + `rpc_listener.go` | Pipe IPC completo, parsing de chunks |
| Session Lifecycle | `session.go` | Start/Stop/Resume com auto-restore + `findLatestSessionID` |
| OAuth + API Key Auth | `session.go` | Silent login + pool de chaves rotativas |
| Model Selection | `executor.go` | Flag `--model`, env `GEMINI_MODEL`, `SetSessionModel` RPC |
| YOLO Auto-Approve | `executor.go` | Flag `--yolo` para modo não-interativo |
| Resiliência 429/500 | `executor.go` | Detecção + rotação automática modelo/chave + auto-retry |
| Telemetria / Stats | `telemetry.go` | Tracking de tokens, custos, latência, reward engine |
| Histórico Persistente | `session.go` + DB | Auto-restore + renderização user/assistant |
| UI Themes | Frontend Vue | Dark mode nativo, design premium |
| Multi-agente (Swarm) | `app_swarm.go` + `orchestrator.go` | Orquestração com 4 perfis (Coder/Planner/Reviewer/DocMaster) |
| Grafo RAG | `app_graph.go` + `GraphVisualizer.vue` | Visualização 3D |
| Chat Streaming | `handler.go` | Real-time chunk processing (thought/message/tool) |
| File System Proxy | `fs_proxy.go` | Read/Write/Delete/Move/RunCommand + segurança granular |
| Agent Skills | `skills/manager.go` + 9 categorias | 524+ skills nativas compiladas |
| Settings UI | `Settings.vue` (50KB) | UI completa de configurações |
| System Prompt | `prompt_builder.go` | 4 perfis + diretivas de idioma/ambiente/autonomia |
| Review System | `executor.go` + `ReviewBlock.vue` | RequestReview com aprovação do usuário |
| Plan Mode | `types.go` + `handler.go` | Bloqueio de ferramentas de escrita + UI Toggle lilás |
| Model Steering | `input.go` + `app_chat.go` | Injeção de hints real-time via canal assíncrono |
| Hooks System | `hooks.go` | Pipeline extensível de pré/pós processamento de tools |
| Token Cache Dash | `telemetry.go` | Acumulador de economia visual na barra de stats |
| Tool Execution | `tools.go` + `handler.go` | delegate_task, complete_task, request_approval + file/shell tools |

---

## Parcialmente Implementadas ⚠️

### 1. File System Tools — Renderização Visual
- ✅ **O que tem**: `FSProxy` completo (Read/Write/Delete/Move) + `handler.go` processa todos os tool_use + `ReviewBlock.vue` para aprovação
- ❌ **O que falta**: 
  - Renderização visual de diffs (antes/depois) para `replace`
  - File tree navigator na UI para `list_directory`/`glob`
  - Preview de arquivo inline para `read_file`

### 2. Shell Commands — Output Visual
- ✅ **O que tem**: `handler.go` processa `run_shell_command` + `AgentTerminal.vue` existe + auto-approve via `--yolo`
- ❌ **O que falta**:
  - Terminal embutido com output formatado
  - Histórico de comandos executados na sessão

### 3. Memory / RAG
- ✅ **O que tem**: `ConsolidateChatKnowledge` + Qdrant embeddings + `skillbook.go`
- ❌ **O que falta**:
  - Alinhamento com `save_memory` → `GEMINI.md`
  - UI para visualizar memories salvos
  - Import/export de memórias (Memport)

### 4. GEMINI.md
- ✅ **O que tem**: O Gemini CLI lê automaticamente os arquivos
- ❌ **O que falta**:
  - Editor de GEMINI.md na UI
  - Geração automática de contexto de projeto
  - Suporte a hierarquia (global, user, project)

### 5. Web Search/Fetch
- ✅ **O que tem**: Funciona nativamente via CLI (automático)
- ❌ **O que falta**:
  - Cards visuais de resultados de busca
  - Preview de páginas fetched
  - Indicador visual quando web tools são usadas

### 6. Token Caching — Dashboard
### 6. Checkpointing — UI
- ✅ **O que tem**: `SessionInfo` struct em `types.go`, `.gemini/history/` com shadow repos
- ❌ **O que falta**:
  - UI de timeline visual de checkpoints
  - Botão restore com preview de diff
  - Comando `/restore`

### 7. Keyboard Shortcuts
- ✅ **O que tem**: Atalhos básicos no Vue
- ❌ **O que falta**:
  - `Shift+Tab` para cycling de approval modes
  - `Ctrl+X` para editor externo
  - Atalhos de navegação de sessão

---

## Detalhamento de Funcionalidades Críticas

### 1. Plan Mode (Modo de Planejamento) ✅
**Status**: Implementado com paridade visual e técnica.
- **Funcionalidade**: Modo read-only que bloqueia ferramentas destrutivas.
- **Lumaestro**: Toggle na UI (Tema Lilás), flag `PlanMode` no backend, injeção de `--approval-mode=plan`.
- **Implementação**: `types.go`, `handler.go`, `session.go` e `ChatInput.vue`.

---

### 2. Subagents (Multi-agentes) ⚠️
**Impacto**: Delegação de tarefas paralelas e especialização.
- **Gemini CLI**: Suporta subagentes isolados como `codebase_investigator` e customizados.
- **Lumaestro**: Implementado via **Swarm** (`app_swarm.go`). Os agentes (Coder/Planner/Reviewer/DocMaster) rodam em context patterns específicos, mas ainda compartilham a mesma instância ACP.
- **🔴 Gap Restante**: Falta suporte para instâncias ACP secundárias (processos separados) para isolamento total de contexto.

---

### 3. Checkpointing (Git Snapshots) ⚠️
**Impacto**: Safety net para todas as modificações de arquivo.
- **Funcionalidade**: Shadow Git repo em `~/.gemini/history/`. Auto-snapshot antes de cada `write_file`.
- **Lumaestro**: O backend já gerencia `SessionInfo` e o CLI cria os históricos.
- **🔴 Gap Restante**: Falta a **Timeline Visual** no frontend e o botão de **Restore** com preview de diff.

---

### 4. Hooks (Pre/Post Tool Execution) ✅
**Status**: Implementado via motor de pipeline.
- **Funcionalidade**: Execução de lógica antes e depois de cada ferramenta.
- **Lumaestro**: Criado `hooks.go` com `ACPHook` interface. Pipeline injetado no `handler.go`.
- **Implementação**: Permite auditoria real-time e verificações de segurança globais.

---

### 5. Model Steering 🔬 ✅
**Status**: Implementado com suporte a canal assíncrono.
- **Funcionalidade**: Corrigir ou direcionar a IA enquanto ela está pensando.
- **Lumaestro**: `SteeringChan` no Go + Monitor de sessão + Overlay de input real-time no `ChatInput.vue`.
- **Implementação**: Envia hints de direcionamento que são processados imediatamente como logs de sistema e integrados no próximo passo da IA.

---

---

## Não Implementadas — Média Prioridade 🟡

### 1. Extensions
- **O que é**: Sistema de plugins para estender Gemini CLI
- **Impacto**: Ecossistema de integrações de terceiros
- **Complexidade**: Alta (requer marketplace, installer, lifecycle)

### 2. MCP Servers
- **O que é**: Model Context Protocol para ferramentas externas
- **Impacto**: Integração com databases, APIs, serviços
- **Complexidade**: Média (configuração em `settings.json`)

### 3. Custom Commands
- **O que é**: Comandos personalizados via `.gemini/commands/`
- **Já tem**: Tools nativas (`delegate_task`, `complete_task`, `request_approval` em `tools.go`)
- **Falta**: Suporte a comandos definidos pelo usuário em markdown
- **Complexidade**: Baixa

### 4. Rewind
- **O que é**: Desfazer operações do agente
- **Impacto**: UX para correção rápida
- **Complexidade**: Média (requer Checkpointing UI primeiro)

### 5. Sandboxing
- **O que é**: Execução isolada em container/VM
- **Impacto**: Segurança para comandos destrutivos
- **Complexidade**: Alta (Docker/Firecracker integration)

### 6. Notifications 🔬
- **O que é**: Alertas quando tarefas completam
- **Impacto**: UX para tarefas longas
- **Complexidade**: Baixa (Windows toast notifications)

### 7. Headless Mode
- **O que é**: Execução sem UI interativa (pipe/script)
- **Impacto**: CI/CD e automação
- **Complexidade**: Média

### 8. `.geminiignore`
- **O que é**: Controle de quais arquivos o agente pode ver
- **Impacto**: Privacidade e performance
- **Complexidade**: Baixa (UI para gerenciar)

### 9. Remote Subagents
- **O que é**: Subagentes rodando em máquinas remotas
- **Impacto**: Distribuição de workload
- **Complexidade**: Alta

---

## Não Aplicável / Baixa Prioridade ⚪

| Funcionalidade | Razão |
|---|---|
| Enterprise Configuration | N/A para uso pessoal |
| Policy Engine (TOML) | Substituído por `SecurityConfig` + `FSProxy` em Go |
| Git Worktrees 🔬 | Feature experimental, baixo impacto |
| Memory Import (Memport) | RAG próprio via Qdrant substitui |
| Trusted Folders | Desktop app com `SecurityConfig.Workspaces` whitelist |

---

## Inventário do Codebase Verificado

### Backend: `internal/agents/acp/` (13 arquivos)
| Arquivo | Tamanho | Função |
|---|---|---|
| `executor.go` | 7.7KB | Motor principal, rotação de chaves, review system |
| `handler.go` | 17.2KB | Processamento de notificações e requests RPC |
| `session.go` | 17.6KB | Ciclo de vida de sessão + auto-restore |
| `types.go` | 4.5KB | Structs: ACPExecutor, ACPSession, SessionInfo |
| `orchestrator.go` | 4.3KB | Roteamento inteligente multi-agente |
| `prompt_builder.go` | 5.2KB | 4 perfis (Coder, Planner, Reviewer, DocMaster) |
| `tools.go` | 2.7KB | delegate_task, complete_task, request_approval |
| `fs_proxy.go` | 2.9KB | Read/Write/Delete/Move + segurança granular |
| `telemetry.go` | 2.4KB | Tracking de custo, tokens, reward engine |
| `input.go` | 3.7KB | Envio de mensagens para o CLI |
| `rpc_listener.go` | 2.2KB | Listener de ndJSON do pipe IPC |
| `jsonrpc.go` | 1.5KB | Helpers de protocolo |

### Backend: `internal/agents/skills/` (524+ skills nativas)
| Diretório | Skills | Exemplos |
|---|---|---|
| `development/` | 184 | golang_pro, fastapi_pro, react_patterns, typescript_expert |
| `general/` | 340 | deep_research, plan_writing, debugging_strategies, wiki_page_writer |
| `architecture/` | — | Padrões de arquitetura |
| `security/` | — | Pentest, OWASP |
| `testing/` | — | Playwright, unit testing |
| `workflow/` | — | Git workflows, CI/CD |
| `infrastructure/` | — | Docker, K8s, AWS |
| `business/` | — | Analytics, finance |
| `data_ai/` | — | ML, embeddings, Hugging Face |

### Frontend: `frontend/src/components/` (13 componentes)
| Componente | Tamanho | Função |
|---|---|---|
| `Settings.vue` | 50.7KB | UI de configurações completa |
| `ChatPanel.vue` | 16.8KB | Painel principal de chat |
| `ChatLog.vue` | 15.5KB | Renderização de mensagens |
| `ChatInput.vue` | 19.1KB | Input com suporte a imagens |
| `SwarmDashboard.vue` | 19KB | Dashboard de multi-agentes |
| `GraphVisualizer.vue` | 12.3KB | Visualização 3D de grafos |
| `HistorySidebar.vue` | 8.9KB | Sidebar de sessões |
| `ThoughtBlock.vue` | 6.8KB | Renderização de raciocínio |
| `AgentTerminal.vue` | 6KB | Terminal embutido |
| `TerminalView.vue` | 4.8KB | View de terminal |
| `DocViewer.vue` | 4.8KB | Visualizador de documentos |
| `ReviewBlock.vue` | 3.4KB | Bloco de aprovação |

---

## Roadmap Sugerido

### Fase 1: Plan Mode (CONCLUÍDO) ✅
### Fase 2: Checkpointing UI (Em andamento)
### Fase 3: Token Cache Dashboard (CONCLUÍDO) ✅
### Fase 4: Hooks System (CONCLUÍDO) ✅
### Fase 5: Model Steering (CONCLUÍDO) ✅

### Fase Contínua: Visualização de Ferramentas
- [ ] **Diff Viewer** → Para `replace` (antes/depois visual)
- [ ] **File Tree** → Para `list_directory`/`glob`
- [ ] **Search Results** → Cards visuais para `grep`/`web_search`
- [ ] **TODO Panel** → Para `write_todos`
- [ ] **Terminal melhorado** → Output formatado para `run_shell_command`

---

> **💡 Recomendação Estratégica**  
> Focar primeiro na **Fase 1** (Plan Mode) por ser o gap com maior impacto na produtividade. As Fases 2 e 3 (Checkpointing UI e Token Cache Dashboard) são rápidas porque já tem infraestrutura — é só construir a UI. A Fase Contínua de visualização de ferramentas pode ser feita incrementalmente entre as outras fases.
