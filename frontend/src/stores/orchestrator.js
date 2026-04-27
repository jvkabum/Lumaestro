import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';

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
  const messages = ref([]);
  const isThinking = ref(false);
  const isNavigating = ref(false); // 🔍 Inteligência de Navegação em Tempo Real
  const isTerminalMode = ref(false);
  const isWeaving = ref(false); // 🧶 Teccelagem de Conhecimento em Background
  const activeAgent = ref(null);
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

  const togglePlanMode = async (agent) => {
    isPlanMode.value = !isPlanMode.value;
    await safeCall('core', 'SetPlanMode', agent || activeAgent.value || 'gemini', isPlanMode.value);
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
  
  // Estado para revisões de segurança pendentes
  const pendingReview = ref(null);

  // 🛡️ Monitor de Silêncio (Watchdog) para evitar timeouts prematuros
  let safetyTimer = null;
  const resetSafetyTimeout = () => {
    if (safetyTimer) clearTimeout(safetyTimer);
    
    safetyTimer = setTimeout(() => {
      if (isThinking.value) {
        console.warn("[Store] Silence Timeout (90s) - A Sinfonia parece travada. Destravando UI.");
        forcedUnlock.value = true; // 🔓 Bloqueia qualquer agent:status de re-ligar o spinner
        isThinking.value = false;
        messages.value.push({ 
          role: 'assistant', 
          text: "⚠️ A Sinfonia está demorando para responder (mais de 90s). Verifique sua conexão ou se o motor local está processando muitas tarefas.", 
          mode: 'system' 
        });
      }
    }, 90000);
  };

  const stopSafetyTimeout = () => {
    if (safetyTimer) {
      clearTimeout(safetyTimer);
      safetyTimer = null;
    }
  };

  // 📂 Workspace Management
  const selectWorkspace = async () => {
    try {
      const result = await safeCall('main', 'SelectWorkspace');
      if (result) {
        workspace.value = result;
        pushStatus(`📂 Projeto: ${result.name}`, 'status');
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
        pushStatus('📂 Workspace limpo. IA no modo Lumaestro.', 'status');
      }
    } catch (err) {
      console.error('[Workspace] Erro ao limpar:', err);
    }
  };

  const loadWorkspace = async () => {
    try {
      const result = await safeCall('main', 'GetWorkspace');
      if (result) workspace.value = result;
    } catch (err) {
      console.error('[Workspace] Erro ao carregar:', err);
    }
  };

  const initListeners = () => {
    if (listenersInitialized.value) {
      console.log('[Store] Listeners já inicializados. Ignorando nova inscrição para evitar duplicidade.');
      return;
    }
    listenersInitialized.value = true;

    // 📂 Carregar workspace salvo ao iniciar
    loadWorkspace();

    // 📂 Listener de mudança de Workspace
    EventsOn('workspace:changed', (data) => {
      if (data) {
        workspace.value = { path: data.path || '', name: data.name || 'Lumaestro (Padrão)' };
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
      const source = log.source || log.Source || "Gemini";
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
      currentStatus.value = {
        agent: s.agent || s.Agent || "",
        tool: s.tool || s.Tool || "",
        action: actionRaw
      };
      currentStatusKind.value = s.kind || 'status';
      const actionStr = String(actionRaw || 'Atualizando estado do agente...');
      let kind = s.kind || 'status';
      if (kind === 'status') {
        const lowered = actionStr.toLowerCase();
        if (lowered.includes('ferramenta') || lowered.includes('tool')) kind = 'tool';
        if (lowered.includes('comando') || lowered.includes('cmd ') || lowered.includes('powershell') || lowered.includes('bash')) kind = 'command';
        if (lowered.includes('erro') || lowered.includes('falha')) kind = 'error';
        if (lowered.includes('memória') || lowered.includes('memoria') || lowered.includes('grafo') || lowered.includes('contexto')) kind = 'memory';
      }
      pushStatus(actionStr, kind);
      // 🔓 Se o watchdog ou o usuário já desbloqueou a UI, NÃO re-ligar o spinner.
      // Status de memória também não deve religar.
      if (kind !== 'memory' && !forcedUnlock.value) {
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
      isThinking.value = false;
    }
  };

  const fetchSessions = async (agent) => {
    if (!agent) return;
    try {
      const list = await safeCall('main', 'ListAgentSessions', agent);
      if (list) {
          // Ordenar por data (mais recente primeiro)
          sessions.value = list.sort((a, b) => new Date(b.updatedAt) - new Date(a.updatedAt));
      }
    } catch (err) {
      console.error("[Store] Erro ao buscar sessões:", err);
    }
  };

  const loadSession = async (agent, acpID) => {
    console.log(`[Store] Carregando Sinfonia: ${acpID}`);
    isThinking.value = true;
    currentACPID.value = acpID;
    messages.value = []; // Limpa o chat para receber o novo contexto restaurado
    
    try {
      await safeCall('main', 'LoadAgentSession', agent, acpID);
      await fetchSessions(agent); // Atualiza a lista lateral
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro ao carregar: ${err}`, mode: 'system' });
      isThinking.value = false;
    }
  };

  const newSession = async (agent) => {
    console.log(`[Store] Iniciando nova Sinfonia personalizada...`);
    isThinking.value = true;
    currentACPID.value = null;
    messages.value = [];
    
    try {
      await safeCall('main', 'NewAgentSession', agent);
      await fetchSessions(agent);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro ao criar: ${err}`, mode: 'system' });
      isThinking.value = false;
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

  return {
    messages, isThinking, isTerminalMode, isWeaving, activeAgent, runningSessions, pendingReview, modelStats,
    sessions, currentACPID, isSidebarOpen, currentStatus, isNavigating, currentStatusKind, statusTimeline, statusFilter,
    isPlanMode, togglePlanMode, subagents, showPlanOverlay, workspace,
    initListeners, ask, startSession, sendInput, submitReview, switchAgent, stopSession, forceUnlock,
    fetchSessions, loadSession, newSession, toggleSidebar, clearStatusTimeline, sendSteeringHint,
    selectWorkspace, clearWorkspace, loadWorkspace
  };
});
