import { defineStore } from 'pinia';
import { ref } from 'vue';
import { EventsOn } from '../../wailsjs/runtime/runtime';

// Helper para chamar funções do Wails com segurança (evita crash se undefined)
const safeCall = async (pkg, func, ...args) => {
  try {
    if (window.go && window.go.main && window.go.main.App && window.go.main.App[func]) {
      return await window.go.main.App[func](...args);
    }
    console.warn(`[Wails SafeCall] Função ${func} não encontrada (Ambiente Dev/Browser?)`);
    return null;
  } catch (err) {
    console.error(`[Wails SafeCall] Erro ao chamar ${func}:`, err);
    throw err;
  }
};

export const useOrchestratorStore = defineStore('orchestrator', () => {
  // --- Estados Reativos (State) ---
  const messages = ref([]);
  const isThinking = ref(false);
  const isTerminalMode = ref(false);
  const isRealPTY = ref(false);
  const activeAgent = ref(null);        // Agente visível no momento
  const runningSessions = ref([]);       // Lista de agentes com sessão ativa (ex: ['gemini', 'claude'])
  const outputBuffer = ref("");

  // --- Inicialização de Eventos (Ouvir o Backend Go) ---
  const initListeners = () => {
    // Escuta logs da IA (Streaming)
    EventsOn('agent:log', (log) => {
      if (log.source === 'CRAWLER') {
         messages.value.push({ role: 'assistant', text: log.content, mode: 'system' });
         return;
      }

      if (log.role === 'assistant') {
        const lastMsg = messages.value[messages.value.length - 1];
        if (lastMsg && lastMsg.role === 'assistant' && lastMsg.mode !== 'system') {
           lastMsg.text += log.content;
        } else {
           messages.value.push({ role: 'assistant', text: log.content, agent: log.agent || activeAgent.value });
        }
      }
    });

    // Escuta status da Sessão PTY
    EventsOn('terminal:started', (info) => {
      isRealPTY.value = !!info?.isRealPTY;
      const agent = info?.agent;
      if (agent && !runningSessions.value.includes(agent)) {
        runningSessions.value.push(agent);
      }
      if (!activeAgent.value && agent) {
        activeAgent.value = agent;
        isTerminalMode.value = true;
      }
    });

    // Escuta encerramento de sessão  
    EventsOn('terminal:closed', (agent) => {
      runningSessions.value = runningSessions.value.filter(a => a !== agent);
      if (activeAgent.value === agent) {
        if (runningSessions.value.length > 0) {
          activeAgent.value = runningSessions.value[0];
        } else {
          activeAgent.value = null;
          isTerminalMode.value = false;
          isRealPTY.value = false;
        }
      }
      isThinking.value = false;
      messages.value.push({ role: 'assistant', text: `Sessão ${agent} encerrada.`, mode: 'system' });
    });

    // Escuta logs brutos de execução
    EventsOn('execution:log', (log) => {
       if (log.source === 'SYSTEM') {
         messages.value.push({ role: 'assistant', text: `⚙️ ${log.content}`, mode: 'system' });
       }
    });
  };

  // --- Ações (Actions) com SafeCall ---
  const ask = async (prompt) => {
    isThinking.value = true;
    messages.value.push({ role: 'user', text: prompt });
    try {
      await safeCall('main', 'AskAgent', prompt);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Erro: ${err}`, mode: 'system' });
      isThinking.value = false;
    }
  };

  const startSession = async (agent) => {
    isThinking.value = true;
    isTerminalMode.value = true;
    activeAgent.value = agent;
    
    messages.value.push({ role: 'user', text: `/cmd ${agent}`, mode: 'system' });

    try {
      await safeCall('main', 'StartAgentSession', agent);
    } catch (err) {
      messages.value.push({ role: 'assistant', text: `❌ Falha ao iniciar sessão PTY: ${err}`, mode: 'system' });
      isTerminalMode.value = false;
      isThinking.value = false;
    }
  };

  const switchAgent = (agent) => {
    if (runningSessions.value.includes(agent)) {
      activeAgent.value = agent;
    }
  };

  const stopSession = async (agent) => {
    const target = agent || activeAgent.value;
    if (!target) return;
    await safeCall('main', 'StopAgentSession', target);
  };

  const stopAllSessions = async () => {
    for (const agent of [...runningSessions.value]) {
      await safeCall('main', 'StopAgentSession', agent);
    }
    isTerminalMode.value = false;
    isRealPTY.value = false;
    isThinking.value = false;
    activeAgent.value = null;
  };

  const sendInput = async (text) => {
    if (!isTerminalMode.value || !activeAgent.value) return;
    return await safeCall('main', 'SendAgentInput', activeAgent.value, text);
  };

  const runScan = async () => {
     messages.value.push({ role: 'assistant', text: "Iniciando indexação do Vault...", mode: 'system' });
     await safeCall('main', 'ScanVault');
  };

  return {
    messages,
    isThinking,
    isTerminalMode,
    isRealPTY,
    activeAgent,
    runningSessions,
    initListeners,
    ask,
    startSession,
    switchAgent,
    stopSession,
    stopAllSessions,
    sendInput,
    runScan
  };
});
