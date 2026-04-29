<script setup>
import { onMounted, reactive, ref } from 'vue'
import { CheckConnection, GetProjectDoc, GetToolsStatus } from '../wailsjs/go/core/App'
import { EventsOn } from '../wailsjs/runtime'
import ChatPanel from './components/ChatPanel.vue'
import GraphVisualizer from './components/GraphVisualizer.vue'
import HistorySidebar from './components/HistorySidebar.vue'
import Settings from './components/Settings.vue'
import DocViewer from './components/DocViewer.vue'
import SwarmDashboard from './components/SwarmDashboard.vue'
import AgentTerminal from './components/AgentTerminal.vue'
import ReposManager from './components/ReposManager.vue'
import MaestroConfirm from './components/MaestroConfirm.vue'
import { useOrchestratorStore } from './stores/orchestrator'
import { useSettingsStore } from './stores/settings'
const CACHE_BUST = "2026-04-21T17:44:00" // 🚀 Bypass de Cache

const orchestrator = useOrchestratorStore()
const settingsStore = useSettingsStore()
const currentView = ref('orchestrator') // views: orchestrator, settings, swarm
const isOnline = ref(false)
const connectionError = ref('Aguardando sincronização com o Maestro (Frontend Booting)...')

// Estado de Boot — Diagnóstico Visual
const bootStages = ref([])
const isBooting = ref(true)
const bootError = ref(null)

// Painel redimensionável
const chatWidth = ref(556)
const isResizing = ref(false)
const minChatWidth = 556
const maxChatWidth = 1400

// Minimização do Chat
const isChatMinimized = ref(false)
const toggleChat = () => { isChatMinimized.value = !isChatMinimized.value }

// Terminal Dock Inferior (Estilo VSCode)
const isTerminalDockOpen = ref(true)

const state = reactive({
  logs: [],
  nodes: [],
  edges: [],
  graphLogs: [],
  activeNode: null,
  // Estado para o Visuzalizador de Documentos
  docViewer: {
    isOpen: false,
    title: '',
    content: ''
  }
})

const openDoc = async (name, title) => {
  try {
    const content = await GetProjectDoc(name)
    state.docViewer.title = title
    state.docViewer.content = content
    state.docViewer.isOpen = true
  } catch (err) {
    console.error("Erro ao carregar documento:", err)
  }
}

