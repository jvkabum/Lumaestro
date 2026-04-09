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
  const awaitingTurnByAgent = ref({});

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
        console.warn("[Store] Silence Timeout (60s) - A Sinfonia parece travada.");
        isThinking.value = false;
        messages.value.push({ 
          role: 'assistant', 
          text: "⚠️ A Sinfonia está demorando para responder (mais de 60s). Verifique sua conexão ou se o motor local está processando muitas tarefas.", 
          mode: 'system' 
        });
      }
    }, 60000);
  };

  const stopSafetyTimeout = () => {
    if (safetyTimer) {
      clearTimeout(safetyTimer);
      safetyTimer = null;
    }
  };

  const initListeners = () => {
    if (listenersInitialized.value) {
      console.log('[Store] Listeners já inicializados. Ignorando nova inscrição para evitar duplicidade.');
      return;
    }
    listenersInitialized.value = true;

    // 0. Sinal de Início do Motor (Recuperação de Sessão)
    EventsOn('agent:starting', (agent) => {
      console.log("[Store] Motor ligando para:", agent);
      activeAgent.value = agent;
      isThinking.value = true; // Ativa o modo de carregamento
      resetSafetyTimeout();
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
      if (source === 'ERROR') {
        pushStatus(content, 'error');
      }

      // TRATAMENTO DE SISTEMA
      if (source === 'SYSTEM' || source === 'ERROR' || source === 'CRAWLER') {
        messages.value = [...messages.value, { role: 'assistant', text: content, mode: 'system', agent: source }];
        return;
      }

      // TRATAMENTO DE MENSAGENS E PENSAMENTOS DA IA
      let lastMsg = messages.value[messages.value.length - 1];
      
      // Se a última mensagem não for do assistente ou for de sistema, cria uma nova
      if (!lastMsg || lastMsg.role !== 'assistant' || lastMsg.mode === 'system' || lastMsg.agent !== source) {
          lastMsg = { 
            role: 'assistant', 
            text: '', 
            thought: '',
            agent: source,
            isPlanning: true,
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
      // Status de memória é pós-processamento e não deve religar o spinner principal da resposta.
      if (kind !== 'memory') {
        isThinking.value = true;
      }
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
      isThinking.value = false;
      if (currentStatus.value && currentStatus.value.action) {
        pushStatus(`Concluído: ${currentStatus.value.action}`, 'status');
      } else if (typeof currentStatus.value === 'string' && currentStatus.value) {
        pushStatus(`Concluído: ${currentStatus.value}`, 'status');
      }
      currentStatus.value = null; // 🧹 Limpa o status ao terminar
      currentStatusKind.value = 'status';

      // Encerra a digitação da mensagem viva
      if (messages.value.length > 0) {
         const lastMsg = messages.value[messages.value.length - 1];
         if (lastMsg.role === 'assistant') {
            lastMsg.isStreaming = false;
         }
         messages.value = [...messages.value];
      }
      fetchSessions(agent);

      // Consolidação de Conhecimento RAG em tempo real
      const sessionID = currentACPID.value || 'default';
      const lastMessages = messages.value.slice(-2).map(m => `${m.role}: ${m.text}`).join("\n");
      
      if (lastMessages) {
        console.log("[Store] Disparando ConsolidateChatKnowledge para sessão:", sessionID);
        try {
          await safeCall('main', 'ConsolidateChatKnowledge', sessionID, lastMessages);
        } finally {
          // Garante encerramento visual do ciclo após pós-processamento de memória.
          isThinking.value = false;
          currentStatus.value = "";
          currentStatusKind.value = 'status';
        }
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

  const isSidebarOpen = ref(false);
  const toggleSidebar = async () => {
    isSidebarOpen.value = !isSidebarOpen.value;
    console.log(`[Store] Histórico ${isSidebarOpen.value ? 'Aberto' : 'Fechado'}`);
    
    // Auto-fetch ao abrir
    if (isSidebarOpen.value && activeAgent.value) {
      await fetchSessions(activeAgent.value);
    }
  };

  return {
    messages, isThinking, isTerminalMode, isWeaving, activeAgent, runningSessions, pendingReview,
    sessions, currentACPID, isSidebarOpen, currentStatus, isNavigating, currentStatusKind, statusTimeline, statusFilter,
    initListeners, ask, startSession, sendInput, submitReview, switchAgent, stopSession,
    fetchSessions, loadSession, newSession, toggleSidebar, clearStatusTimeline
  };
});
