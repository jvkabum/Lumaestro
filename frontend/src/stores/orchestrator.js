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
  const activeAgent = ref(null);
  const runningSessions = ref([]);
  
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
        console.warn("[Store] Silence Timeout (25s) - A Sinfonia parece travada.");
        isThinking.value = false;
        messages.value.push({ 
          role: 'assistant', 
          text: "⚠️ A Sinfonia está demorando para responder. O processo ainda pode estar ativo no background.", 
          mode: 'system' 
        });
      }
    }, 25000);
  };

  const stopSafetyTimeout = () => {
    if (safetyTimer) {
      clearTimeout(safetyTimer);
      safetyTimer = null;
    }
  };

  const initListeners = () => {
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
            isPlanning: true
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

    // 🚀 Sincronização de Sinfonias (Checkpoints): Quando o turno termina, atualizamos a lista lateral e consolidamos a memória
    window.runtime.EventsOn("agent:turn_complete", async (agent) => {
      console.log(`[Store] Turno concluído para ${agent}. Atualizando Sinfonias e Consolidando Memória...`);
      stopSafetyTimeout(); // 🛑 Turno finalizado, para o cronômetro
      isThinking.value = false;
      fetchSessions(agent);

      // Consolidação de Conhecimento RAG em tempo real
      const sessionID = currentACPID.value || 'default';
      const lastMessages = messages.value.slice(-2).map(m => `${m.role}: ${m.text}`).join("\n");
      
      if (lastMessages) {
        console.log("[Store] Disparando ConsolidateChatKnowledge para sessão:", sessionID);
        await safeCall('main', 'ConsolidateChatKnowledge', sessionID, lastMessages);
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

    try {
      await safeCall('main', 'AskAgent', agent, prompt);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro: ${err}`, mode: 'system' });
      isThinking.value = false;
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

    try {
      // 🛠️ SINCRONIZAÇÃO CRÍTICA: Agora enviamos 3 argumentos conforme o novo contrato Go
      const resp = await safeCall('main', 'SendAgentInput', agent, text, images);
      return resp;
    } catch (err) {
      console.error('[Store] Erro ao enviar input:', err);
      isThinking.value = false;
      stopSafetyTimeout();
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
    messages, isThinking, isTerminalMode, activeAgent, runningSessions, pendingReview,
    sessions, currentACPID, isSidebarOpen,
    initListeners, ask, startSession, sendInput, submitReview, switchAgent, stopSession,
    fetchSessions, loadSession, newSession, toggleSidebar
  };
});
