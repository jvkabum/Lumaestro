<script setup>
import { storeToRefs } from 'pinia'
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'
import ChatInput from './ChatInput.vue'
import ChatLog from './ChatLog.vue'
import TerminalView from './TerminalView.vue'
import ReviewBlock from './ReviewBlock.vue'

// --- Uso da Store (Pinia) ---
const orchestrator = useOrchestratorStore()
const { messages, isThinking, isNavigating, isTerminalMode, activeAgent, runningSessions, pendingReview, statusTimeline, statusFilter, currentStatusKind } = storeToRefs(orchestrator)

// --- Estados Locais de UI ---
const logContainer = ref(null)
const showRawTerminal = ref(false)
const processingElapsed = ref(0)
let processingTimer = null

const filterOptions = [
  { label: 'TODOS', value: 'all' },
  { label: 'THINK', value: 'think' },
  { label: 'TOOL', value: 'tool' },
  { label: 'COMMAND', value: 'command' },
  { label: 'MEMORY', value: 'memory' },
  { label: 'ERROR', value: 'error' },
  { label: 'STATUS', value: 'status' },
]

const filteredTimeline = computed(() => {
  if (statusFilter.value === 'all') return statusTimeline.value || []
  return (statusTimeline.value || []).filter((item) => item.kind === statusFilter.value)
})

const processingStages = [
  'Interpretando sua solicitação',
  'Explorando contexto e memória',
  'Executando raciocínio e ferramentas',
  'Montando a melhor resposta',
]

const activeEngineLabel = computed(() => {
  const engine = orchestrator.activeProfile?.name || activeAgent.value || 'IA'
  return String(engine).toUpperCase()
})

const processingStage = computed(() => {
  if (orchestrator.currentStatus) return orchestrator.currentStatus
  const index = Math.min(Math.floor(processingElapsed.value / 4), processingStages.length - 1)
  return processingStages[index]
})

const processingKindLabel = computed(() => {
  const kind = currentStatusKind.value || 'status'
  if (kind === 'tool') return 'TOOL'
  if (kind === 'command') return 'COMMAND'
  if (kind === 'memory') return 'MEMORY'
  if (kind === 'error') return 'ERROR'
  return 'STATUS'
})

const processingElapsedLabel = computed(() => {
  const total = processingElapsed.value
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  if (minutes === 0) return `${seconds}s em processamento`
  return `${minutes}m ${String(seconds).padStart(2, '0')}s em processamento`
})

const startProcessingTimer = () => {
  if (processingTimer) return
  processingTimer = window.setInterval(() => {
    processingElapsed.value += 1
  }, 1000)
}

const stopProcessingTimer = () => {
  if (processingTimer) {
    window.clearInterval(processingTimer)
    processingTimer = null
  }
}

watch(isThinking, (value) => {
  if (value) {
    processingElapsed.value = 0
    startProcessingTimer()
    return
  }

  stopProcessingTimer()
  processingElapsed.value = 0
}, { immediate: true })

