<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import * as THREE from 'three'
import { useGraphStore } from '../stores/graph'
import { useOrchestratorStore } from '../stores/orchestrator'
import { useGraphSetup } from '../composables/useGraphSetup'
import { useGraphData } from '../composables/useGraphData'
import { useGraphControls } from '../composables/useGraphControls'
import { useGraphEvents } from '../composables/useGraphEvents'
import { useGraphSync } from '../composables/useGraphSync'
import { useGraphXRay } from '../composables/useGraphXRay'

// ── Stores ──
const store = useGraphStore()
const orchestrator = useOrchestratorStore()
const isUiMinimized = ref(false)

// ── Composables ──
const { initGraph, focusNode } = useGraphSetup()
const { getGraphData } = useGraphData()
const { registerKeyboardControls } = useGraphControls()
const { registerGraphEvents, resolveConflict } = useGraphEvents()
const { handleFastSync, handleFullSync, confirmSync } = useGraphSync()
const { syncAllOnStartup } = useGraphSync()
const { runReconScan, pruneNodes } = useGraphXRay()

// ── Props (mantidos para compatibilidade com App.vue) ──
const props = defineProps({
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] },
  graphLogs: { type: Array, default: () => [] },
  activeNode: { type: String, default: null } 
})

// ── Refs de DOM ──
const containerRef = ref(null)
const logContainerRef = ref(null)

// ── Cleanup references ──
let cleanupKeyboard = null
let cleanupEvents = null

// ── Lifecycle ──
onMounted(async () => {
  await nextTick()
  // 1. Inicializar o grafo 3D
  initGraph(containerRef.value, props.nodes, props.edges, props.activeNode)

  // 2. Sincronizar todos os nós do banco na partida
  syncAllOnStartup()

  // 3. Registrar eventos do Wails
  cleanupEvents = registerGraphEvents(containerRef)

  // 4. Registrar controles de teclado (WASD)
  cleanupKeyboard = registerKeyboardControls()
})

onUnmounted(() => {
  // Limpeza de listeners
  if (cleanupKeyboard) cleanupKeyboard()
  if (cleanupEvents) cleanupEvents()
  
  // Limpeza de memória do 3d-force-graph
  if (store.graphInstance) {
    store.graphInstance._destructor()
    store.graphInstance = null
  }
})

// ── Watchers (Sincronização Reativa) ──

watch(() => [props.nodes, props.edges], () => {
  if (store.graphInstance) {
    console.log(`[NeuralGraph] Dados Recebidos: ${props.nodes.length} nós, ${props.edges.length} arestas.`)
    store.graphInstance.graphData(getGraphData(props.nodes, props.edges))
  }
})

// W2: Fly-to no nó ativo + abertura de detalhes
watch(() => props.activeNode, (newId) => {
  if (!store.graphInstance || !newId) return

  const node = store.graphInstance.graphData().nodes.find(n => n.id === newId)
  if (node) {
    // 🎯 Foco completo: Zoom + Detalhes + Brilho
    focusNode(node)
    
    // Luz de pulso adicional para destaque extra no 3D
    const light = new THREE.PointLight(0xfcd34d, 2.5, 120)
    light.position.set(node.x, node.y, node.z)
    store.graphInstance.scene().add(light)
    setTimeout(() => { store.graphInstance.scene().remove(light) }, 3000)
  }
})

// W3: X-Ray reativo
watch(() => store.xRayThreshold, () => {
  if (store.graphInstance) {
    store.graphInstance.graphData(getGraphData(props.nodes, props.edges))
  }
})

// W4: Modo esqueletal (MST)
watch(() => store.skeletalMode, () => {
  if (store.graphInstance) {
    store.graphInstance.graphData(getGraphData(props.nodes, props.edges))
  }
})

// W5: Auto-scroll dos logs
watch(() => props.graphLogs, () => {
  nextTick(() => {
    if (logContainerRef.value) {
      logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
    }
  })
}, { deep: true })
</script>