// ── Resize Handle Logic ──
const startResize = (e) => {
  isResizing.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'

  const startX = e.clientX
  const startWidth = chatWidth.value

  const onMouseMove = (moveEvent) => {
    const delta = startX - moveEvent.clientX
    const newWidth = Math.min(maxChatWidth, Math.max(minChatWidth, startWidth + delta))
    chatWidth.value = newWidth
  }

  const onMouseUp = () => {
    isResizing.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

onMounted(async () => {
  // 🚀 [PRIORIDADE MÁXIMA] Registro Imediato de Listeners
  EventsOn('view:change', (view) => {
    currentView.value = view
  })

  // 🚀 [Mixer v2] Throttling de Logs + Streaming para Performance e Fluidez
  let logBuffer = []
  const flushLogs = () => {
    if (logBuffer.length === 0) return
    state.logs.push(...logBuffer)
    logBuffer = []
    if (state.logs.length > 500) state.logs = state.logs.slice(-500)
  }
  setInterval(flushLogs, 200)

  // Escuta os logs em tempo real
  EventsOn('agent:log', (log) => {
    if (!log || !log.content) return
    const lastLog = state.logs[state.logs.length - 1]
    
    // 🧠 Lógica de Streaming (Verde): Se for o Maestro e o último também for Maestro, anexa direto
    if (log.source === 'MAESTRO' && lastLog && lastLog.source === 'MAESTRO') {
      lastLog.content += log.content
    } else {
      // 🛡️ Lógica de Buffer (Vermelho): Protege contra estouro de logs de outros agentes
      logBuffer.push(log)
    }
  })

  // 🚀 Otimização de Massa: Processamento em lote (Batch Sync)
  EventsOn('graph:nodes:batch', (batchNodes) => {
    if (!batchNodes || batchNodes.length === 0) return
    const existingIds = new Set(state.nodes.map(n => n.id))
    const freshNodes = batchNodes.filter(n => !existingIds.has(n.id))
    if (freshNodes.length > 0) {
      state.nodes.push(...freshNodes)
    }
  })

  EventsOn('graph:edges:batch', (batchEdges) => {
    if (!batchEdges || batchEdges.length === 0) return
    state.edges.push(...batchEdges)
  })

  // 🚀 [Mixer v3] Buffer de Acumulação para Nós e Arestas individuais (anti-flood)
  let nodeBuffer = []
  let edgeBuffer = []
  const nodeIdSet = new Set(state.nodes.map(n => n.id))

  const flushGraph = () => {
    if (nodeBuffer.length > 0) {
      const fresh = nodeBuffer.filter(n => !nodeIdSet.has(n.id))
      for (const n of fresh) nodeIdSet.add(n.id)
      if (fresh.length > 0) state.nodes.push(...fresh)
      nodeBuffer = []
    }
    if (edgeBuffer.length > 0) {
      state.edges.push(...edgeBuffer)
      edgeBuffer = []
    }
  }
  setInterval(flushGraph, 200)

  EventsOn('graph:node', (node) => {
    if (!node) return
    nodeBuffer.push(node)
  })

  EventsOn('graph:edge', (edge) => {
    if (!edge) return
    const s = edge.source?.id || edge.source
    const t = edge.target?.id || edge.target
    if (!s || !t) return
    edgeBuffer.push(edge)
  })

  EventsOn('graph:log', (glog) => {
    if (!glog) return
    state.graphLogs.push(glog)
    if(state.graphLogs.length > 20) state.graphLogs.shift()
  })

  EventsOn('node:active', (nodeId) => {
    state.activeNode = nodeId
  })

  EventsOn('graph:clear', () => {
    console.log("[App] ☢️ Reset Total: Limpando estado local...")
    state.nodes = []
    state.edges = []
    state.graphLogs = []
    state.activeNode = null
    nodeIdSet.clear()
  })

  EventsOn('boot:stage', (data) => {
    if (data.stage === 'error') {
      bootError.value = data.message
      return
    }
    const index = bootStages.value.findIndex(s => s.stage === data.stage)
    if (index !== -1) {
      bootStages.value[index].message = data.message
      bootStages.value[index].icon = data.icon
    } else {
      bootStages.value.push({ ...data, done: false })
    }
    bootStages.value.forEach((s, i) => {
      if (i < bootStages.value.length - 1) s.done = true
    })
    if (data.stage === 'ready') {
      const readyIdx = bootStages.value.findIndex(s => s.stage === 'ready')
      if (readyIdx !== -1) bootStages.value[readyIdx].done = true
      setTimeout(() => { isBooting.value = false }, 1500)
    }
  })

  const tryConnect = async () => {
    try {
      isOnline.value = await CheckConnection()
      if (isOnline.value) {
        connectionError.value = "Maestro Online (Backend e Motor Vetorial Ativos)"
        isBooting.value = false
        
        // 🛠️ [Mixer] Atualiza badges de ferramentas globalmente após conexão
        const toolsStatus = await GetToolsStatus()
        if (toolsStatus) {
            settingsStore.status.tools = toolsStatus
        }
      } else {
        connectionError.value = "Backend respondeu, mas Qdrant ou Configuração falharam."
      }
    } catch(e) {
      isOnline.value = false
      connectionError.value = "Erro Wails IPC: " + String(e)
    }
  }

  await tryConnect()
  
  if (isOnline.value) {
    if (window.go?.core?.App?.TriggerInitialSync) {
        window.go.core.App.TriggerInitialSync()
    } else {
        setTimeout(() => {
            window.go?.core?.App?.TriggerInitialSync?.()
        }, 1000)
    }
  }

  const connInterval = setInterval(() => {
    if (!isOnline.value) tryConnect()
  }, 5000)

  setTimeout(() => {
    if (isBooting.value) isBooting.value = false
  }, 4000)
})
</script>

<template>
  <div class="lumaestro-app">

    <!-- Barra Lateral -->
    <aside class="sidebar glass">
      <div class="logo">LM</div>
      <nav>
        <button @click="currentView = 'orchestrator'" :class="{ active: currentView === 'orchestrator' }" title="Cérebro & Grafo">🧠</button>
        <button @click="currentView = 'swarm'" :class="{ active: currentView === 'swarm' }" title="Painel de Comando Executivo">🏛️</button>
        <button @click="currentView = 'repos'" :class="{ active: currentView === 'repos' }" title="Gerenciar Repositórios (RAG Radial)">📂</button>
        
        <div class="sidebar-divider"></div>
        
        <button @click="openDoc('tasks', 'Checklist de Tarefas')" title="Tarefas e Progresso">📋</button>
        <button @click="openDoc('implementation', 'Plano de Implementação')" title="Arquitetura do Sistema">📐</button>
        <button @click="openDoc('walkthrough', 'Guia de Uso')" title="Manual de Operação">📖</button>
        
        <div class="sidebar-divider"></div>
        
        <button @click="currentView = 'settings'" :class="{ active: currentView === 'settings' }" title="Configurações">⚙️</button>
      </nav>
      <!-- Indicador de Status -->
      <div class="status-indicator" :title="connectionError">
         <div class="dot" :class="{ online: isOnline }"></div>
      </div>
    </aside>

    <main id="lumaestro-main" :class="{ 'is-orchestrator': currentView === 'orchestrator' }">
      <template v-if="currentView === 'orchestrator'">
        <div class="left-workspace">
          <div class="graph-area">
            <GraphVisualizer :nodes="state.nodes" :edges="state.edges" :graphLogs="state.graphLogs" :activeNode="state.activeNode" />
          </div>

          <div class="orchestrator-bottom-terminal" v-show="isTerminalDockOpen">
            <AgentTerminal :isOpen="isTerminalDockOpen" @close="isTerminalDockOpen = false" />
          </div>

          <!-- Puxador para reabrir o terminal quando escondido -->
          <div 
            v-if="!isTerminalDockOpen" 
            class="terminal-expand-handle glass"
            @click="isTerminalDockOpen = true"
            title="Mostrar Terminal de Atividade"
          >
            <span class="handle-icon">
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="3">
                <polyline points="18 15 12 9 6 15"></polyline>
              </svg>
            </span>
            <span class="handle-text">TERMINAL</span>
          </div>
        </div>

        <!-- Resize Handle (arrastável) -->
        <div 
          class="resize-handle"
          @mousedown="startResize"
          :class="{ 'is-dragging': isResizing }"
        >
          <div class="resize-grip">
            <span></span><span></span><span></span>
          </div>
        </div>

        <!-- Barra de Histórico (ACP Capable) - Retrátil -->
        <Transition name="slide">
          <HistorySidebar v-if="orchestrator.isSidebarOpen" />
        </Transition>

        <aside 
          class="glass chat-area" 
          :class="{ 'chat-minimized': isChatMinimized }"
          :style="isChatMinimized ? {} : { width: chatWidth + 'px', minWidth: chatWidth + 'px' }"
        >
          <ChatPanel :is-minimized="isChatMinimized" @toggle-minimize="toggleChat" />

          <!-- 🚀 Overlay de Boot — Diagnóstico Visual (Movido para o Chat) -->
          <Transition name="boot-fade">
            <div v-if="isBooting" class="boot-overlay">
              <div class="boot-card glass">
                <div class="boot-logo-ring">
                  <div class="boot-logo-pulse"></div>
                  <span class="boot-logo-text">LM</span>
                </div>
                <h2 class="boot-title">Maestro está acordando...</h2>
                <p class="boot-subtitle">Preparando os motores de inteligência artificial</p>

                <div class="boot-stages">
                  <TransitionGroup name="stage-list">
                    <div 
                      v-for="s in bootStages" 
                      :key="s.stage" 
                      class="boot-stage"
                      :class="{ done: s.done, active: !s.done }"
                    >
                      <span class="stage-icon">{{ s.icon }}</span>
                      <span class="stage-msg">{{ s.message }}</span>
                      <span class="stage-check" v-if="s.done">✓</span>
                      <span class="stage-spinner" v-else></span>
                    </div>
                  </TransitionGroup>
                </div>

                <div v-if="bootError" class="boot-error">
                  <span>🔴</span> {{ bootError }}
                </div>

                <p v-if="bootStages.length === 0" class="boot-waiting">
                  Aguardando sinal do backend...
                </p>
              </div>
            </div>
          </Transition>
        </aside>
      </template>

      <template v-else-if="currentView === 'swarm'">
        <SwarmDashboard />
      </template>

      <template v-else-if="currentView === 'settings'">
        <Settings />
      </template>

      <template v-else-if="currentView === 'repos'">
        <ReposManager />
      </template>
    </main>

    <!-- Visualizador de Inteligência do Projeto -->
    <DocViewer 
      :isOpen="state.docViewer.isOpen" 
      :title="state.docViewer.title" 
      :content="state.docViewer.content" 
      @close="state.docViewer.isOpen = false"
    />

    <!-- 🛡️ GLOBAL MAESTRO CONFIRM MODAL -->
    <MaestroConfirm 
      :isOpen="orchestrator.confirmModal.show"
      :title="orchestrator.confirmModal.title"
      :message="orchestrator.confirmModal.message"
      :type="orchestrator.confirmModal.type"
      :confirmText="orchestrator.confirmModal.confirmText"
      :cancelText="orchestrator.confirmModal.cancelText"
      @confirm="orchestrator.confirmModal.onConfirm"
      @cancel="orchestrator.confirmModal.onCancel"
    />

    <!-- 🌌 PREMIUM COSMOS TOAST NOTIFICATION -->
    <transition name="maestro-toast">
      <div v-if="settingsStore.toast.show" 
           class="premium-toast" 
           :class="'toast-' + settingsStore.toast.type"
           @click="settingsStore.toast.show = false">
        <div class="toast-glow"></div>
        <div class="toast-icon-wrapper">
           <div class="icon-pulse"></div>
           <span class="icon-glyph" v-if="settingsStore.toast.type === 'success'">💎</span>
           <span class="icon-glyph" v-else-if="settingsStore.toast.type === 'error'">⚠️</span>
           <span class="icon-glyph" v-else>💠</span>
        </div>
        <div class="toast-body">
          <div class="toast-header">
            <span class="toast-label">{{ settingsStore.toast.type === 'success' ? 'Sincronia Completa' : settingsStore.toast.type === 'error' ? 'Alerta de Célula' : 'Pulso de Dados' }}</span>
            <span class="toast-system-tag">COSMOS CORE</span>
          </div>
          <div class="toast-text">{{ settingsStore.toast.message }}</div>
        </div>
        <div class="toast-progress-container">
           <div class="toast-progress-bar" :style="{ animationDuration: (settingsStore.toast.duration || 4000) + 'ms' }"></div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.lumaestro-app {
  display: flex;
  width: 100vw;
  height: 100vh;
  background: #0d1117;
  color: white;
  overflow: hidden;
}

.sidebar {
  width: 60px;
  background: rgba(255, 255, 255, 0.02);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 20px 0;
  border-right: 1px solid rgba(255, 255, 255, 0.05);
  justify-content: space-between;
}

.logo {
  font-weight: bold;
  color: #4facfe;
  font-size: 1.2rem;
}

nav button {
  background: transparent;
  border: none;
  font-size: 1.4rem;
  color: rgba(255, 255, 255, 0.3);
  margin-bottom: 30px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  padding: 8px;
  border-radius: 10px;
  position: relative;
}

nav button:hover {
  background: rgba(59, 130, 246, 0.08);
  color: rgba(255, 255, 255, 0.6);
}

nav button.active {
  color: white;
  background: rgba(59, 130, 246, 0.12);
  transform: scale(1.05);
  box-shadow: inset 0 0 12px rgba(79, 172, 254, 0.15);
}

.sidebar-divider {
  width: 20px;
  height: 1px;
  background: rgba(255, 255, 255, 0.05);
  margin: 10px 0;
}

.status-indicator {
  margin-bottom: 20px;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #ff5252;
  box-shadow: 0 0 8px #ff5252;
  transition: all 0.5s;
}

.dot.online {
  background: #00e676;
  box-shadow: 0 0 10px #00e676;
}

#lumaestro-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow-y: auto; /* Habilita scroll para views como Settings */
}

#lumaestro-main.is-orchestrator {
  display: flex;
  flex-direction: row;
  overflow: hidden; /* Mantém o layout fixo */
}

