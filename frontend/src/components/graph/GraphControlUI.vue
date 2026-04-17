<script setup>
import { useGraphStore } from '../../stores/graph'
import { useOrchestratorStore } from '../../stores/orchestrator'
import { useGraphActions } from '../../composables/deck/useGraphActions'

/**
 * 🎛️ GraphControlUI — O Centro de Comando do Grafo
 * 
 * Responsável por:
 * - Sincronização (Rápida e Total)
 * - Controles X-Ray e Recon
 * - HUD de Saúde do Grafo
 * - Console de Logs da IA
 */
const props = defineProps({
  nodesCount: { type: Number, default: 0 },
  graphLogs: { type: Array, default: () => [] },
  isUiMinimized: { type: Boolean, default: false },
  logContainerRef: { type: Object, default: null }
})

const emit = defineEmits(['toggle-minimize', 'update-log-ref'])

const store = useGraphStore()
const orchestrator = useOrchestratorStore()
const { 
  handleFastSync, 
  handleFullSync, 
  runReconScan, 
  pruneNodes 
} = useGraphActions()
</script>

<template>
  <div class="graph-ui glass">
    <!-- CABEÇALHO DO PAINEL -->
    <div class="ui-header" @click="emit('toggle-minimize')" style="cursor: pointer;">
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
        <!-- AÇÕES DE SINCRONIZAÇÃO -->
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
            <span class="val">{{ store.graphHealth.active_nodes || nodesCount }}</span>
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
        <div class="graph-logs-console" :ref="el => $emit('update-log-ref', el)" v-if="graphLogs.length > 0">
          <div v-for="(log, idx) in graphLogs" :key="idx" class="log-entry">
            <span class="log-text">{{ log }}</span>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>
