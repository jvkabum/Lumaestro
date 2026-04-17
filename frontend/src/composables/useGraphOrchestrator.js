import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useGraphStore } from '../stores/graph'
import { useDeckRender } from './deck/useDeckRender'
import { useGraphData } from './useGraphData'
import { useGraphControls } from './useGraphControls'
import { useGraphEvents } from './useGraphEvents'
import { useGraphSync } from './useGraphSync'

/**
 * 🎼 useGraphOrchestrator — O Maestro do Ciclo de Vida do Grafo
 * 
 * Centraliza a orquestração entre Deck.gl, Sincronização e UI.
 * Este arquivo contém toda a lógica comportamental extraída do GraphVisualizer.vue.
 */
export function useGraphOrchestrator(props) {
  const store = useGraphStore()
  
  // ── Dom Refs ──
  const containerRef = ref(null)
  const logContainerRef = ref(null)
  const isUiMinimized = ref(false)

  // ── Importações de Sub-Lógicas ──
  const { initGraph, updateGraph, destroyGraph, currentViewState } = useDeckRender()
  const { getGraphData } = useGraphData()
  const { registerKeyboardControls } = useGraphControls()
  const { registerGraphEvents } = useGraphEvents()
  const { syncAllOnStartup } = useGraphSync()

  // ── Cleanup ──
  let cleanupKeyboard = null
  let cleanupEvents = null

  // ── Ciclo de Vida ──
  onMounted(async () => {
    await nextTick()
    if (!containerRef.value) return

    // 1. Inicializar o Deck.gl via useDeckRender
    initGraph(containerRef.value, props.nodes, props.edges, props.activeNode)

    // 2. Sincronizar banco local na partida
    syncAllOnStartup()

    // 3. Registrar Listeners (Eventos Wails e Teclado)
    cleanupEvents = registerGraphEvents(containerRef)
    cleanupKeyboard = registerKeyboardControls(currentViewState)
  })

  onUnmounted(() => {
    if (cleanupKeyboard) cleanupKeyboard()
    if (cleanupEvents) cleanupEvents()
    destroyGraph()
  })

  // ── Watchers Críticos ──

  // W4: Vigia dados de backend + Filtros UI (X-Ray, Esqueleto) com DEBOUNCE Híbrido
  let renderTimeout = null
  watch(() => [props.nodes, props.edges, store.xRayThreshold, store.skeletalMode], () => {
    if (containerRef.value && props.nodes.length > 0) {
      clearTimeout(renderTimeout)
      renderTimeout = setTimeout(() => {
        // 1. Passa pela lógica preciosa de X-Ray / Esqueleto / Nós Virtuais
        const { nodes: filteredNodes, links: filteredEdges } = getGraphData(props.nodes, props.edges)
        
        // 2. Sincronização Incremental (Zero Jitter) em vez de rebuild total
        updateGraph(filteredNodes, filteredEdges)
      }, 450) // Agrupa rajadas de websocket E cliques rápidos no Slider de X-Ray
    }
  }, { deep: true })

  // W5: Auto-scroll dos logs
  watch(() => props.graphLogs, () => {
    nextTick(() => {
      if (logContainerRef.value) {
        logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
      }
    })
  }, { deep: true })

  return {
    containerRef,
    logContainerRef,
    isUiMinimized,
    currentViewState
  }
}
