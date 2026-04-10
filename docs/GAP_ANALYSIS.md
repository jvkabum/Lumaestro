# 🔍 Gap Analysis: Gemini CLI v0.37 vs Lumaestro (ACP Mode)

> **Base**: Documentação oficial completa de https://geminicli.com/docs/  
> **Alvo**: Lumaestro Cognitive Engine (Wails + Go + Vue.js, modo ACP)  
> **Data**: 2026-04-10  

---

## Índice
1. [Resumo Executivo](#resumo-executivo)
2. [Matriz de Funcionalidades](#matriz-de-funcionalidades)
3. [Ferramentas (Tools) — Análise Detalhada](#ferramentas-tools)
4. [Funcionalidades Já Implementadas ✅](#já-implementadas-)
5. [Parcialmente Implementadas ⚠️](#parcialmente-implementadas-️)
6. [Não Implementadas — Alta Prioridade 🔴](#não-implementadas--alta-prioridade-)
7. [Não Implementadas — Média Prioridade 🟡](#não-implementadas--média-prioridade-)
8. [Não Aplicável / Baixa Prioridade ⚪](#não-aplicável--baixa-prioridade-)
9. [Roadmap Sugerido](#roadmap-sugerido)

---

## Resumo Executivo

| Categoria | Qtd |
|---|---|
| ✅ Implementado | 12 |
| ⚠️ Parcial | 8 |
| 🔴 Gap Crítico (Alta) | 7 |
| 🟡 Gap Médio | 9 |
| ⚪ Não Aplicável / Baixo | 6 |

O Lumaestro cobre **~55%** das funcionalidades core do Gemini CLI em modo ACP. As lacunas mais críticas estão em: **Plan Mode**, **Subagents**, **Hooks**, **Checkpointing**, **Model Routing completo**, **Extensions** e **Tool Discovery**.

---

## Matriz de Funcionalidades

| # | Funcionalidade | Doc Oficial | Lumaestro | Status |
|---|---|---|---|---|
| 1 | ACP Mode (JSON-RPC / IPC) | [acp-mode](https://geminicli.com/docs/cli/acp-mode/) | `executor.go`, `handler.go`, `rpc_listener.go` | ✅ |
| 2 | Session Management | [session-management](https://geminicli.com/docs/cli/tutorials/session-management/) | `session.go` + auto-restore | ✅ |
| 3 | Authentication (OAuth + API Key) | [authentication](https://geminicli.com/docs/get-started/authentication/) | `session.go` (OAuth silent + API Key pool) | ✅ |
| 4 | Model Selection (`--model`, env var) | [model](https://geminicli.com/docs/cli/model/) | `executor.go` (flag `--model`) | ✅ |
| 5 | YOLO Mode (Auto-approve) | [configuration](https://geminicli.com/docs/reference/configuration/) | `executor.go` (`--yolo`) | ✅ |
| 6 | File System Tools | [file-system](https://geminicli.com/docs/tools/file-system) | Handler processa `tool_use` chunks | ⚠️ |
| 7 | Shell Commands (`run_shell_command`) | [shell](https://geminicli.com/docs/cli/tutorials/shell-commands/) | Auto-approve via `--yolo` | ⚠️ |
| 8 | Memory / `save_memory` | [memory](https://geminicli.com/docs/tools/memory) | RAG via `ConsolidateChatKnowledge` | ⚠️ |
| 9 | GEMINI.md (Project Context) | [gemini-md](https://geminicli.com/docs/cli/gemini-md/) | Não gerenciado pela UI | ⚠️ |
| 10 | Web Search (`google_web_search`) | [web-search](https://geminicli.com/docs/tools/web-search) | Nativo via CLI, sem UI específica | ⚠️ |
| 11 | Web Fetch (`web_fetch`) | [web-fetch](https://geminicli.com/docs/tools/web-fetch) | Nativo via CLI | ⚠️ |
| 12 | Telemetry / Stats (`/stats`) | [telemetry](https://geminicli.com/docs/cli/telemetry/) | `telemetry.go` + dashboard frontend | ✅ |
| 13 | Model Routing (Fallback) | [model-routing](https://geminicli.com/docs/cli/model-routing/) | Rotação de chaves custom | ⚠️ |
| 14 | Token Caching | [token-caching](https://geminicli.com/docs/cli/token-caching/) | Sem implementação | 🔴 |
| 15 | Checkpointing | [checkpointing](https://geminicli.com/docs/cli/checkpointing/) | Sem implementação | 🔴 |
| 16 | Plan Mode | [plan-mode](https://geminicli.com/docs/cli/plan-mode/) | Sem implementação | 🔴 |
| 17 | Model Steering 🔬 | [model-steering](https://geminicli.com/docs/cli/model-steering/) | Sem implementação | 🔴 |
| 18 | Subagents | [subagents](https://geminicli.com/docs/core/subagents/) | Sem implementação | 🔴 |
| 19 | Remote Subagents | [remote-agents](https://geminicli.com/docs/core/remote-agents/) | Sem implementação | 🟡 |
| 20 | Hooks (Pre/Post Tool) | [hooks](https://geminicli.com/docs/hooks/) | Sem implementação | 🔴 |
| 21 | Agent Skills | [skills](https://geminicli.com/docs/cli/skills/) | Sem implementação | 🔴 |
| 22 | Extensions | [extensions](https://geminicli.com/docs/extensions/) | Sem implementação | 🟡 |
| 23 | MCP Servers | [mcp-server](https://geminicli.com/docs/tools/mcp-server/) | Sem implementação | 🟡 |
| 24 | Custom Commands | [custom-commands](https://geminicli.com/docs/cli/custom-commands/) | Sem implementação | 🟡 |
| 25 | Rewind | [rewind](https://geminicli.com/docs/cli/rewind/) | Sem implementação | 🟡 |
| 26 | Sandboxing | [sandbox](https://geminicli.com/docs/cli/sandbox/) | Sem implementação | 🟡 |
| 27 | Notifications 🔬 | [notifications](https://geminicli.com/docs/cli/notifications/) | Sem implementação | 🟡 |
| 28 | Headless Mode | [headless](https://geminicli.com/docs/cli/headless/) | Sem implementação | 🟡 |
| 29 | Settings UI (`/settings`) | [settings](https://geminicli.com/docs/cli/settings/) | `settings.json` manual | ⚠️ |
| 30 | `.geminiignore` | [gemini-ignore](https://geminicli.com/docs/cli/gemini-ignore/) | Sem gerenciamento | 🟡 |
| 31 | Themes | [themes](https://geminicli.com/docs/cli/themes/) | UI customizada própria | ✅ |
| 32 | Keyboard Shortcuts | [keyboard-shortcuts](https://geminicli.com/docs/reference/keyboard-shortcuts/) | Parcial na UI Vue | ⚠️ |
| 33 | System Prompt Override | [system-prompt](https://geminicli.com/docs/cli/system-prompt/) | Sem implementação | ⚪ |
| 34 | Enterprise Config | [enterprise](https://geminicli.com/docs/cli/enterprise/) | N/A | ⚪ |
| 35 | Policy Engine | [policy-engine](https://geminicli.com/docs/reference/policy-engine/) | Sem implementação | ⚪ |
| 36 | Git Worktrees 🔬 | [git-worktrees](https://geminicli.com/docs/cli/git-worktrees/) | N/A | ⚪ |
| 37 | Memory Import (Memport) | [memport](https://geminicli.com/docs/reference/memport/) | Sem implementação | ⚪ |
| 38 | Trusted Folders | [trusted-folders](https://geminicli.com/docs/cli/trusted-folders/) | Não precisa (desktop app) | ⚪ |
| 39 | Resiliência (Error 429/500) | Parcial em model-routing | `executor.go` (rotação de chaves/modelos) | ✅ |
| 40 | Histórico Persistente | Nativo Gemini | `session.go` (auto-restore) + SQLite | ✅ |
| 41 | Multi-agente (Swarm) | Não oficial | `app_swarm.go` | ✅ |
| 42 | Grafo 3D / RAG Visual | Não oficial | `app_graph.go` (planejado) | ✅ |

---

## Ferramentas (Tools)

### Ferramentas que o Gemini CLI oferece nativamente:

| Categoria | Ferramenta | Disponível no ACP? | Lumaestro Renderiza? |
|---|---|---|---|
| **Shell** | `run_shell_command` | ✅ via `tool_use` | ⚠️ Output bruto apenas |
| **File System** | `read_file` | ✅ | ❌ Sem preview de arquivo |
| | `read_many_files` | ✅ | ❌ |
| | `write_file` | ✅ | ⚠️ Sem diff visual |
| | `replace` | ✅ | ⚠️ Sem diff visual |
| | `list_directory` | ✅ | ❌ Sem tree view |
| | `glob` | ✅ | ❌ |
| | `grep_search` / `search_file_content` | ✅ | ❌ |
| **Web** | `google_web_search` | ✅ | ❌ Sem card de resultados |
| | `web_fetch` | ✅ | ❌ |
| **Interaction** | `ask_user` | ✅ | ❌ Sem UI modal |
| | `write_todos` | ✅ | ❌ Sem painel de TODOs |
| **Memory** | `save_memory` | ✅ | ⚠️ RAG diferente |
| | `get_internal_docs` | ✅ | ❌ |
| | `activate_skill` | ✅ | ❌ |
| **Planning** | `enter_plan_mode` | ✅ | ❌ |
| | `exit_plan_mode` | ✅ | ❌ |
| **System** | `complete_task` | ✅ | ❌ Sem sinalização visual |

> **⚠️ IMPORTANTE**  
> O Gemini CLI em modo ACP envia todas as chamadas de ferramentas como eventos `tool_use` via JSON-RPC. O Lumaestro **recebe** esses eventos no `handler.go`, mas a maioria é processada apenas como texto bruto, sem UI dedicada.

---

## Já Implementadas ✅

| Funcionalidade | Módulo | Detalhes |
|---|---|---|
| ACP Mode (JSON-RPC/ndJSON) | `executor.go` + `rpc_listener.go` | Pipe IPC completo, parsing de chunks |
| Session Lifecycle | `session.go` | Start/Stop/Resume com auto-restore |
| OAuth + API Key Auth | `session.go` | Silent login + pool de chaves rotativas |
| Model Selection | `executor.go` | Flag `--model`, env `GEMINI_MODEL` |
| YOLO Auto-Approve | `executor.go` | Flag `--yolo` para modo não-interativo |
| Resiliência 429/500 | `executor.go` | Detecção + rotação automática modelo/chave |
| Telemetria / Stats | `telemetry.go` | Tracking de tokens, custos, latência |
| Histórico Persistente | `session.go` + DB | Auto-restore + renderização user/assistant |
| UI Themes | Frontend Vue | Dark mode nativo, design premium |
| Multi-agente (Swarm) | `app_swarm.go` | Orquestração proprietária |
| Grafo RAG | `app_graph.go` | Visualização 3D (planejada) |
| Chat Streaming | `handler.go` | Real-time chunk processing |

---

## Parcialmente Implementadas ⚠️

### 1. File System Tools
- **O que tem**: Handler captura `tool_use` do tipo file system
- **O que falta**: 
  - Renderização visual de diffs (antes/depois)
  - File tree navigator na UI
  - Preview de arquivo inline
  - Confirmação visual antes de write

### 2. Shell Commands
- **O que tem**: Auto-approve via `--yolo`
- **O que falta**:
  - Terminal embutido na UI para output
  - Confirmação granular por comando
  - Histórico de comandos executados

### 3. Memory / RAG
- **O que tem**: `ConsolidateChatKnowledge` no backend
- **O que falta**:
  - Alinhamento com `save_memory` → `GEMINI.md`
  - UI para visualizar memories salvos
  - Import/export de memórias (Memport)

### 4. GEMINI.md
- **O que tem**: O Gemini CLI lê automaticamente
- **O que falta**:
  - Editor de GEMINI.md na UI
  - Geração automática de contexto de projeto
  - Suporte a hierarquia (global, user, project)

### 5. Web Search/Fetch
- **O que tem**: Funciona nativamente via CLI
- **O que falta**:
  - Cards visuais de resultados de busca
  - Preview de páginas fetched
  - Indicador visual quando web tools são usadas

### 6. Model Routing
- **O que tem**: Rotação custom de chaves e modelos
- **O que falta**:
  - `ModelAvailabilityService` equivalente
  - Fallback chain oficial: `flash-lite → flash → pro`
  - UI para configurar precedência de modelos
  - Local Model Routing (Gemma)

### 7. Settings
- **O que tem**: `settings.json` manipulado manualmente
- **O que falta**:
  - UI de settings equivalente ao `/settings`
  - Toggle de features experimentais
  - Configuração visual de modelos/auth

### 8. Keyboard Shortcuts
- **O que tem**: Atalhos básicos no Vue
- **O que falta**:
  - `Shift+Tab` para cycling de approval modes
  - `Ctrl+X` para editor externo
  - Atalhos de navegação de sessão

---

## Não Implementadas — Alta Prioridade 🔴

### 1. 🔴 Plan Mode
**Impacto**: Crítico para workflows complexos de desenvolvimento

O Gemini CLI oferece um modo de planejamento read-only completo:
- **Entrada**: `/plan [goal]`, `Shift+Tab`, ou linguagem natural
- **Ferramentas restritas**: Apenas leitura (read_file, grep, web_search, etc.)
- **Planos em Markdown**: Salvos em `~/.gemini/tmp/<project>/<session>/plans/`
- **Edição colaborativa**: `Ctrl+X` abre editor externo, o modelo lê as edições
- **Aprovação formal**: Opções de auto-edit ou manual-edit após aprovação
- **Policy engine**: Regras custom em TOML para controlar o que Plan Mode pode fazer

**Implementação sugerida**:
```
Frontend: Novo componente PlanMode.vue
   - Toggle visual Plan/Execute
   - Renderização de plano Markdown
   - Botões Approve/Iterate/Cancel
Backend: Novo módulo plan.go
   - Intercepta mode switching
   - Filtra ferramentas por modo
   - Gerencia diretório de planos
```

---

### 2. 🔴 Subagents
**Impacto**: Delegação de tarefas paralelas

O Gemini CLI suporta subagentes nativos:
- **`codebase_investigator`**: Análise profunda de codebase
- **`cli_help`**: Assistente de ajuda CLI
- **Custom subagents**: Definidos em `~/.gemini/agents/`
- **Comunicação**: Cada subagent roda em contexto isolado

**Implementação sugerida**:
```
Backend: Módulo subagent_manager.go
   - Spawn de instâncias ACP secundárias
   - Context isolation por subagent
   - Comunicação inter-agente via channels
Frontend: SubagentPanel.vue
   - Visualização de subagents ativos
   - Output multiplexado por agente
```

---

### 3. 🔴 Hooks (Pre/Post Tool Execution)
**Impacto**: Automação e controle de qualidade

Sistema de hooks documentado:
- **Pre-hooks**: Executados antes de uma ferramenta (ex: lint antes de write)
- **Post-hooks**: Executados após (ex: format após write)
- **Configuração**: Via `hooks.json` ou `~/.gemini/hooks/`
- **Triggers**: Por ferramenta, por evento, por padrão de arquivo

**Implementação sugerida**:
```
Backend: hooks.go
   - Registry de hooks por evento
   - Pipeline pre → tool → post
   - Timeout e error handling
Config: hooks.json em ~/.gemini/
   - Definição de hooks por tool
```

---

### 4. 🔴 Checkpointing (Git Snapshots)
**Impacto**: Safety net para todas as modificações de arquivo

Funcionalidade documentada:
- **Shadow Git repo**: `~/.gemini/history/<project_hash>/`
- **Auto-snapshot**: Antes de qualquer `write_file` ou `replace`
- **Restore**: `/restore <checkpoint_file>`
- **Inclui**: Estado de arquivos + conversa + tool call
- **Config**: `settings.json` → `general.checkpointing.enabled: true`

**Implementação sugerida**:
```
Backend: checkpoint.go
   - Shadow git init/commit automático
   - Serialização de estado de conversa
   - Restore de snapshot + replay
Frontend: CheckpointPanel.vue
   - Timeline visual de checkpoints
   - Botão restore com preview de diff
```

---

### 5. 🔴 Model Steering 🔬
**Impacto**: Correção em tempo real durante execução

Funcionalidade experimental:
- **Input durante execução**: Digitar enquanto o agente trabalha
- **Acknowledgment rápido**: Modelo pequeno confirma recebimento
- **Context injection**: Hint injetado no próximo turn
- **Config**: `settings.json` → `experimental.modelSteering: true`
- **Casos de uso**: Corrigir caminho, pular etapas, adicionar contexto

**Implementação sugerida**:
```
Backend: steering.go
   - Canal de input paralelo durante execução
   - Injeção de hint no próximo request
   - Acknowledgment assíncrono
Frontend:
   - Input overlay durante streaming
   - Badge visual "Steering active"
```

---

### 6. 🔴 Agent Skills
**Impacto**: Expertise especializada sob demanda

Sistema de skills documentado:
- **Localização**: `.gemini/skills/` no projeto ou global
- **Ativação**: `activate_skill` tool ou automático por contexto
- **Formato**: Markdown com instruções procedurais
- **Integração com Plan Mode**: Skills guiam planejamento

**Implementação sugerida**:
```
Backend: skills.go
   - Scanner de diretório de skills
   - Injeção de instrução no system prompt
   - Ativação por contexto ou manual
Frontend: SkillsPanel.vue
   - Lista de skills disponíveis
   - Toggle ativo/inativo
   - Editor de skills custom
```

---

### 7. 🔴 Token Caching
**Impacto**: Otimização de custos (reduz consumo de API)

Funcionalidade nativa:
- **Disponível para**: API Key e Vertex AI (não OAuth)
- **Automático**: Reutiliza system instructions e contexto
- **Monitoramento**: Via `/stats` mostra cached tokens
- **Impacto**: Redução significativa de custo em sessões longas

**Implementação sugerida**:
```
Backend: Ajustar session.go
   - Detectar tipo de auth e habilitar caching
   - Tracking de cache hits/misses
Frontend: Dashboard de telemetria
   - Mostrar economia de tokens por cache
   - Gráfico cumulativo de savings
```

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
- **Impacto**: Atalhos para workflows repetitivos
- **Complexidade**: Baixa

### 4. Rewind
- **O que é**: Desfazer operações do agente
- **Impacto**: UX para correção rápida
- **Complexidade**: Média (requer Checkpointing primeiro)

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
| System Prompt Override | Lumaestro injeta system prompt internamente |
| Enterprise Configuration | N/A para uso pessoal |
| Policy Engine (TOML) | Substituído por lógica Go customizada |
| Git Worktrees 🔬 | Feature experimental, baixo impacto |
| Memory Import (Memport) | RAG próprio substitui |
| Trusted Folders | Desktop app já tem confiança implícita |

---

## Roadmap Sugerido

### Fase 1: Fundações de Segurança (1-2 semanas)
- [ ] **Checkpointing** → Shadow git + restore
- [ ] **Token Caching** → Dashboard de economia
- [ ] **Model Routing completo** → Fallback chain oficial

### Fase 2: Planejamento Inteligente (2-3 semanas)
- [ ] **Plan Mode** → UI + tool filtering + planos em markdown
- [ ] **Agent Skills** → Scanner + ativação + UI
- [ ] **GEMINI.md Editor** → UI para gerenciar contexto de projeto

### Fase 3: Orquestração Avançada (3-4 semanas)
- [ ] **Subagents** → Manager + UI multiplexada
- [ ] **Hooks** → Pipeline pre/post tool
- [ ] **Model Steering** → Input overlay durante execução

### Fase 4: Ecossistema (4+ semanas)
- [ ] **MCP Servers** → Configuração + registry
- [ ] **Extensions** → Sistema de plugins
- [ ] **Custom Commands** → Atalhos personalizados
- [ ] **Notifications** → Toast de conclusão

### Fase 5: Visualização de Ferramentas (Contínuo)
- [ ] **Diff Viewer** → Para `write_file`/`replace`
- [ ] **File Tree** → Para `list_directory`/`glob`
- [ ] **Search Results** → Cards para `grep`/`web_search`
- [ ] **ask_user Modal** → UI interativa de confirmação
- [ ] **TODO Panel** → Para `write_todos`
- [ ] **Terminal embutido** → Para `run_shell_command`

---

> **⚠️ Decisões Arquiteturais Pendentes**
> 1. **Checkpointing**: Usar shadow git (como Gemini CLI) ou snapshots SQLite?
> 2. **Plan Mode**: Implementar como modo no ACP (`--approval-mode=plan`) ou como feature da UI?
> 3. **Subagents**: Spawnar processos ACP separados ou reusar a mesma instância?
> 4. **MCP**: Integrar via settings.json do Gemini ou gerenciar independentemente?
> 5. **Skills**: Usar formato `.gemini/skills/` compatível ou formato proprietário?

---

> **💡 Recomendação Estratégica**  
> Focar primeiro na **Fase 1** (Checkpointing + Token Caching + Model Routing) para construir uma base sólida de segurança e economia. Depois, a **Fase 2** (Plan Mode + Skills) é o maior diferencial competitivo para produtividade. A **Fase 5** (visualização de ferramentas) pode ser feita incrementalmente entre as outras fases, pois melhora a UX a cada iteração.