.left-workspace {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  overflow: hidden;
}

.orchestrator-bottom-terminal {
  height: 220px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  transition: all 0.3s ease;
}

.terminal-expand-handle {
  height: 28px;
  background: rgba(13, 17, 23, 0.8) !important;
  border-top: 1px solid rgba(59, 130, 246, 0.2);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s;
  user-select: none;
  z-index: 100;
}

.terminal-expand-handle:hover {
  background: rgba(59, 130, 246, 0.1) !important;
  height: 32px;
}

.handle-icon {
  color: #3b82f6;
  display: flex;
  animation: bounce-up 2s infinite;
}

.handle-text {
  font-size: 10px;
  font-weight: 900;
  letter-spacing: 2px;
  color: #64748b;
}

@keyframes bounce-up {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-2px); }
}

.graph-area {
  flex: 1;
  position: relative;
  overflow: hidden;
  min-width: 0;
}

.chat-area {
  margin: 10px 10px 0 10px;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  transition: width 0.35s cubic-bezier(0.4, 0, 0.2, 1), min-width 0.35s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative; /* Necessário para conter o overlay de boot */
  overflow: hidden; /* Mantém o overlay dentro dos cantos arredondados */
}

.chat-area.chat-minimized {
  width: 52px !important;
  min-width: 52px !important;
  margin: 10px 6px 0 6px;
}

