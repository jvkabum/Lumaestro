<script setup>
import { useGraphOrchestrator } from '../composables/useGraphOrchestrator'

// ── Sub-componentes Especializados ──
import GraphModals from './graph/GraphModals.vue'
import GraphControlUI from './graph/GraphControlUI.vue'
import GraphStatusHUD from './graph/GraphStatusHUD.vue'
import GraphProvenance from './graph/GraphProvenance.vue'

// ── Props ──
const props = defineProps({
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] },
  graphLogs: { type: Array, default: () => [] },
  activeNode: { type: String, default: null } 
})

// ── Orquestração Central (Logic extraction v10.0) ──
const { 
  containerRef, 
  logContainerRef, 
  isUiMinimized 
} = useGraphOrchestrator(props)

</script>

<template>
  <div class="graph-wrapper animate-fade-in">
    <!-- 1. Camada de Modais (Confirmação e Conflitos) -->
    <GraphModals :nodesCount="nodes.length" />

    <!-- 2. Motor Gráfico (Deck.gl Canvas) -->
    <div ref="containerRef" class="main-canvas"></div>

    <!-- 3. Camada de Status (FPS e Saúde do Grafo) -->
    <GraphStatusHUD />
    
    <!-- 4. Painel de Controle e Console de Logs -->
    <GraphControlUI 
      :nodesCount="nodes.length"
      :graphLogs="graphLogs"
      :isUiMinimized="isUiMinimized"
      @toggle-minimize="isUiMinimized = !isUiMinimized"
      @update-log-ref="el => logContainerRef = el"
    />

    <!-- 5. Painel de Detalhes (Anatomia do Nó) -->
    <GraphProvenance />

    <!-- 6. Background Atmosférico -->
    <div class="graph-bg"></div>
  </div>
</template>

<style src="../assets/css/GraphVisualizer.css"></style>
