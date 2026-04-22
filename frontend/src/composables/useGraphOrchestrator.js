import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useGraphStore } from '../stores/graph'
import { useDeckRender } from './deck/useDeckRender'
import { useDataTransformer } from './deck/internal/DataTransformer'
import { useInputDriver } from './deck/engine/InputDriver'
import { useBridgeDriver } from './deck/engine/BridgeDriver'
import { useSyncManager } from './deck/internal/SyncManager'
// O XRayProcessor agora é consumido apenas via useGraphActions.js para UI

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

  // ── Importações de Sub-Lógicas (Domínio Deck) ──
  const { 
    initGraph, updateGraph, destroyGraph, updateForce, 
    currentViewState, savePositions, currentNodes 
  } = useDeckRender()
  const { transform } = useDataTransformer()
  const { registerKeyboardControls } = useInputDriver()
  const { registerGraphEvents } = useBridgeDriver()
  const { syncAllOnStartup } = useSyncManager()

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
    cleanupEvents = registerGraphEvents({ 
        updateGraph, 
        focusNode: (id) => store.graphInstance?.focusNode(id) // Ponte para o Pilot via Store Contract
    })
    cleanupKeyboard = registerKeyboardControls(currentViewState, (dx, dy, dz) => {
        store.graphInstance?.panTarget(dx, dy, dz)
    })
  })

  onUnmounted(() => {
    if (cleanupKeyboard) cleanupKeyboard()
    if (cleanupEvents) cleanupEvents()
    
    // 💾 Última chamada de salvamento antes de desmontar (v20)
    if (currentNodes.value && currentNodes.value.length > 0) {
        const finalPositions = currentNodes.value.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z }));
        savePositions(finalPositions);
    }

    destroyGraph()
    window.removeEventListener('beforeunload', handleBeforeUnload)
  })

  // 🏁 Salvamento de Emergência (v20.2)
  const handleBeforeUnload = () => {
    if (currentNodes.value && currentNodes.value.length > 0) {
        const finalPositions = currentNodes.value.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z }));
        // Como o app está fechando, usamos uma chamada síncrona ou fire-and-forget
        window.go.core.App.UpdateNodePositions(finalPositions);
    }
  }

  window.addEventListener('beforeunload', handleBeforeUnload)

  // ── Watchers Críticos ──

  // W4: Vigia dados de backend + Filtros UI (X-Ray, Esqueleto) com DEBOUNCE Híbrido
  let renderTimeout = null
  watch(() => [props.nodes, props.edges, store.xRayThreshold, store.skeletalMode], () => {
    if (containerRef.value && props.nodes.length > 0) {
      clearTimeout(renderTimeout)
      renderTimeout = setTimeout(() => {
        // 1. Passa pela lógica preciosa de X-Ray / Esqueleto / Nós Virtuais
        const { nodes: filteredNodes, links: filteredEdges } = transform(props.nodes, props.edges)
        
        // 2. Sincronização Incremental (Zero Jitter) em vez de rebuild total
        updateGraph(filteredNodes, filteredEdges)
      }, 450) // Agrupa rajadas de websocket E cliques rápidos no Slider de X-Ray
    }
  }, { deep: true })

  // W6: Foco em Nó Ativo (Zoom reativo via Props)
  watch(() => props.activeNode, (newId) => {
    if (newId) {
      store.graphInstance?.focusNode(newId)
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

  return {
    containerRef,
    logContainerRef,
    isUiMinimized,
    updateForce,
    currentViewState
  }
}
