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
    currentViewState, savePositions, currentNodes, focusNodeById 
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
    
    // 🚀 [CONEXÃO VITAL] Registra a interface do grafo na Store para controle externo (Efeito de Descoberta)
    Object.assign(store.graphInstance, { 
        focusNodeById,
        pan: (dx, dy, dz) => focusNodeById(null)
    })

    // 2. Sincronizar banco local na partida
    syncAllOnStartup()

    // 3. Registrar Listeners (Eventos Wails e Teclado)
    cleanupEvents = registerGraphEvents({ 
        updateGraph, 
        focusNode: (id) => focusNodeById(id)
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

  // W3: ✨ EFEITO DE DESCOBERTA — Quando o RAG identifica um neurônio relevante,
  // o backend emite node:active → App.vue atualiza state.activeNode → 
  // esta prop muda → fazemos zoom cinematográfico + abrimos detalhes.
  watch(() => props.activeNode, (newNodeId, oldNodeId) => {
    if (!newNodeId || newNodeId === oldNodeId) return
    
    // O BridgeDriver já cuida do Efeito de Descoberta via evento direto.
    // Este watcher serve como rede de segurança para garantir a sincronização.
    setTimeout(() => {
      // 🛡️ Normalização de IDs: Previne falso re-trigger por diferenças de case/formato
      const normalizedNew = String(newNodeId).toLowerCase().trim()
      const normalizedCurrent = String(store.selectedNode?.id || '').toLowerCase().trim()
      
      // Só reforça se o BridgeDriver NÃO conseguiu resolver (status !== 'found')
      if (normalizedCurrent !== normalizedNew && store.discoveryStatus !== 'found') {
        console.log(`[Watcher] 🔍 Reforçando Efeito de Descoberta para: ${newNodeId}`)
        focusNodeById(newNodeId)
      }
    }, 800) // Aumentado de 600ms para 800ms para dar mais tempo ao BridgeDriver
  })

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
      }, 150) // Reduzido de 450ms para 150ms para suportar o Efeito de Descoberta em tempo real
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
    updateForce,
    currentViewState
  }
}

