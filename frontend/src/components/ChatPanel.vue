<script setup>
import { storeToRefs } from 'pinia'
import { onMounted, ref, watch } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'
import ChatInput from './ChatInput.vue'
import ChatLog from './ChatLog.vue'
import TerminalView from './TerminalView.vue'
import ReviewBlock from './ReviewBlock.vue'

// --- Uso da Store (Pinia) ---
const orchestrator = useOrchestratorStore()
const { messages, isThinking, isTerminalMode, activeAgent, runningSessions, pendingReview } = storeToRefs(orchestrator)

// --- Estados Locais de UI ---
const logContainer = ref(null)
const showRawTerminal = ref(false)

// O Terminal Bruto (Raw) só deve abrir via botão ou comando explícito (/cmd)
// para garantir que a experiência primária (Chat) não seja interrompida.


// Inicializa a escuta de eventos do Backend Go
onMounted(() => {
  orchestrator.initListeners()
})

// --- Ações de UI ---
const sendChatMessage = async (payload) => {
  const text = typeof payload === 'string' ? payload : payload.text
  if (!text.trim()) return

  // Roteamento de Comandos
  if (text.startsWith('/cmd ')) {
    const agentName = text.replace('/cmd ', '').trim()
    await orchestrator.startSession(agentName || 'gemini')
    showRawTerminal.value = true // Força o visual do terminal ao abrir sessão
    return
  }

  if (text === '/exit' || text === '/quit') {
    await orchestrator.stopSession()
    return
  }

  if (text === '/scan') {
    await orchestrator.runScan()
    return
  }

  // Envio Padrão
  const targetAgent = payload.agent || 'gemini'
  const isActMode = payload.mode === 'act'

  if (isActMode) {
    // Garante que a sessão está ativa antes de enviar
    if (!runningSessions.value.includes(targetAgent)) {
      await orchestrator.startSession(targetAgent)
      // Pequeno delay para a sessão inicializar no backend
      await new Promise(r => setTimeout(r, 500))
    }
    await orchestrator.sendInput(targetAgent, text)
  } else {
    // Modo CHAT (Legacy/RAG) - Sem PTY, apenas requisição direta
    await orchestrator.ask(targetAgent, text)
  }
}

const handleSessionEnded = (agent) => {
    console.log('[ChatPanel] Sessão encerrada:', agent)
}
</script>

<template>
  <div class="chat-panel-parent">
    <!-- Grade de Fundo Sutil -->
    <div class="panel-grain"></div>

    <!-- Sistema de Revisão de Segurança (ACP Hands) -->
    <ReviewBlock v-if="pendingReview" :review="pendingReview" />

    <header class="panel-header glass">
      <div class="header-left">
        <span class="orchestra-icon">🎻</span>
        <div class="header-titles">
          <h2>MAESTRO</h2>
          <span v-if="activeAgent" class="active-agent-badge" :class="activeAgent">
            {{ activeAgent.toUpperCase() }} ACTIVE
          </span>
          <span v-else class="active-agent-badge standby">STANDBY</span>
        </div>
      </div>
      <div class="header-actions">
        <!-- Toggle Terminal View -->
        <button @click="showRawTerminal = !showRawTerminal" class="action-btn" :class="{ 'btn-active': showRawTerminal }" title="Alternar Terminal Bruto">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
            <line x1="8" y1="21" x2="16" y2="21"></line>
            <line x1="12" y1="17" x2="12" y2="21"></line>
          </svg>
        </button>

        <!-- Botão Discreto de Histórico (Relógio/Lista) -->
        <button @click="orchestrator.toggleSidebar()" class="action-btn" :class="{ 'btn-active-history': orchestrator.isSidebarOpen }" title="Expandir Histórico de Sinfonias">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="12 8 12 12 14 14"></polyline>
            <path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5"></path>
          </svg>
        </button>
        
        <button v-if="isTerminalMode" @click="orchestrator.stopSession()" class="exit-btn-circle" title="Encerrar Sessão">
           <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="3">
             <line x1="18" y1="6" x2="6" y2="18"></line>
             <line x1="6" y1="6" x2="18" y2="18"></line>
           </svg>
        </button>
      </div>
    </header>

    <!-- Área Principal do Chat (Premium) -->
    <div v-show="!showRawTerminal" class="chat-main-area" ref="logContainer">
      <div class="chat-scroll-boundary">
        <!-- Tela de Harmonização Inicial (Loading Screen) -->
        <Transition name="fade">
          <div v-if="isThinking && messages.length === 0" class="loading-overlay glass">
            <div class="loader-content">
              <div class="pulsing-icon">🎻</div>
              <div class="loader-text">
                <h3>Afinando instrumentos...</h3>
                <p>O Maestro está sintonizando com o Gemini.</p>
              </div>
              <div class="loader-bars">
                <span></span><span></span><span></span><span></span><span></span>
              </div>
            </div>
          </div>
        </Transition>

        <ChatLog :messages="messages" :is-thinking="isThinking" />
      </div>
      <div class="input-persistent-area">
        <ChatInput @send="sendChatMessage" :is-thinking="isThinking" />
      </div>
    </div>

    <!-- Terminal Real com TABS de Agentes -->
    <div v-show="showRawTerminal" class="raw-terminal-view">
      <div class="terminal-overlay-header">
        <div class="terminal-tabs">
          <button
            v-for="agent in runningSessions"
            :key="agent"
            class="terminal-tab"
            :class="{ active: activeAgent === agent, gemini: agent === 'gemini', claude: agent === 'claude' }"
            @click="orchestrator.switchAgent(agent)"
          >
            <span class="tab-dot"></span>
            {{ agent }}
          </button>
          <span v-if="runningSessions.length === 0" class="no-sessions">Nenhuma sessão ativa</span>
        </div>
        <div class="terminal-actions">
          <button @click="showRawTerminal = false" class="back-btn">Chat View</button>
        </div>
      </div>
      <!-- Uma instância de TerminalView para cada agente ativo -->
       <div class="terminal-stack">
        <TerminalView
          v-for="agent in runningSessions"
          :key="agent"
          :agent="agent"
          :active="activeAgent === agent"
          @session-ended="handleSessionEnded"
        />
       </div>
    </div>
  </div>