onUnmounted(() => {
  stopProcessingTimer()
})

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

  // Envio Padrão (Multimodal)
  const targetAgent = payload.agent || 'gemini'
  const isActMode = payload.mode === 'act'
  const images = payload.images || [] // Captura as imagens do Ctrl+V

  if (isActMode) {
    // Garante que a sessão está ativa antes de enviar
    if (!runningSessions.value.includes(targetAgent)) {
      await orchestrator.startSession(targetAgent)
      // Pequeno delay para a sessão inicializar no backend
      await new Promise(r => setTimeout(r, 500))
    }
    // 🛠️ SINCRONIZAÇÃO: Enviando texto e imagens capturadas
    await orchestrator.sendInput(targetAgent, text, images)
  } else {
    // Modo CHAT (Legacy/RAG) - Agora também aceita contexto visual
    await orchestrator.ask(targetAgent, text, images)
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
          <span 
            v-if="orchestrator.activeProfile" 
            class="active-agent-badge" 
            :class="((orchestrator.activeProfile.engine || orchestrator.activeProfile.name || '').toLowerCase()).replace(/\s+/g, '')"
          >
            {{ orchestrator.activeProfile.name.toUpperCase() }}
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

        <Transition name="status-fade">
          <div v-if="isThinking" class="processing-beacon glass" :class="`kind-${currentStatusKind || 'status'}`">
            <div class="processing-core">
              <span class="processing-orb"></span>
              <span class="processing-ring ring-one"></span>
              <span class="processing-ring ring-two"></span>
            </div>
            <div class="processing-copy">
              <div class="processing-title">{{ activeEngineLabel }} EM PROCESSAMENTO</div>
              <div class="processing-stage">{{ processingStage }}</div>
              <div class="processing-meta">
                <span class="processing-kind-chip">{{ processingKindLabel }}</span>
                <span class="processing-dots"><i></i><i></i><i></i></span>
                <span>{{ processingElapsedLabel }}</span>
              </div>
            </div>
          </div>
        </Transition>

        <!-- 📡 Pulso de Atividade: Mostra o que a IA está fazendo AGORA (Anti-Travamento) -->
        <Transition name="status-fade">
          <div v-if="orchestrator.currentStatus && orchestrator.currentStatus.action" class="activity-status-bar glass">
            <div class="activity-pulse"></div>
            <div class="activity-info">
              <span v-if="orchestrator.currentStatus.tool" class="activity-tool">{{ orchestrator.currentStatus.tool.replace('_', ' ').toUpperCase() }}</span>
              <span class="activity-text">{{ orchestrator.currentStatus.action }}</span>
            </div>
          </div>
        </Transition>

        <Transition name="status-fade">
          <div v-if="statusTimeline.length > 0" class="activity-window glass">
            <div class="activity-window-header">
              <span>ATIVIDADE DO AGENTE</span>
              <div class="activity-window-controls">
                <button
                  v-for="opt in filterOptions"
                  :key="opt.value"
                  class="activity-filter-btn"
                  :class="{ active: statusFilter === opt.value }"
                  @click="statusFilter = opt.value"
                >
                  {{ opt.label }}
                </button>
                <button class="activity-clear-btn" @click="orchestrator.clearStatusTimeline()">LIMPAR</button>
              </div>
            </div>
            <div class="activity-window-list">
              <div v-for="item in filteredTimeline.slice(-10).reverse()" :key="item.id" class="activity-window-item" :class="`kind-${item.kind || 'status'}`">
                <span class="activity-time">{{ item.at }}</span>
                <span class="activity-line">{{ item.text }}</span>
              </div>
            </div>
          </div>
        </Transition>

        <!-- Indicador de Navegação do Grafo (Context Flow) -->
        <Transition name="slide-up">
          <div v-if="isNavigating" class="navigation-status glass">
            <span class="nav-pulse"></span>
            <span class="nav-text">Explorando Base de Conhecimento...</span>
          </div>
        </Transition>
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
            :class="{ active: activeAgent === agent, gemini: agent === 'gemini', claude: agent === 'claude', lmstudio: agent === 'lmstudio' }"
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
.active-agent-badge.lmstudio { background: rgba(20, 184, 166, 0.12); color: #2dd4bf; }
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
.terminal-tab.active.lmstudio { background: rgba(20, 184, 166, 0.15); border-color: rgba(20, 184, 166, 0.3); color: #2dd4bf; }
.tab-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.terminal-stack { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.back-btn {
  background: #222; border: 1px solid #333; color: #888; padding: 4px 12px;
  border-radius: 4px; cursor: pointer; font-size: 11px;
}

.activity-window {
  margin: 10px 20px 0 20px;
  border: 1px solid rgba(56, 189, 248, 0.22);
  background: rgba(2, 6, 23, 0.55);
  border-radius: 10px;
  overflow: hidden;
}

.activity-window-header {
  padding: 8px 12px;
  font-size: 10px;
  letter-spacing: 1px;
  font-weight: 800;
  color: #7dd3fc;
  border-bottom: 1px solid rgba(125, 211, 252, 0.16);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.activity-window-controls {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.activity-filter-btn,
.activity-clear-btn {
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(15, 23, 42, 0.5);
  color: #94a3b8;
  border-radius: 999px;
  padding: 2px 8px;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 0.5px;
  cursor: pointer;
}

.activity-filter-btn.active {
  border-color: rgba(56, 189, 248, 0.45);
  color: #bae6fd;
  background: rgba(14, 116, 144, 0.35);
}

.activity-clear-btn {
  border-color: rgba(239, 68, 68, 0.35);
  color: #fca5a5;
}

.activity-window-list {
  max-height: 140px;
  overflow-y: auto;
  padding: 8px 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.activity-window-item {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  font-size: 12px;
  line-height: 1.35;
}

.activity-time {
  color: #94a3b8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace;
  flex-shrink: 0;
}

.activity-line {
  color: #cbd5e1;
}

.activity-window-item.kind-think .activity-line { color: #c4b5fd; }
.activity-window-item.kind-tool .activity-line { color: #7dd3fc; }
.activity-window-item.kind-command .activity-line { color: #86efac; }
.activity-window-item.kind-memory .activity-line { color: #fcd34d; }
.activity-window-item.kind-error .activity-line { color: #fca5a5; }

.processing-beacon {
  margin: 10px 20px 0 20px;
  padding: 14px 16px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  gap: 14px;
  background: linear-gradient(135deg, rgba(15, 23, 42, 0.78), rgba(17, 24, 39, 0.54));
  border: 1px solid rgba(96, 165, 250, 0.18);
  box-shadow: 0 10px 30px rgba(2, 6, 23, 0.28);
}

.processing-beacon.kind-tool {
  border-color: rgba(125, 211, 252, 0.24);
}

.processing-beacon.kind-command {
  border-color: rgba(134, 239, 172, 0.24);
}

.processing-beacon.kind-memory {
  border-color: rgba(252, 211, 77, 0.24);
}

.processing-beacon.kind-error {
  border-color: rgba(252, 165, 165, 0.28);
}

.processing-core {
  position: relative;
  width: 34px;
  height: 34px;
  flex-shrink: 0;
}

.processing-orb {
  position: absolute;
  inset: 9px;
  border-radius: 999px;
  background: radial-gradient(circle, #93c5fd 0%, #3b82f6 55%, #1d4ed8 100%);
  box-shadow: 0 0 22px rgba(59, 130, 246, 0.55);
  animation: processing-orb-pulse 1.8s infinite ease-in-out;
}

.processing-ring {
  position: absolute;
  inset: 0;
  border-radius: 999px;
  border: 1px solid rgba(96, 165, 250, 0.28);
  animation: processing-ring-expand 2.4s infinite ease-out;
}

.processing-ring.ring-two {
  animation-delay: 1.2s;
}

.processing-copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.processing-title {
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 1.1px;
  color: #7dd3fc;
}

.processing-stage {
  font-size: 13px;
  font-weight: 700;
  color: #e2e8f0;
}

.processing-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 700;
}

.processing-kind-chip {
  border-radius: 999px;
  padding: 3px 7px;
  font-size: 9px;
  font-weight: 900;
  letter-spacing: 0.8px;
  color: #bfdbfe;
  background: rgba(59, 130, 246, 0.14);
  border: 1px solid rgba(96, 165, 250, 0.2);
}

.processing-beacon.kind-tool .processing-kind-chip {
  color: #bae6fd;
  background: rgba(14, 116, 144, 0.26);
  border-color: rgba(125, 211, 252, 0.22);
}

.processing-beacon.kind-command .processing-kind-chip {
  color: #bbf7d0;
  background: rgba(21, 128, 61, 0.22);
  border-color: rgba(134, 239, 172, 0.24);
}

.processing-beacon.kind-memory .processing-kind-chip {
  color: #fde68a;
  background: rgba(161, 98, 7, 0.22);
  border-color: rgba(252, 211, 77, 0.24);
}

.processing-beacon.kind-error .processing-kind-chip {
  color: #fecaca;
  background: rgba(153, 27, 27, 0.22);
  border-color: rgba(252, 165, 165, 0.24);
}

.processing-dots {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.processing-dots i {
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: #60a5fa;
  animation: processing-dot-bounce 1.1s infinite ease-in-out;
}

.processing-dots i:nth-child(2) {
  animation-delay: 0.15s;
}

.processing-dots i:nth-child(3) {
  animation-delay: 0.3s;
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

@keyframes processing-orb-pulse {
  0%, 100% { transform: scale(0.92); opacity: 0.9; }
  50% { transform: scale(1.06); opacity: 1; }
}

@keyframes processing-ring-expand {
  0% { transform: scale(0.7); opacity: 0; }
  20% { opacity: 0.65; }
  100% { transform: scale(1.25); opacity: 0; }
}

@keyframes processing-dot-bounce {
  0%, 80%, 100% { transform: translateY(0); opacity: 0.4; }
  40% { transform: translateY(-4px); opacity: 1; }
}

/* Transições */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.5s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
/* 📡 Activity Status Bar (Anti-Travamento) */
.activity-status-bar {
  position: absolute;
  bottom: 120px;
  left: 20px;
  right: 20px;
  padding: 10px 16px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  gap: 12px;
  z-index: 50;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
  pointer-events: none;
}

.activity-pulse {
  width: 6px;
  height: 6px;
  background: #3b82f6;
  border-radius: 50%;
  animation: activity-glow 1.2s infinite ease-in-out;
  box-shadow: 0 0 8px #3b82f6;
}

@keyframes activity-glow {
  0%, 100% { transform: scale(1); opacity: 0.5; }
  50% { transform: scale(1.5); opacity: 1; }
}

.activity-text {
  font-size: 11px;
  font-weight: 500;
  color: #94a3b8;
  letter-spacing: 0.5px;
}

.activity-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.activity-tool {
  font-size: 9px;
  font-weight: 900;
  color: #3b82f6;
  letter-spacing: 1px;
}

/* Perfis de Identidade Visual */
.active-agent-badge.doc-master { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.2); }
.active-agent-badge.coder { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.2); }
.active-agent-badge.planner { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.2); }

.status-fade-enter-active, .status-fade-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.status-fade-enter-from, .status-fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

@media (max-width: 640px) {
  .processing-beacon {
    margin: 10px 12px 0 12px;
    padding: 12px 14px;
    gap: 12px;
  }

  .processing-stage {
    font-size: 12px;
  }

  .processing-meta {
    gap: 8px;
    font-size: 10px;
    flex-wrap: wrap;
  }

  .activity-status-bar {
    left: 12px;
    right: 12px;
    bottom: 112px;
  }
}

</style>