/* ── Resize Handle ── */
.resize-handle {
  width: 8px;
  cursor: col-resize;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  z-index: 10;
  flex-shrink: 0;
  transition: background 0.2s;
}

.resize-handle:hover,
.resize-handle.is-dragging {
  background: rgba(59, 130, 246, 0.1);
}

.resize-handle::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 2px;
  background: rgba(255, 255, 255, 0.06);
  transition: all 0.3s;
}

.resize-handle:hover::before,
.resize-handle.is-dragging::before {
  width: 3px;
  background: var(--primary);
  box-shadow: 0 0 8px var(--primary-glow);
}

.resize-grip {
  display: flex;
  flex-direction: column;
  gap: 3px;
  opacity: 0;
  transition: opacity 0.2s;
}

.resize-handle:hover .resize-grip,
.resize-handle.is-dragging .resize-grip {
  opacity: 1;
}

.resize-grip span {
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--primary);
}

/* ── Transição Sidebar (Modo Gaveta) ── */
.slide-enter-active,
.slide-leave-active {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  white-space: nowrap;
}

.slide-enter-from,
.slide-leave-to {
  width: 0 !important;
  opacity: 0;
  margin: 0 !important;
  transform: translateX(-40px);
}

/* ═══════════════════════════════════════════ */
/*  BOOT OVERLAY — Diagnóstico Visual Premium  */
/* ═══════════════════════════════════════════ */
.boot-overlay {
  position: absolute;
  inset: 0;
  z-index: 900; /* Abaixo do z-index global, mas acima do chat local */
  display: flex;
  align-items: center;
  justify-content: center;
  background: radial-gradient(ellipse at center, rgba(13, 17, 23, 0.97) 0%, rgba(13, 17, 23, 0.90) 100%);
  backdrop-filter: blur(8px);
  border-radius: inherit; /* Segue o arredondamento do chat-area */
}