<template>
  <div class="graph-wrapper animate-fade-in">
    <!-- MODAL DE CONFIRMAÇÃO DINÂMICO -->
    <div v-if="store.showConfirmModal" class="premium-modal-overlay">
      <div class="premium-modal-content">
        <div class="modal-icon">{{ store.modalMode === 'full' ? '⚙️' : '🚀' }}</div>
        <h3 class="modal-title">{{ store.modalMode === 'full' ? 'Reindexação Forçada' : 'Sincronização Inteligente' }}</h3>
        
        <div class="modal-body">
          <p v-if="store.modalMode === 'full'" class="modal-text">
            Deseja forçar uma varredura completa de todos os <strong>{{ store.graphHealth.active_nodes || nodes.length }} arquivos</strong>?<br/>
            <span class="warning-sub">Isso reconstrói o cache de auditoria e garante 100% de integridade. Use apenas se notar dados faltando.</span>
          </p>
          <p v-else class="modal-text">
            Deseja iniciar a sincronização incremental?<br/>
            <span class="info-sub">O Maestro buscará apenas notas <strong>novas ou modificadas</strong>. É o método mais rápido e econômico.</span>
          </p>
        </div>

        <div class="modal-actions">
           <button @click="store.showConfirmModal = false" class="btn-cancel">CANCELAR</button>
           <button @click="confirmSync" class="btn-confirm" :class="store.modalMode">
             {{ store.modalMode === 'full' ? 'INICIAR FAXINA' : 'SINCRONIZAR AGORA' }}
           </button>
        </div>
      </div>
    </div>

    <!-- Container para o Grafo 3D (WebGL) -->
    <div ref="containerRef" class="main-canvas"></div>

    <!-- 📊 FPS MONITOR (F1 para toggle) -->
    <Transition name="fade">
      <div v-if="store.showFps" class="fps-hud">
        <span class="fps-label">FPS</span>
        <span class="fps-value" :class="{
          'fps-good': store.currentFps >= 50,
          'fps-warn': store.currentFps >= 20 && store.currentFps < 50,
          'fps-bad': store.currentFps < 20
        }">{{ store.currentFps }}</span>
      </div>
    </Transition>
    
    <!-- Controles & Console de Logs (Painel de Pensamento Vidrado) -->
    <div class="graph-ui glass">
      <div class="ui-header" @click="isUiMinimized = !isUiMinimized" style="cursor: pointer;">
        <span class="pulse" :class="{ 'ai-active': orchestrator.isNavigating }"></span>
        <h3>Conhecimento Obsidian 3D</h3>
        <span v-if="orchestrator.isNavigating" class="ai-status-label animate-pulse">IA RACIOCINANDO...</span>
        <button class="minimize-btn" :class="{ rotated: isUiMinimized }">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2.5">
            <path d="M6 9l6 6 6-6" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
      </div>
      
      <Transition name="collapse">
        <div v-if="!isUiMinimized" class="ui-content-wrapper">
          <div class="ui-actions">
            <div class="sync-controls">
              <button @click="handleFastSync" class="action-btn main-sync" :class="{'scanning-btn': store.scanning}" title="Sincronização Rápida">
                <span v-if="!store.scanning">🚀</span><span v-else class="spin">⏳</span>
                <span>SINCRONIZAR</span>
              </button>
              <button @click="handleFullSync" class="action-btn icon-only-btn" :class="{'scanning-btn': store.scanning}" title="Sincronização Total">
                <span>⚙️</span>
              </button>
            </div>
            <div class="stat-item">
              <span class="val">{{ store.graphHealth.active_nodes || nodes.length }}</span>
              <span class="lab">NOTAS</span>
            </div>
          </div>

      <!-- 🩻 CONTROLES X-RAY & RECON -->
      <div class="xray-panel glass">
        <div class="xray-header">
          <span class="xray-icon">🩻</span>
          <span>MODO X-RAY</span>
          <span class="xray-val">{{ (store.xRayThreshold * 100).toFixed(0) }}</span>
        </div>
        <input type="range" min="0" max="1" step="0.01" v-model.number="store.xRayThreshold" class="xray-slider" />
        
        <div class="recon-actions">
           <button @click="runReconScan" class="recon-btn" :disabled="store.scanLoading" title="Scan Proativo">
             <span v-if="!store.scanLoading">🕵️ RECON</span>
             <span v-else class="spin">⏳</span>
           </button>
           <button @click="pruneNodes" class="prune-btn" :disabled="store.pruneLoading" title="Poda Neural">
             <span v-if="!store.pruneLoading">🧹 PODA</span>
             <span v-else class="spin">⏳</span>
           </button>
           <button @click="store.skeletalMode = !store.skeletalMode" :class="['recon-btn', { active: store.skeletalMode }]" title="Modo Esqueleto (MST)">
             <span v-if="!store.skeletalMode">🩻 MST</span>
             <span v-else>👁️ FULL</span>
           </button>
        </div>
      </div>

      <!-- HUD DE SAÚDE DO GRAFO (HEALTH MONITOR) -->
      <div class="graph-health-hud">
        <div class="health-info">
          <div class="health-stat">
            <span class="label">DENSIDADE</span>
            <span class="value">{{ (store.graphHealth.density * 100).toFixed(0) }}%</span>
          </div>
          <div class="health-stat" :class="{'has-conflicts': store.graphHealth.conflicts > 0}">
            <span class="label">CONFLITOS</span>
            <span class="value">{{ store.graphHealth.conflicts }}</span>
          </div>
        </div>
        <div class="hud-actions" style="display: flex; gap: 8px;">
          <button @click="store.graphInstance.zoomToFit(800, 150)" class="health-btn" title="Resetar Câmera">🎯 RECENTRAR</button>
          <button @click="store.checkHealth" class="health-btn" title="Analisar Integridade">🛡️ CHECK</button>
        </div>
      </div>

      <!-- O CONSOLE VIVO DO RACIOCÍNIO IA -->
      <div class="graph-logs-console" ref="logContainerRef" v-if="graphLogs.length > 0">
        <div v-for="(log, idx) in graphLogs" :key="idx" class="log-entry">
          <span class="log-text">{{ log }}</span>
        </div>
      </div>
        </div> <!-- Fim da ui-content-wrapper -->
      </Transition>
    </div> <!-- Fim da graph-ui -->

    <!-- POP-UP DE VALIDAÇÃO (AGENTE DA VERDADE) -->
    <div v-if="store.currentConflict" class="conflict-overlay">
      <div class="conflict-modal glass">
        <div class="conflict-header">
          <span class="alert-icon">⚠️</span>
          <h4>Contradição Semântica</h4>
        </div>
        <p>A IA detectou uma divergência sobre <b>{{ store.currentConflict.subject }}</b>:</p>
        <div class="conflict-options">
          <div class="opt old" @click="resolveConflict('old')">
            <span class="lab">PASSADO</span>
            <span class="val">{{ store.currentConflict.old }}</span>
          </div>
          <div class="opt new" @click="resolveConflict('new')">
            <span class="lab">PRESENTE</span>
            <span class="val">{{ store.currentConflict.new }}</span>
          </div>
        </div>
        <p class="hint">Escolha a verdade ativa. A outra será marcada como legado.</p>
      </div>
    </div>

    <!-- PAINEL DE PROVENIÊNCIA (AUDITORIA) -->
    <transition name="slide-fade">
      <aside v-if="store.selectedNode" class="provenance-panel glass">
        <header class="panel-header">
          <div class="header-content">
            <div class="source-icon">🔎</div>
            <h3>Proveniência</h3>
          </div>
          <button @click="store.closeDetails" class="close-btn">×</button>
        </header>

        <div class="panel-body">
          <!-- Estado: Carregando -->
          <div v-if="!store.nodeDetails || store.nodeDetails.loading" class="loading-provenance">
            <div class="spinner"></div>
            <span>Sintonizando Base...</span>
          </div>

          <!-- Estado: Sucesso ou Erro (com conteúdo) -->
          <div v-else class="details-content">
            <div class="provenance-metadata">
              <div class="meta-item">
                <span class="lab">DOCUMENTO ORIGEM</span>
                <div class="val-box">{{ store.nodeDetails?.path || 'Escaneando...' }}</div>
              </div>
              
              <div class="meta-item">
                <span class="lab">TRECHO FUNDAMENTADO (CHUNK)</span>
                <div class="content-box glass">
                   {{ store.nodeDetails?.content || 'Aguardando recuperação semântica...' }}
                </div>
              </div>
            </div>

            <button v-if="store.nodeDetails && store.nodeDetails.path && !store.nodeDetails.isVirtual && store.nodeDetails.path !== 'Conceito Neural'" 
                    @click="store.openSource" class="open-btn premium-btn">
              ABRIR ARQUIVO FONTE ✨
            </button>
          </div>
        </div>
      </aside>
    </transition>

    <!-- Background Imersivo -->
    <div class="graph-bg"></div>
  </div>
</template>

<style scoped src="../assets/css/GraphVisualizer.css"></style>
