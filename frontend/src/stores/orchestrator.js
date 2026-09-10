import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { useSettingsStore } from './settings';

// Helper para chamar funções do Wails com segurança
const safeCall = async (pkg, func, ...args) => {
  try {
    // 🚀 SUPORTE MODULAR: Tenta encontrar a função no pacote core (novo) ou main (legado)
    const bridge = (window.go && window.go.core && window.go.core.App) || 
                   (window.go && window.go.main && window.go.main.App);
                   
    if (bridge && bridge[func]) {
      return await bridge[func](...args);
    }
    console.warn(`[Wails SafeCall] Função ${func} não encontrada em core ou main`);
    return null;
  } catch (err) {
    console.error(`[Wails SafeCall] Erro ao chamar ${func}:`, err);
    throw err;
  }
};

export const useOrchestratorStore = defineStore('orchestrator', () => {
  const settingsStore = useSettingsStore();
  const messages = ref([]);
  const isThinking = ref(false);
  const isNavigating = ref(false); // 🔍 Inteligência de Navegação em Tempo Real
  const isTerminalMode = ref(false);
  const isWeaving = ref(false); // 🧶 Teccelagem de Conhecimento em Background
  const activeAgent = ref('antigravity'); // 🚀 Motor Soberano Antigravity CLI
  const activeProfile = ref(null); // 🎭 Perfil de Agente (Doc-Master, etc) - Começa limpo
  const currentStatus = ref(""); // 📡 Status de Ação em Tempo Real
  const currentStatusKind = ref('status');
  const statusTimeline = ref([]); // 🪟 Janela de atividade (histórico curto)
  const statusFilter = ref('all');
  const runningSessions = ref([]);
  const lastTurnCompleteByAgent = ref({});
  const listenersInitialized = ref(false);
  const modelStats = ref({ agent: null, info: '' }); // 📊 Estatísticas de Cota e Performance
  const awaitingTurnByAgent = ref({});
  const forcedUnlock = ref(false); // 🔓 Trava de segurança: impede re-lock após watchdog/cancel
  const isPlanMode = ref(false); // 🔒 Modo de segurança: leitura apenas
  const showPlanOverlay = ref(false); // 🖼️ Overlay dedicado para visualização de planos
  const subagents = ref(new Map()); // 🌳 Árvore de subagentes ativos {sessionId: {agentName, goal, status}}
  const workspace = ref({ path: '', name: 'Lumaestro (Padrão)' }); // 📂 Workspace ativo
  const customAgents = ref([]); // 🤖 Lista de agentes customizados descobertos (.agents/)
  const isAgentsPanelOpen = ref(false); // 🐝 Visibilidade explícita do painel de agentes
  const isCodeSearchOpen = ref(false); // 🔎 Modal de pesquisa de código (/codesearch)
  const isDiffViewerOpen = ref(false); // 📑 Modal de visualização de git diff (/diff)
  const isPermissionsModalOpen = ref(false); // 🛡️ Modal de políticas de segurança (/permissions)
  
  // 🛡️ Motor de Confirmação Modal Premium
  const confirmModal = ref({
    show: false,
    title: '',
    message: '',
    type: 'danger',
    confirmText: 'CONFIRMAR',
    cancelText: 'ABORTAR',
    onConfirm: null,
    onCancel: null
  });

  const confirm = (options) => {
    return new Promise((resolve) => {
      confirmModal.value = {
        show: true,
        title: options.title || 'Confirmação',
        message: options.message || 'Deseja prosseguir?',
        type: options.type || 'danger',
        confirmText: options.confirmText || 'CONFIRMAR',
        cancelText: options.cancelText || 'ABORTAR',
        onConfirm: () => {
          confirmModal.value.show = false;
          resolve(true);
        },
        onCancel: () => {
          confirmModal.value.show = false;
          resolve(false);
        }
      };
    });
  };

  const executionMode = ref('default'); // 'default' | 'accept-edits' | 'plan'

  const setExecutionMode = async (mode, agent) => {
    const validModes = ['default', 'accept-edits', 'plan'];
    if (!validModes.includes(mode)) mode = 'default';
    executionMode.value = mode;
    isPlanMode.value = (mode === 'plan');
    await safeCall('core', 'SetExecutionMode', agent || activeAgent.value || 'antigravity', mode);
    pushStatus(`🛡️ Modo de Execução: [${mode}]`, 'status');
  };

  const cycleExecutionMode = async (agent) => {
    const sequence = ['default', 'accept-edits', 'plan'];
    const currentIndex = sequence.indexOf(executionMode.value);
    const nextIndex = (currentIndex + 1) % sequence.length;
    await setExecutionMode(sequence[nextIndex], agent);
  };

  const togglePlanMode = async (agent) => {
    const nextMode = isPlanMode.value ? 'default' : 'plan';
    await setExecutionMode(nextMode, agent);
  };

  const pushStatus = (text, kind = 'status') => {
    const line = String(text || '').trim();
    if (!line) return;
    statusTimeline.value.push({
      id: Date.now() + Math.random(),
      text: line,
      kind,
      at: new Date().toLocaleTimeString('pt-BR', { hour12: false })
    });
    if (statusTimeline.value.length > 40) {
      statusTimeline.value = statusTimeline.value.slice(-40);
    }
  };

  const clearStatusTimeline = () => {
    statusTimeline.value = [];
  };
  
  // Estado para histórico e checkpoints (Sinfonias)
  const sessions = ref([]);
  const currentACPID = ref(null);
  const isCreatingNewSession = ref(false);
  const hasInitialRestored = ref(false);
  
  // Estado para revisões de segurança pendentes
  const pendingReview = ref(null);

  // 🛡️ Monitor de Silêncio (Watchdog) para evitar timeouts prematuros (300s para Antigravity CLI)
  let safetyTimer = null;
  const resetSafetyTimeout = () => {
    if (safetyTimer) clearTimeout(safetyTimer);
    
    safetyTimer = setTimeout(() => {
      if (isThinking.value) {
        console.warn("[Store] Silence Timeout (300s) - A Sinfonia parece travada. Destravando UI.");
        forcedUnlock.value = true;
        isThinking.value = false;
        messages.value.push({ 
          role: 'assistant', 
          text: "⚠️ A Sinfonia está demorando para responder (mais de 5 min). Verifique sua conexão ou se o motor local está processando muitas tarefas.", 
          mode: 'system' 
        });
      }
    }, 300000); // 5 minutos para permitir raciocínio profundo e execução de ferramentas
  };

  const stopSafetyTimeout = () => {
    if (safetyTimer) {
      clearTimeout(safetyTimer);
      safetyTimer = null;
    }
  };

  // 📂 Workspace Management
  const setWorkspace = async (path) => {
    try {
      await safeCall('main', 'SetWorkspace', path);
      const result = await safeCall('main', 'GetWorkspace');
      if (result) {
        workspace.value = result;
        pushStatus(`📂 Órbita alterada: ${result.name}`, 'status');
      }
      await settingsStore.loadConfig();
      if (activeAgent.value) {
        messages.value = [];
        currentACPID.value = null;
        await fetchSessions(activeAgent.value);
      }
      return result;
    } catch (err) {
      console.error('[Workspace] Erro ao definir órbita:', err);
      throw err;
    }
  };

  const selectWorkspace = async () => {
    try {
      const result = await safeCall('main', 'SelectWorkspace');
      if (result) {
        workspace.value = result;
        pushStatus(`📂 Projeto: ${result.name}`, 'status');
        await settingsStore.loadConfig();
        if (activeAgent.value) {
          messages.value = [];
          currentACPID.value = null;
          await fetchSessions(activeAgent.value);
        }
      }
    } catch (err) {
      console.error('[Workspace] Erro ao selecionar:', err);
    }
  };

  const clearWorkspace = async () => {
    try {
      const result = await safeCall('main', 'ClearWorkspace');
      if (result) {
        workspace.value = result;
        pushStatus('📂 Workspace limpo. IA operando em Sandbox segura.', 'status');
        await settingsStore.loadConfig();
        if (activeAgent.value) {
          messages.value = [];
          currentACPID.value = null;
          await fetchSessions(activeAgent.value);
        }
      }
    } catch (err) {
      console.error('[Workspace] Erro ao limpar:', err);
    }
  };

  const loadWorkspace = async () => {
    try {
      const result = await safeCall('main', 'GetWorkspace');
      if (result && (result.path !== undefined || result.name !== undefined)) {
        workspace.value = result;
      }
      await settingsStore.loadConfig();
    } catch (err) {
      console.error('[Workspace] Erro ao carregar:', err);
    }
  };

  const initListeners = () => {
    if (listenersInitialized.value) {
      console.log('[Store] Listeners já inicializados. Ignorando nova inscrição para evitar duplicidade.');
      return;
    }
    // 🚀 AUTO-START: Dispara a varredura de Sinfonias logo após inicializar os listeners
    fetchSessions(activeAgent.value || 'antigravity');

    listenersInitialized.value = true;

    // 📂 Carregar workspace salvo ao iniciar
    loadWorkspace();

    // 📂 Listener de mudança de Workspace
    EventsOn('workspace:changed', async (data) => {
      if (data) {
        workspace.value = { path: data.path || '', name: data.name || 'Nenhuma Órbita (Sandbox)' };
        await settingsStore.loadConfig();
        if (activeAgent.value) {
          console.log("[Store] 🪐 Órbita alterada para:", data.path, "- Atualizando sinfonias...");
          messages.value = [];
          currentACPID.value = null;
          await fetchSessions(activeAgent.value);
        }
      }
    });

    // 🎼 Sincronização de Sinfonias com o Backend (Auto-Start / Auto-Resume)
    EventsOn('sessions:current', async (sessionId) => {
      console.log("[Store] 🎼 Sinfonia sincronizada com o backend:", sessionId);
      if (sessionId) {
        // Se havia uma sinfonia placeholder temporária 'nova-*', substitui pelo ID real recebido
        if (currentACPID.value && String(currentACPID.value).startsWith('nova-')) {
          const tempIndex = sessions.value.findIndex(s => s.sessionId === currentACPID.value);
          if (tempIndex !== -1) {
            sessions.value[tempIndex].sessionId = sessionId;
            sessions.value[tempIndex].title = `Sinfonia Ativa (${sessionId.substring(0, 8)})`;
          }
        }
        currentACPID.value = sessionId;
        isThinking.value = false; // 🚀 Destrava a tela inicial imediatamente
        if (messages.value.length === 0 && !isCreatingNewSession.value) {
          await restoreSessionMessages(sessionId);
        }
      }
    });

    EventsOn('sessions:updated', async () => {
      if (activeAgent.value) {
        await fetchSessions(activeAgent.value);
      }
    });

    // 0. Sinal de Início do Motor (Recuperação de Sessão)
    EventsOn('agent:starting', (agent) => {
      console.log("[Store] Motor ligando para:", agent);
      activeAgent.value = agent;
      isThinking.value = true; // Ativa o modo de carregamento
      resetSafetyTimeout();
    });

    // 📊 Telemetria de Tokens e Cache
    EventsOn('agent:tokens', (data) => {
      console.log("[Store] 📊 TELEMETRIA RECEBIDA:", data);
      modelStats.value = {
        agent: data.agent,
        info: `Prompt: ${data.prompt} | Output: ${data.candidates} | 💎 Cache: ${data.cacheCurrent} (Total: ${data.cacheTotal})`
      };
    });

    // 1. Logs Estruturados da IA (ACP)
    EventsOn('agent:log', (log) => {
      console.log("[Store] 🎻 EVENTO RECEBIDO (agent:log):", log);
      resetSafetyTimeout();

      if (!log || (!log.content && !log.Content)) return;
      const content = log.content || log.Content || "";
      const rawSource = log.source || log.Source || "Antigravity";
      const source = (rawSource.toLowerCase() === 'gemini') ? 'Antigravity' : rawSource;
      const type = log.type || log.Type || "message";

      if (type === 'thought') {
        pushStatus(content, 'think');
      }
      if (source === 'ERROR' || type === 'error') {
        pushStatus(content, 'error');
      }

      // 🔍 VISIBILIDADE UI: Encaminha logs neurais e de sistema para o Terminal de Processamento
      if (['NEURAL', 'SYSTEM', 'CRAWLER', 'RAG'].includes(source.toUpperCase())) {
        let kind = 'status';
        if (source === 'NEURAL' || source === 'RAG') kind = 'memory';
        if (source === 'CRAWLER') kind = 'status';
        if (source === 'ERROR') kind = 'error';
        pushStatus(content, kind);
      }

      // TRATAMENTO DE SISTEMA
      if (source === 'SYSTEM' || source === 'ERROR' || source === 'CRAWLER') {
        if (type === 'progress') {
          // Status de progresso transitório: atualiza indicador ao vivo sem poluir o histórico com bolhas estáticas
          pushStatus(content, 'status');
          currentStatus.value = { agent: 'Maestro', tool: '', action: content };
          isThinking.value = true;
          return;
        }
        messages.value = [...messages.value, { role: 'assistant', text: content, mode: 'system', agent: source }];
        return;
      }

      // TRATAMENTO DE MENSAGENS E PENSAMENTOS
      let lastMsg = messages.value[messages.value.length - 1];
      const role = type === 'user' ? 'user' : 'assistant';
      
      // Se a última mensagem não for do mesmo autor (role/source) ou for de sistema, cria uma nova
      if (!lastMsg || lastMsg.role !== role || lastMsg.mode === 'system' || (role === 'assistant' && lastMsg.agent !== source)) {
          lastMsg = { 
            role: role, 
            text: '', 
            thought: '',
            agent: source,
            isPlanning: role === 'assistant',
            isStreaming: true
          };
          messages.value = [...messages.value, lastMsg];
      }

      // Atualiza a última mensagem (reatividade via índice para garantir o Vue)
      const idx = messages.value.length - 1;
      if (type === 'thought') {
          messages.value[idx].thought += content;
      } else {
          messages.value[idx].isPlanning = false;
          messages.value[idx].text += content;
          
          // 🎬 Zoom Cinematográfico: Extração reativa de links
          const matches = [...messages.value[idx].text.matchAll(/\[\[(.*?)\]\]/g)];
          for (const match of matches) {
              const nodeName = match[1].trim();
              if (nodeName && nodeName !== "") {
                  if (!messages.value[idx].focusedNodes) {
                      messages.value[idx].focusedNodes = new Set();
                  }
                  if (!messages.value[idx].focusedNodes.has(nodeName)) {
                      messages.value[idx].focusedNodes.add(nodeName);
                      console.log(`[Store] 🎬 Zoom Cinematográfico Detectado: ${nodeName}`);
                      // Dispara evento globalmente no frontend para o BridgeDriver pegar
                      window.dispatchEvent(new CustomEvent('cinematic:zoom', { detail: nodeName }));
                  }
              }
          }
      }
      
      // Forçar atualização do array (Sincronização definitiva)
      messages.value[idx].isStreaming = true;
      messages.value = [...messages.value];
      isThinking.value = false;
    });

    // 2. Pedidos de Revisão Manual (Security Hands)
    EventsOn('agent:review_request', (review) => {
      console.log("[Store] Pedido de Revisão:", review);
      pendingReview.value = review;
    });

    EventsOn('terminal:started', (info) => {
      const agent = info?.agent;
      if (agent && !runningSessions.value.includes(agent)) runningSessions.value.push(agent);
      activeAgent.value = agent;
      isTerminalMode.value = true;
      isThinking.value = false; // Destrava a tela inicial de carregamento
    });

    EventsOn('terminal:closed', (agent) => {
      runningSessions.value = runningSessions.value.filter(a => a !== agent);
      if (activeAgent.value === agent) {
        activeAgent.value = runningSessions.value[0] || null;
        if (!activeAgent.value) isTerminalMode.value = false;
      }
      isThinking.value = false;
    });

    // 3. Detecção de Erros de Autenticação (Login)
    EventsOn('agent:login_required', async (agent) => {
      console.warn("[Store] Login necessário para:", agent);
      isThinking.value = false;
      messages.value.push({ 
        role: 'assistant', 
        text: `⚠️ O ${agent} precisa de autenticação. Abrindo terminal de login...`, 
        mode: 'system' 
      });
      // Dispara o SetupTool (terminal externo) para o agente
      await safeCall('main', 'SetupTool', agent);
    });

    // 3.5 Identidade e Status (Maestro UI Evolution)
    EventsOn('agent:profile', (p) => {
      console.log("[Store] 🎭 Identidade assumida:", p);
      activeProfile.value = p;
    });

    // 📡 Status de Atividade: Mostra o que a IA está fazendo AGORA (ex: lendo arquivo)
    EventsOn('agent:status', (s) => {
      console.log("[Store] 🛠️ Status da IA:", s);
      const actionRaw = s.action || s.Action || "";
      const fileRaw = s.file || s.File || "";
      currentStatus.value = {
        agent: s.agent || s.Agent || "",
        tool: s.tool || s.Tool || "",
        action: actionRaw,
        file: fileRaw,
        state: s.state || s.State || ""
      };
      currentStatusKind.value = s.kind || 'status';
      const actionStr = String(actionRaw || 'Atualizando estado do agente...');
      let kind = s.kind || 'status';
      const lowered = actionStr.toLowerCase();
      if (kind === 'status') {
        if (lowered.includes('ferramenta') || lowered.includes('tool')) kind = 'tool';
        if (lowered.includes('comando') || lowered.includes('cmd ') || lowered.includes('powershell') || lowered.includes('bash')) kind = 'command';
        if (lowered.includes('erro') || lowered.includes('falha')) kind = 'error';
        if (lowered.includes('memória') || lowered.includes('memoria') || lowered.includes('grafo') || lowered.includes('contexto')) kind = 'memory';
      }
      pushStatus(actionStr, kind);
      resetSafetyTimeout();

      // 🔓 Se o motor avisar que está PRONTO ou ONLINE, DESTRAVA a UI imediatamente!
      if (lowered.includes('pronto') || lowered.includes('online') || lowered.includes('aguardando') || kind === 'ready') {
        isThinking.value = false;
        forcedUnlock.value = false;
      } else if (kind !== 'memory' && !forcedUnlock.value) {
        isThinking.value = true;
      }

      // 🔄 Sincroniza status para subagentes também
      if (subagents.value.has(s.agentId || s.sessionId)) {
        const sub = subagents.value.get(s.agentId || s.sessionId);
        sub.status = actionRaw;
        sub.kind = kind;
      }
    });

    // 📡 Listener de Estatísticas (Uso de Tokens/Latência)
    EventsOn('agent:stats', (s) => {
      modelStats.value = {
        agent: s.agent || s.Agent || "",
        info: s.info || s.Info || ""
      };
    });

    // 🧶 WEAVER: Sinalização de Tecelagem de Conhecimento
    EventsOn('weaver:started', () => {
      console.log("[Store] 🧶 WEAVER ativada: Tecendo conexões neurais...");
      isWeaving.value = true;
    });

    EventsOn('weaver:finished', () => {
      console.log("[Store] 🧶 WEAVER finalizada: Sinapses consolidadas.");
      isWeaving.value = false;
    });

    // 🌳 HIERARQUIA: Monitoramento de Subagentes
    EventsOn('agent:subagent_spawned', (data) => {
      console.log("[Store] 🚀 Subagente detectado no enxame:", data);
      subagents.value.set(data.childId, {
        parentId: data.parentId,
        agentName: data.agentName,
        goal: data.goal,
        status: 'Iniciando...',
        kind: 'status'
      });
      pushStatus(`🚀 Subagente ${data.agentName} iniciado para: ${data.goal}`, 'status');
    });

    EventsOn('agent:subagent_stopped', (data) => {
      console.log("[Store] 🛑 Subagente encerrado:", data.sessionId);
      subagents.value.delete(data.sessionId);
    });

    // 🚀 Sincronização de Sinfonias (Checkpoints): Quando o turno termina, atualizamos a lista lateral e consolidamos a memória
    EventsOn('agent:turn_complete', async (agent) => {
      const key = String(agent || 'unknown').toLowerCase();

      // Só processa 1 encerramento por mensagem enviada para evitar loops de pós-processamento.
      if (!awaitingTurnByAgent.value[key]) {
        return;
      }

      const now = Date.now();
      const last = lastTurnCompleteByAgent.value[key] || 0;
      if (now-last < 800) {
        return;
      }
      lastTurnCompleteByAgent.value[key] = now;
      awaitingTurnByAgent.value[key] = false;

      console.log(`[Store] Turno concluído para ${agent}. Atualizando Sinfonias e Consolidando Memória...`);
      stopSafetyTimeout(); // 🛑 Turno finalizado, para o cronômetro
      
      // 🔓 DESTRAVAMENTO ASSÍNCRONO: Libera a interface IMEDIATAMENTE após a resposta da IA.
      // O pós-processamento de memória (RAG) agora roda em background para não travar a UI.
      isThinking.value = false;
      currentStatus.value = null; // 🧹 Limpa o status ao terminar
      currentStatusKind.value = 'status';

      if (messages.value.length > 0) {
         const lastMsg = messages.value[messages.value.length - 1];
         if (lastMsg.role === 'assistant') {
            lastMsg.isStreaming = false;
         }
         messages.value = [...messages.value];
      }
      
      // Auto-seleciona a nova sessão criada se viemos de um 'Novo Chat'
      fetchSessions(agent).then(() => {
        if (!currentACPID.value && sessions.value.length > 0) {
           currentACPID.value = sessions.value[0].sessionId;
        }
      });

      // Consolidação de Conhecimento RAG (Memória) - Roda em background sem AWAIT
      const sessionID = currentACPID.value || (sessions.value.length > 0 ? sessions.value[0].sessionId : 'default');
      const lastMessages = messages.value.slice(-2).map(m => `${m.role}: ${m.text}`).join("\n");
      
      if (lastMessages) {
        console.log("[Store] Disparando ConsolidateChatKnowledge em background para sessão:", sessionID);
        // Sem 'await' para não bloquear a UI enquanto o motor de IA luta com as cotas da API
        safeCall('main', 'ConsolidateChatKnowledge', sessionID, lastMessages).catch(err => {
          console.error("[Store] Erro na consolidação de memória de background:", err);
        });
      }
    });

    // 🏷️ Atualização em tempo real de título de Sinfonia (Manual ou IA)
    EventsOn('session:renamed', (data) => {
      if (!data || !data.sessionId) return;
      console.log("[Store] 🏷️ Sessão renomeada:", data);
      const target = sessions.value.find(s => s.sessionId === data.sessionId);
      if (target) {
        target.title = data.title;
      }
    });

    // 🛡️ Sincronização de Modo de Execução (default | accept-edits | plan)
    EventsOn('mode:changed', (data) => {
      if (!data || !data.mode) return;
      console.log("[Store] 🛡️ Modo de execução atualizado:", data.mode);
      executionMode.value = data.mode;
      isPlanMode.value = (data.mode === 'plan');
    });

    // 4. Watcher de Resiliência: Mantém a UI síncrona com a realidade do Backend
    watch(runningSessions, (sessions) => {
      console.log("[Store] Resiliência: Sessões Ativas:", sessions);
      if (sessions.length === 0) {
        console.warn("[Store] Nenhuma sessão ativa. Limpando estados fantasmas.");
        activeAgent.value = null;
        isThinking.value = false;
        isTerminalMode.value = false;
      } else if (activeAgent.value && !sessions.includes(activeAgent.value)) {
        // Se o agente ativo atual morreu, foca no próximo disponível
        activeAgent.value = sessions[0];
      }
    }, { deep: true });
  };

  const ask = async (agent, prompt) => {
    // 🚀 RESET DE NAVEGAÇÃO: Limpa o foco atual para evitar zooms residuais de pesquisas anteriores
    window.dispatchEvent(new CustomEvent('cinematic:zoom', { detail: null }));
    
    messages.value.push({ role: 'user', text: prompt });
    isThinking.value = true;
    activeAgent.value = agent;
    const key = String(agent || 'unknown').toLowerCase();
    awaitingTurnByAgent.value[key] = true;

    try {
      await safeCall('main', 'AskAgent', agent, prompt);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro: ${err}`, mode: 'system' });
      isThinking.value = false;
      awaitingTurnByAgent.value[key] = false;
    }
  };

  const startSession = async (agent) => {
    // 🛡️ Trava de Segurança: Não inicia se já estiver rodando
    if (runningSessions.value.includes(agent)) {
      console.log(`[Store] Agente ${agent} já está ativo. Ignorando novo Start.`);
      isThinking.value = false;
      return;
    }

    console.log(`[Store] Iniciando Sessão ACP para: ${agent}`);
    isThinking.value = true;
    isTerminalMode.value = true;
    activeAgent.value = agent;
    
    try {
      await safeCall('main', 'StartAgentSession', agent);
      
      // 🚀 Após iniciar o processo, tentamos buscar o histórico
      await fetchSessions(agent);
      
      // Se houver histórico e não estivermos carregando um específico,
      // sugerimos o último checkpoint encontrado.
      if (sessions.value.length > 0 && !currentACPID.value) {
          const last = sessions.value[0]; 
          currentACPID.value = last.sessionId;
      }
      
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Falha: ${err}`, mode: 'system' });
    } finally {
      isThinking.value = false;
    }
  };

  const restoreSessionMessages = async (sessionId) => {
    if (!sessionId) return;
    try {
      console.log(`[Store] 📜 Restaurando mensagens da Sinfonia: ${sessionId}`);
      const rawMsgs = await safeCall('core', 'GetSessionMessages', sessionId);
      if (rawMsgs && Array.isArray(rawMsgs) && rawMsgs.length > 0) {
        messages.value = rawMsgs.map(m => ({
          role: m.role || 'assistant',
          text: m.text || '',
          thought: m.thought || '',
          agent: m.agent || 'Antigravity',
          isPlanning: m.isPlanning ?? (m.role === 'assistant'),
          mode: m.mode || ''
        }));
        console.log(`[Store] ✅ ${messages.value.length} mensagens restauradas para a Sinfonia ${sessionId}`);
      } else {
        console.log(`[Store] ℹ️ Nenhuma mensagem anterior encontrada para a Sinfonia ${sessionId}`);
      }
    } catch (err) {
      console.warn(`[Store] ⚠️ Falha ao restaurar mensagens da sessão ${sessionId}:`, err);
    }
  };

  const fetchSessions = async (agent) => {
    if (!agent) return;
    try {
      const list = await safeCall('core', 'ListAgentSessions', agent);
      if (list) {
          // Se estivermos no meio da criação de nova sinfonia temporária, preserva-a no topo
          const tempSession = isCreatingNewSession.value ? sessions.value.find(s => String(s.sessionId).startsWith('nova-')) : null;

          // Ordenar por data (mais recente primeiro)
          let sorted = list.sort((a, b) => new Date(b.updatedAt) - new Date(a.updatedAt));
          if (tempSession) {
            sorted = [tempSession, ...sorted.filter(s => s.sessionId !== tempSession.sessionId)];
          }
          sessions.value = sorted;

          // 🚀 Auto-restauração APENAS na montagem inicial se o chat estiver vazio E não estivermos criando nova sessão
          if (messages.value.length === 0 && !isCreatingNewSession.value && !hasInitialRestored.value) {
            const targetId = currentACPID.value || (sessions.value.length > 0 ? sessions.value[0].sessionId : null);
            if (targetId) {
              console.log("[Store] 🚀 Auto-restaurando mensagens da sinfonia mais recente:", targetId);
              currentACPID.value = targetId;
              await restoreSessionMessages(targetId);
            }
            hasInitialRestored.value = true;
          }
      }
    } catch (err) {
      console.error("[Store] Erro ao buscar sessões:", err);
    }
  };

  const isLoadingSession = ref(false); // 🛡️ Trava anti-duplicação de cliques

  const loadSession = async (agent, acpID) => {
    // 🛡️ ANTI-DUPLICAÇÃO: Ignora apenas se já estamos nesta sinfonia E as mensagens já estão em tela
    if (currentACPID.value === acpID && messages.value.length > 0) {
      console.log(`[Store] Sinfonia ${acpID} já está ativa com mensagens. Ignorando.`);
      isThinking.value = false;
      return;
    }
    if (isLoadingSession.value) {
      console.log(`[Store] Já carregando uma Sinfonia. Ignorando clique duplicado.`);
      return;
    }

    console.log(`[Store] Carregando Sinfonia: ${acpID}`);
    isLoadingSession.value = true;
    isThinking.value = true;
    currentACPID.value = acpID;
    messages.value = []; // Limpa o chat para receber o novo contexto restaurado
    
    try {
      await safeCall('core', 'LoadAgentSession', agent, acpID);
      await restoreSessionMessages(acpID);
      await fetchSessions(agent); // Atualiza a lista lateral
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro ao carregar: ${err}`, mode: 'system' });
    } finally {
      isLoadingSession.value = false;
      isThinking.value = false;
    }
  };

  const newSession = async (agent) => {
    console.log(`[Store] Iniciando nova Sinfonia personalizada...`);
    const targetAgent = agent || activeAgent.value || 'antigravity';
    isCreatingNewSession.value = true;
    hasInitialRestored.value = true;
    isThinking.value = true;
    currentACPID.value = null;
    messages.value = [];
    
    // Adiciona uma sinfonia placeholder imediata no topo para feedback instantâneo de 0ms
    const tempId = 'nova-' + Date.now();
    currentACPID.value = tempId;
    sessions.value.unshift({
      sessionId: tempId,
      title: 'Nova Sinfonia',
      updatedAt: new Date().toISOString(),
      workspace: workspace.value?.name || 'Workspace'
    });

    try {
      await safeCall('core', 'NewAgentSession', targetAgent);
      await fetchSessions(targetAgent);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro ao criar: ${err}`, mode: 'system' });
    } finally {
      isThinking.value = false;
      setTimeout(() => {
        isCreatingNewSession.value = false;
      }, 1500);
    }
  };

  const renameSession = async (sessionId, newTitle) => {
    try {
      await safeCall('core', 'RenameSession', sessionId, newTitle);
      const target = sessions.value.find(s => s.sessionId === sessionId);
      if (target) {
        target.title = newTitle;
      }
    } catch (err) {
      console.error("[Store] Erro ao renomear sessão:", err);
      throw err;
    }
  };

  const autoNameSession = async (sessionId) => {
    try {
      const title = await safeCall('core', 'AutoNameSession', sessionId);
      if (title) {
        const target = sessions.value.find(s => s.sessionId === sessionId);
        if (target) {
          target.title = title;
        }
        return title;
      }
    } catch (err) {
      console.error("[Store] Erro ao auto-nomear sessão com IA:", err);
      throw err;
    }
  };

  const sendInput = async (agent, text, images = []) => {
    console.log(`[Store] Enviando Input ACP (${agent}): ${text} com ${images.length} imagens`);
    
    // Registra a mensagem no histórico local incluindo as imagens para o ChatLog renderizar
    messages.value.push({ 
      role: 'user', 
      text: text,
      images: images // Formato [{data, type}]
    });
    
    forcedUnlock.value = false; // 🔓 Nova mensagem: reseta a trava de segurança
    isThinking.value = true; // Feedback visual imediato
    resetSafetyTimeout(); // Inicia o contador de silêncio
    const key = String(agent || 'unknown').toLowerCase();
    awaitingTurnByAgent.value[key] = true;

    try {
      // 🛠️ SINCRONIZAÇÃO CRÍTICA: Agora enviamos 3 argumentos conforme o novo contrato Go
      const resp = await safeCall('main', 'SendAgentInput', agent, text, images);
      return resp;
    } catch (err) {
      console.error('[Store] Erro ao enviar input:', err);
      isThinking.value = false;
      stopSafetyTimeout();
      awaitingTurnByAgent.value[key] = false;
    }
  };

  const submitReview = async (approved) => {
    if (!pendingReview.value) return;
    const id = pendingReview.value.id;
    pendingReview.value = null;
    try {
      await safeCall('main', 'SubmitReview', id, approved);
    } catch (err) {
      console.error("Falha ao enviar review:", err);
    }
  };

  const switchAgent = (agent) => {
    activeAgent.value = agent;
  };

  const stopSession = async () => {
    if (!activeAgent.value) return;
    try {
      await safeCall('main', 'StopAgentSession', activeAgent.value);
    } catch (err) {
      console.error("Erro ao fechar sessão:", err);
    }
  };

  // 🛑 FORÇA o desbloqueio da UI (botão PARAR)
  const forceUnlock = () => {
    console.warn('[Store] 🛑 FORCE UNLOCK acionado pelo usuário.');
    forcedUnlock.value = true;
    isThinking.value = false;
    stopSafetyTimeout();
    currentStatus.value = null;
    currentStatusKind.value = 'status';
    // Encerra streaming de qualquer mensagem ativa
    if (messages.value.length > 0) {
      const lastMsg = messages.value[messages.value.length - 1];
      if (lastMsg.role === 'assistant' && lastMsg.isStreaming) {
        lastMsg.isStreaming = false;
        lastMsg.text += '\n\n🛑 *Interrompido pelo usuário.*';
        messages.value = [...messages.value];
      }
    }
    pushStatus('🛑 Processamento interrompido pelo usuário', 'error');
  };

  const isSidebarOpen = ref(false);
  const toggleSidebar = async () => {
    isSidebarOpen.value = !isSidebarOpen.value;
    console.log(`[Store] Histórico ${isSidebarOpen.value ? 'Aberto' : 'Fechado'}`);
    
    // Auto-fetch ao abrir
    if (isSidebarOpen.value && activeAgent.value) {
      await fetchSessions(activeAgent.value);
    }
  };

  // ⚡ MODEL STEERING: Envia dicas de direcionamento enquanto o agente está processando
  const sendSteeringHint = async (agent, text) => {
    if (!text.trim()) return;

    console.log(`[Store] ⚡ Enviando Steering Hint para ${agent}: ${text}`);
    
    // Adiciona feedback visual imediato no chat como uma mensagem de sistema/direcionamento
    messages.value.push({
      role: 'user',
      text: text,
      isSteering: true // Flag para estilização futura se desejado
    });

    pushStatus(`⚡ Direcionamento enviado: "${text.substring(0, 20)}..."`, 'status');

    try {
      await safeCall('main', 'SendSteeringHint', agent, text);
    } catch (err) {
      console.error('[Store] Falha ao enviar steering hint:', err);
      pushStatus('❌ Falha ao enviar direcionamento', 'error');
    }
  };

  // 🤖 Descoberta e Gerenciamento de Agentes Customizados (/agents)
  const fetchCustomAgents = async () => {
    try {
      const list = await safeCall('core', 'GetCustomAgents');
      if (Array.isArray(list)) {
        customAgents.value = list;
      }
      return customAgents.value;
    } catch (err) {
      console.error('[Store] Falha ao buscar custom agents:', err);
      return [];
    }
  };

  const killSubagent = async (sessionId) => {
    try {
      console.log(`[Store] Encerrando subagente ${sessionId}...`);
      await safeCall('core', 'KillSubagent', sessionId);
      subagents.value.delete(sessionId);
      pushStatus(`Subagente encerrado: ${sessionId}`, 'status');
      return true;
    } catch (err) {
      console.error(`[Store] Erro ao matar subagente ${sessionId}:`, err);
      return false;
    }
  };

  // 🔎 Pesquisa de Código no Workspace (/codesearch)
  const runCodeSearch = async (query, pathFilter = '', isLiteral = false) => {
    try {
      const results = await safeCall('core', 'RunCodeSearch', query, pathFilter, isLiteral);
      return results || [];
    } catch (err) {
      console.error('[Store] Erro na busca de código:', err);
      return [];
    }
  };

  // 📑 Diff do Workspace (/diff)
  const getWorkspaceDiff = async () => {
    try {
      const diffResult = await safeCall('core', 'GetWorkspaceDiff');
      return diffResult || { status: '', files: [], diff: '' };
    } catch (err) {
      console.error('[Store] Erro ao obter diff do workspace:', err);
      return { status: '', files: [], diff: '' };
    }
  };

  // 🛡️ Permissões e Sandbox (/permissions)
  const getSecurityPermissions = async () => {
    try {
      const perms = await safeCall('core', 'GetSecurityPermissions');
      return perms || { allowed_tools: [], denied_commands: [], auto_approve: false, sandbox_mode: 'standard' };
    } catch (err) {
      console.error('[Store] Erro ao obter permissões:', err);
      return null;
    }
  };

  const toggleAgentsPanel = (state) => {
    isAgentsPanelOpen.value = (typeof state === 'boolean') ? state : !isAgentsPanelOpen.value;
  };

  const toggleCodeSearch = (state) => {
    isCodeSearchOpen.value = (typeof state === 'boolean') ? state : !isCodeSearchOpen.value;
  };

  const toggleDiffViewer = (state) => {
    isDiffViewerOpen.value = (typeof state === 'boolean') ? state : !isDiffViewerOpen.value;
  };

  const currentView = ref('orchestrator');

  const openSettings = (tab = 'geral') => {
    currentView.value = 'settings';
    try {
      const settingsStore = useSettingsStore();
      settingsStore.activeTab = tab;
    } catch (e) {
      console.warn('[Store] Erro ao trocar aba de configurações:', e);
    }
  };

  const togglePermissionsModal = () => {
    openSettings('seguranca');
  };

  const toggleArtifactModal = (state) => {
    showPlanOverlay.value = (typeof state === 'boolean') ? state : !showPlanOverlay.value;
  };

  const forkSession = async () => {
    const currentId = currentACPID.value;
    const agent = activeAgent.value || 'antigravity';
    pushStatus('🌿 Ramificando sessão ativa (/fork)...', 'status');
    try {
      const bridge = window.go?.core?.App || window.go?.main?.App;
      if (bridge && typeof bridge.ForkSession === 'function') {
        const newId = await bridge.ForkSession(agent, currentId || '');
        if (newId) {
          await loadSession(agent, newId);
          pushStatus(`🌿 Sessão ramificada: ${newId.substring(0, 8)}...`, 'success');
          return;
        }
      }
      await newSession(agent);
      pushStatus('🌿 Nova ramificação de sessão criada com sucesso.', 'success');
    } catch (err) {
      console.error('[Store] Falha ao ramificar sessão:', err);
      pushStatus('❌ Falha ao ramificar sessão', 'error');
    }
  };

  return {
    messages, isThinking, isTerminalMode, isWeaving, activeAgent, runningSessions, pendingReview, modelStats,
    sessions, currentACPID, isSidebarOpen, currentStatus, isNavigating, currentStatusKind, statusTimeline, statusFilter,
    isPlanMode, togglePlanMode, executionMode, setExecutionMode, cycleExecutionMode, subagents, showPlanOverlay, workspace,
    customAgents, isAgentsPanelOpen, isCodeSearchOpen, isDiffViewerOpen, isPermissionsModalOpen,
    initListeners, ask, startSession, sendInput, submitReview, switchAgent, stopSession, forceUnlock,
    fetchSessions, loadSession, restoreSessionMessages, newSession, renameSession, autoNameSession, toggleSidebar, clearStatusTimeline, sendSteeringHint,
    selectWorkspace, clearWorkspace, loadWorkspace, setWorkspace,
    fetchCustomAgents, killSubagent, runCodeSearch, getWorkspaceDiff, getSecurityPermissions,
    toggleAgentsPanel, toggleCodeSearch, toggleDiffViewer, togglePermissionsModal, toggleArtifactModal, forkSession,
    currentView, openSettings,
    confirm, confirmModal
  };
});