.boot-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1.2rem;
  padding: 2.5rem 2rem;
  border-radius: 20px;
  border: 1px solid rgba(79, 172, 254, 0.1);
  background: rgba(255, 255, 255, 0.02);
  min-width: 380px;
  max-width: 450px;
  box-shadow:
    0 0 50px rgba(79, 172, 254, 0.05),
    0 15px 45px rgba(0, 0, 0, 0.3);
  transform: scale(0.95);
}

.boot-logo-ring {
  position: relative;
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.boot-logo-pulse {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  border: 2px solid rgba(79, 172, 254, 0.3);
  animation: boot-ring-spin 2s linear infinite;
  border-top-color: #4facfe;
}

@keyframes boot-ring-spin {
  to { transform: rotate(360deg); }
}

.boot-logo-text {
  font-size: 1.8rem;
  font-weight: 900;
  background: linear-gradient(135deg, #4facfe, #00f2fe);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  letter-spacing: 2px;
}

@keyframes boot-ring-spin {
  to { transform: rotate(360deg); }
}

.boot-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: rgba(255, 255, 255, 0.9);
  margin: 0;
  letter-spacing: 0.5px;
}

.boot-subtitle {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.35);
  margin: -0.5rem 0 0.5rem;
  letter-spacing: 0.5px;
}

.boot-stages {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.boot-stage {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 0.78rem;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.04);
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.boot-stage.active {
  border-color: rgba(79, 172, 254, 0.2);
  background: rgba(79, 172, 254, 0.04);
}

.boot-stage.done {
  opacity: 0.5;
}

.stage-icon {
  font-size: 1rem;
  flex-shrink: 0;
}

.stage-msg {
  flex: 1;
  color: rgba(255, 255, 255, 0.7);
}

.stage-check {
  color: #00e676;
  font-weight: bold;
  font-size: 0.85rem;
}

.stage-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid rgba(79, 172, 254, 0.2);
  border-top-color: #4facfe;
  border-radius: 50%;
  animation: boot-ring-spin 0.8s linear infinite;
  flex-shrink: 0;
}

