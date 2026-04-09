<script setup>
import { onMounted, reactive, ref } from 'vue'
import { CheckConnection } from '../wailsjs/go/core/App'
import { EventsOn } from '../wailsjs/runtime'
import ChatPanel from './components/ChatPanel.vue'
import GraphVisualizer from './components/GraphVisualizer.vue'
import HistorySidebar from './components/HistorySidebar.vue'
import Settings from './components/Settings.vue'
import DocViewer from './components/DocViewer.vue'
import SwarmDashboard from './components/SwarmDashboard.vue'
import AgentTerminal from './components/AgentTerminal.vue'
import { useOrchestratorStore } from './stores/orchestrator'
import { GetProjectDoc } from '../wailsjs/go/core/App'

const orchestrator = useOrchestratorStore()
const currentView = ref('orchestrator') // views: orchestrator, settings, swarm
const isOnline = ref(false)
const connectionError = ref('Aguardando sincronização com o Maestro (Frontend Booting)...')

// Estado de Boot — Diagnóstico Visual
const bootStages = ref([])
const isBooting = ref(true)
const bootError = ref(null)

// Painel redimensionável
const chatWidth = ref(500)
const isResizing = ref(false)
const minChatWidth = 500
const maxChatWidth = 1400

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
    // Puxa da esquerda para a direita → diminui chat
    // Puxa da direita para a esquerda → aumenta chat
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
  // 🧠 Escuta o Diagnóstico de Boot (cada estágio do backend)
  EventsOn('boot:stage', (data) => {
    if (data.stage === 'error') {
      bootError.value = data.message
      return
    }
    // Evita duplicatas no HMR
    if (!bootStages.value.find(s => s.stage === data.stage)) {
      bootStages.value.push({ ...data, done: false })
    }
    // Marca estágio anterior como concluído
    if (bootStages.value.length > 1) {
      bootStages.value[bootStages.value.length - 2].done = true
    }
    // Quando o backend reporta 'ready', fecha o overlay com animação
    if (data.stage === 'ready') {
      bootStages.value[bootStages.value.length - 1].done = true
      setTimeout(() => { isBooting.value = false }, 1500)
    }
  })

  // Verificar conexão inicial com diagnóstico visual
  try {
    isOnline.value = await CheckConnection()
    if (isOnline.value) {
      connectionError.value = "Maestro Online (Backend e Motor Vetorial Ativos)"
      // Se já estava online (HMR/reload), pula o overlay
      isBooting.value = false
    } else {
      connectionError.value = "Backend respondeu, mas Qdrant ou Configuração falharam. Verifique as configurações."
    }
  } catch(e) {
    isOnline.value = false
    connectionError.value = "Erro Wails IPC: Falha Crítica de Comunicação com o Backend Go. (" + String(e) + ")"
  }
  
  // Escuta troca de visualização remota (ex: vindo das Settings)
  EventsOn('view:change', (view) => {
    currentView.value = view
  })

  // Escuta os logs em tempo real
  EventsOn('agent:log', (log) => {
    const lastLog = state.logs[state.logs.length - 1]
    
    // Se for o Maestro e o último também for Maestro, anexa o texto (Streaming)
    if (log.source === 'MAESTRO' && lastLog && lastLog.source === 'MAESTRO') {
      lastLog.content += log.content
    } else {
      state.logs.push(log)
    }
  })

  // Escuta os dados do Grafo (Nodes e Edges) em lote (Batch Sync)
  EventsOn('graph:nodes:batch', (batchNodes) => {
    batchNodes.forEach(node => {
      if (!state.nodes.find(n => n.id === node.id)) {
        state.nodes.push(node)
      }
    })
  })

  EventsOn('graph:node', (node) => {
    if (!state.nodes.find(n => n.id === node.id)) {
      state.nodes.push(node)
    }
  })

  EventsOn('graph:edge', (edge) => {
    const s = edge.source.id || edge.source
    const t = edge.target.id || edge.target
    if (!state.edges.find(e => {
      const es = e.source.id || e.source
      const et = e.target.id || e.target
      return (es === s && et === t) || (es === t && et === s) // Evita duplicatas bidirecionais se redundante
    })) {
      state.edges.push(edge)
    }
  })

  EventsOn('graph:log', (glog) => {
    state.graphLogs.push(glog)
    if(state.graphLogs.length > 20) {
      state.graphLogs.shift() // Mantém o console UI leve (apenas os 20 últimos pensamentos)
    }
  })

  // Escuta saltos entre notas nas pesquisas
  EventsOn('node:active', (nodeId) => {
    state.activeNode = nodeId
  })
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
        
        <div class="sidebar-divider"></div>
        
        <button @click="openDoc('tasks', 'Checklist de Tarefas')" title="Tarefas e Progresso">📋</button>
        <button @click="openDoc('implementation', 'Plano de Implementação')" title="Arquitetura do Sistema">📐</button>
        <button @click="openDoc('walkthrough', 'Guia de Uso')" title="Manual de Operação">📖</button>
        
        <div class="sidebar-divider"></div>

        <button @click="isTerminalDockOpen = !isTerminalDockOpen" :class="{ active: isTerminalDockOpen && currentView === 'orchestrator' }" title="Terminal de Sincronia (Atividade do Agente)">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="display: block; margin: auto;">
            <polyline points="4 17 10 11 4 5"></polyline>
            <line x1="12" y1="19" x2="20" y2="19"></line>
          </svg>
        </button>
        
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

        <aside class="glass chat-area" :style="{ width: chatWidth + 'px', minWidth: chatWidth + 'px' }">
          <ChatPanel />

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
    </main>

    <!-- Visualizador de Inteligência do Projeto -->
    <DocViewer 
      :isOpen="state.docViewer.isOpen" 
      :title="state.docViewer.title" 
      :content="state.docViewer.content" 
      @close="state.docViewer.isOpen = false"
    />
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
  transition: none;
  position: relative; /* Necessário para conter o overlay de boot */
  overflow: hidden; /* Mantém o overlay dentro dos cantos arredondados */
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
</style>