</template>

<style scoped>
.chat-panel-parent {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #0f172a; /* Slate 900 mais profundo */
  position: relative;
  overflow: hidden;
  border-radius: 12px 12px 0 0;
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4);
}

.panel-grain {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.65' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E");
  opacity: 0.02;
  pointer-events: none;
  z-index: 1;
}

.panel-header {
  height: 64px;
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  z-index: 10;
  background: rgba(15, 23, 42, 0.7);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.header-left { display: flex; align-items: center; gap: 14px; }
.orchestra-icon { font-size: 1.2rem; filter: drop-shadow(0 0 8px rgba(59, 130, 246, 0.5)); }

.header-titles h2 {
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 3px;
  margin: 0;
  color: #94a3b8;
}

.active-agent-badge {
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 1px;
  padding: 2px 6px;
  border-radius: 100px;
  background: rgba(255, 255, 255, 0.05);
  color: #64748b;
  margin-top: 2px;
  display: inline-block;
}

.active-agent-badge.gemini { background: rgba(59, 130, 246, 0.1); color: #60a5fa; }
.active-agent-badge.claude { background: rgba(16, 185, 129, 0.1); color: #34d399; }
.active-agent-badge.standby { background: rgba(245, 158, 11, 0.1); color: #fbbf24; }

.header-actions { display: flex; align-items: center; gap: 10px; }

.action-btn {
  background: transparent; border: none; color: #64748b; cursor: pointer;
  padding: 8px; border-radius: 8px; transition: all 0.2s;
}
.action-btn:hover { background: rgba(255, 255, 255, 0.05); color: #fff; }
.action-btn.btn-active-history { color: #38bdf8; background: rgba(56, 189, 248, 0.1); border: 1px solid rgba(56, 189, 248, 0.2); }
.action-btn.btn-active { color: #3b82f6; background: rgba(59, 130, 246, 0.1); }

.exit-btn-circle {
  background: #ef4444; border: none; color: white; width: 28px; height: 28px;
  border-radius: 50%; cursor: pointer; display: flex; align-items: center; justify-content: center;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.4); transition: transform 0.2s;
}
.exit-btn-circle:hover { transform: scale(1.1) rotate(90deg); }

.chat-main-area { flex: 1; display: flex; flex-direction: column; min-height: 0; z-index: 5; }
.chat-scroll-boundary { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.input-persistent-area { padding: 16px 20px 24px 20px; background: linear-gradient(to top, #0f172a 80%, transparent); }

.raw-terminal-view { flex: 1; display: flex; flex-direction: column; background: #000; min-height: 0; }
.terminal-overlay-header {
  padding: 6px 16px; background: #111; border-bottom: 1px solid #222;
  display: flex; justify-content: space-between; align-items: center;
}
.terminal-tabs { display: flex; gap: 4px; align-items: center; }
.terminal-tab {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 14px; border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  color: #64748b; font-size: 11px; font-weight: 800;
  text-transform: uppercase; cursor: pointer;
  transition: all 0.2s;
}
.terminal-tab.active { color: #f8fafc; border-color: rgba(255, 255, 255, 0.15); }
.terminal-tab.active.gemini { background: rgba(59, 130, 246, 0.15); border-color: rgba(59, 130, 246, 0.3); color: #60a5fa; }
.terminal-tab.active.claude { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.3); color: #34d399; }
.tab-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.terminal-stack { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.back-btn {
  background: #222; border: 1px solid #333; color: #888; padding: 4px 12px;
  border-radius: 4px; cursor: pointer; font-size: 11px;
}

/* --- Loading Overlay & Splah Screen --- */
.loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.85);
  backdrop-filter: blur(12px);
  z-index: 100;
}

.loader-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  text-align: center;
}

.pulsing-icon {
  font-size: 3rem;
  animation: heart-pulse 2s infinite ease-in-out;
}

.loader-text h3 {
  font-size: 1.2rem;
  font-weight: 700;
  color: #fff;
  margin-bottom: 4px;
  letter-spacing: 0.5px;
}

.loader-text p {
  font-size: 0.9rem;
  color: #94a3b8;
}

.loader-bars {
  display: flex;
  gap: 4px;
  height: 20px;
  align-items: flex-end;
}

.loader-bars span {
  width: 3px;
  height: 100%;
  background: #3b82f6;
  border-radius: 2px;
  animation: bar-rise 1s infinite ease-in-out;
}

.loader-bars span:nth-child(2) { animation-delay: 0.1s; height: 70%; }
.loader-bars span:nth-child(3) { animation-delay: 0.2s; height: 90%; }
.loader-bars span:nth-child(4) { animation-delay: 0.3s; height: 60%; }
.loader-bars span:nth-child(5) { animation-delay: 0.4s; height: 80%; }

@keyframes heart-pulse {
  0%, 100% { transform: scale(1); filter: drop-shadow(0 0 10px rgba(59, 130, 246, 0.3)); }
  50% { transform: scale(1.1); filter: drop-shadow(0 0 20px rgba(59, 130, 246, 0.6)); }
}

@keyframes bar-rise {
  0%, 100% { height: 40%; }
  50% { height: 100%; }
}

/* Transições */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.5s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
