<script setup>
import { storeToRefs } from 'pinia'
import { onMounted, ref } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'
import ChatInput from './ChatInput.vue'
import ChatLog from './ChatLog.vue'
import TerminalView from './TerminalView.vue'

// --- Uso da Store (Pinia) ---
const orchestrator = useOrchestratorStore()
const { messages, isThinking, isTerminalMode, activeAgent } = storeToRefs(orchestrator)

// --- Estados Locais de UI ---
const logContainer = ref(null)
const showRawTerminal = ref(false)

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
  if (isTerminalMode.value) {
    await orchestrator.sendInput(text)
  } else {
    await orchestrator.ask(text)
  }
}

const handleSessionEnded = () => {
    orchestrator.isTerminalMode = false
    orchestrator.isThinking = false
}
</script>

<template>
  <div class="chat-panel-parent">
    <!-- Grade de Fundo Sutil -->
    <div class="bg-grid"></div>

    <header class="premium-header">
      <div class="header-left">
        <div class="logo-area">
          <div class="logo-icon">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"></path>
            </svg>
          </div>
          <div class="logo-text">
            <span class="brand">Lumaestro</span>
            <span class="version">v1.2.0</span>
          </div>
        </div>
      </div>

      <div class="header-right">
        <!-- Indicador de Status do PTY -->
        <div class="status-chip" :class="{ 'active': isTerminalMode }">
          <span class="status-dot" :class="{ 'pulsing': isTerminalMode }"></span>
          <span class="status-label">{{ isTerminalMode ? 'Sinfonia Ativa' : 'Standby' }}</span>
        </div>
        
        <div class="agent-selector-mini" v-if="activeAgent" :class="activeAgent.toLowerCase()">
           <span class="agent-dot"></span>
           <span class="agent-name">{{ activeAgent }}</span>
        </div>

        <button @click="showRawTerminal = !showRawTerminal" class="action-btn" title="Alternar Terminal Bruto">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
            <line x1="8" y1="21" x2="16" y2="21"></line>
            <line x1="12" y1="17" x2="12" y2="21"></line>
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
    <div v-if="!showRawTerminal" class="chat-main-area" ref="logContainer">
      <div class="chat-scroll-boundary">
        <ChatLog :messages="messages" :is-thinking="isThinking" />
      </div>
      <div class="input-persistent-area">
        <ChatInput @send="sendChatMessage" :is-thinking="isThinking" />
      </div>
    </div>

    <!-- Terminal Real (Visível apenas em modo Debug ou quando solicitado) -->
    <div v-show="showRawTerminal" class="raw-terminal-view">
      <div class="terminal-overlay-header">
         <span>Terminal Console (Low Level)</span>
         <button @click="showRawTerminal = false">Voltar para o Chat</button>
      </div>
      <TerminalView
        :agent="activeAgent"
        :active="isTerminalMode"
        @session-ended="handleSessionEnded"
      />
    </div>
  </div>
</template>

<style scoped>
.chat-panel-parent {
  height: 100vh;
  min-width: 500px; /* Alinhado com minChatWidth do App.vue */
  display: flex;
  flex-direction: column;
  background: #09090b; /* Darker near black */
  position: relative;
  overflow: hidden;
  color: #f8fafc;
}

.bg-grid {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background-image: 
    radial-gradient(circle at 2px 2px, rgba(255, 255, 255, 0.03) 1px, transparent 0);
  background-size: 32px 32px;
  pointer-events: none;
  z-index: 0;
}

.premium-header {
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  background: rgba(9, 9, 11, 0.7);
  backdrop-filter: blur(20px) saturate(180%);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  z-index: 10;
}

.logo-area { display: flex; align-items: center; gap: 12px; }
.logo-icon {
  width: 36px;
  height: 36px;
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  box-shadow: 0 4px 15px rgba(37, 99, 235, 0.4);
}

.logo-text { display: flex; flex-direction: column; line-height: 1.2; }
.brand { font-weight: 800; font-size: 16px; letter-spacing: -0.5px; }
.version { font-size: 10px; color: #64748b; font-weight: 600; font-family: 'JetBrains Mono', monospace; }

.header-right { display: flex; align-items: center; gap: 16px; }

.status-chip {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(255, 255, 255, 0.04);
  padding: 6px 14px;
  border-radius: 100px;
  border: 1px solid rgba(255, 255, 255, 0.06);
}
.status-chip.active { background: rgba(16, 185, 129, 0.08); border-color: rgba(16, 185, 129, 0.2); }

.status-dot { width: 6px; height: 6px; border-radius: 50%; background: #64748b; }
.status-chip.active .status-dot { background: #10b981; }

.status-label { font-size: 11px; font-weight: 700; color: #94a3b8; text-transform: uppercase; letter-spacing: 0.5px; }
.status-chip.active .status-label { color: #34d399; }

.pulsing { animation: pulseStatus 2s infinite; }
@keyframes pulseStatus {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.5); opacity: 0.5; }
  100% { transform: scale(1); opacity: 1; }
}

.agent-selector-mini {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  font-size: 11px;
  font-weight: 800;
  text-transform: uppercase;
  color: #94a3b8;
}
.agent-selector-mini.terminal { color: #f59e0b; background: rgba(245, 158, 11, 0.1); }
.agent-selector-mini.claude { color: #10b981; background: rgba(16, 185, 129, 0.1); }
.agent-selector-mini.gemini { color: #3b82f6; background: rgba(59, 130, 246, 0.1); }

.agent-dot { width: 4px; height: 4px; border-radius: 50%; background: currentColor; }

.action-btn { background: none; border: none; color: #64748b; cursor: pointer; padding: 6px; transition: color 0.2s; }
.action-btn:hover { color: #f8fafc; }

.exit-btn-circle {
  width: 28px; height: 28px; border-radius: 50%; background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2); color: #ef4444;
  display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.2s;
}
.exit-btn-circle:hover { background: #ef4444; color: white; transform: rotate(90deg); }

.chat-main-area {
  flex: 1; display: flex; flex-direction: column; min-height: 0;
}
.chat-scroll-boundary { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.input-persistent-area { padding: 0 24px 10px 24px; position: relative; z-index: 5; }

.raw-terminal-view { flex: 1; display: flex; flex-direction: column; background: #000; }
.terminal-overlay-header {
  padding: 8px 16px; background: #111; border-bottom: 1px solid #222;
  display: flex; justify-content: space-between; align-items: center;
  font-size: 11px; color: #555; font-family: 'JetBrains Mono', monospace;
}
.terminal-overlay-header button {
  background: #222; border: 1px solid #333; color: #888; padding: 2px 8px; border-radius: 3px; cursor: pointer;
}
</style>