.boot-error {
  width: 100%;
  padding: 10px 14px;
  border-radius: 12px;
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #fca5a5;
  font-size: 0.75rem;
}

.boot-waiting {
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.25);
  animation: pulse-op 1.5s infinite;
}

/* Transições */
.boot-fade-enter-active { transition: opacity 0.3s ease; }
.boot-fade-leave-active { transition: opacity 0.8s ease; }
.boot-fade-enter-from,
.boot-fade-leave-to { opacity: 0; }

.stage-list-enter-active { transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1); }
.stage-list-enter-from { opacity: 0; transform: translateY(10px); }

/* ═══════════════════════════════════════════ */
/*   🌌 PREMIUM COSMOS TOAST — Visual DNA    */
/* ═══════════════════════════════════════════ */
.premium-toast {
  position: fixed;
  top: 32px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  min-width: 380px;
  max-width: 500px;
  background: rgba(13, 17, 23, 0.8);
  backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 20px;
  padding: 16px 20px;
  display: flex;
  align-items: center;
  gap: 18px;
  cursor: pointer;
  box-shadow: 
    0 20px 40px rgba(0, 0, 0, 0.4),
    inset 0 0 0 1px rgba(255, 255, 255, 0.05);
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.premium-toast:hover {
  transform: translateX(-50%) translateY(-2px);
  background: rgba(13, 17, 23, 0.9);
  border-color: rgba(255, 255, 255, 0.2);
}

.toast-glow {
  position: absolute;
  top: -50%;
  left: -20%;
  width: 100px;
  height: 200px;
  background: radial-gradient(circle, var(--toast-color, #3b82f6) 0%, transparent 70%);
  opacity: 0.15;
  filter: blur(30px);
  pointer-events: none;
}

.toast-success { --toast-color: #3b82f6; border-color: rgba(59, 130, 246, 0.3); }
.toast-error { --toast-color: #ef4444; border-color: rgba(239, 68, 68, 0.3); }
.toast-info { --toast-color: #8b5cf6; border-color: rgba(139, 92, 246, 0.3); }

.toast-icon-wrapper {
  position: relative;
  width: 48px;
  height: 48px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-pulse {
  position: absolute;
  inset: -4px;
  border: 2px solid var(--toast-color);
  border-radius: 16px;
  opacity: 0;
  animation: icon-pulse-anim 2s infinite;
}

@keyframes icon-pulse-anim {
  0% { transform: scale(0.9); opacity: 0.5; }
  100% { transform: scale(1.3); opacity: 0; }
}

.icon-glyph {
  font-size: 1.4rem;
  filter: drop-shadow(0 0 8px var(--toast-color));
}

.toast-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toast-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.toast-label {
  font-size: 0.65rem;
  font-weight: 900;
  text-transform: uppercase;
  letter-spacing: 2px;
  color: var(--toast-color);
}

.toast-system-tag {
  font-size: 0.55rem;
  font-weight: 800;
  color: rgba(255, 255, 255, 0.2);
  letter-spacing: 1px;
}

.toast-text {
  font-size: 0.9rem;
  font-weight: 600;
  color: #fff;
  line-height: 1.4;
}

.toast-progress-container {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: rgba(255, 255, 255, 0.05);
}

.toast-progress-bar {
  height: 100%;
  background: linear-gradient(to right, transparent, var(--toast-color));
  box-shadow: 0 0 10px var(--toast-color);
  width: 100%;
  animation: toast-progress linear forwards;
}

@keyframes toast-progress {
  from { width: 100%; }
  to { width: 0%; }
}

/* 🌀 Animação de Entrada Cinematográfica */
.maestro-toast-enter-active {
  animation: toast-orbit-in 0.6s cubic-bezier(0.34, 1.56, 0.64, 1);
}

.maestro-toast-leave-active {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}

.maestro-toast-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(-20px) scale(0.9);
}

@keyframes toast-orbit-in {
  0% { opacity: 0; transform: translateX(-50%) translateY(-100px) scale(0.8); }
  100% { opacity: 1; transform: translateX(-50%) translateY(0) scale(1); }
}
</style>
