import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'

/**
 * 🧠 GRAPH STORE — Núcleo Reativo do Grafo de Conhecimento 3D
 * 
 * Centraliza TODO o estado que antes vivia espalhado dentro do GraphVisualizer.vue.
 * Qualquer composable ou componente pode consumir e mutar esse estado via Pinia.
 */
export const useGraphStore = defineStore('graph', () => {
  // ── Estado de Dados ──
  const nodes = ref([])
  const edges = ref([])
  const graphLogs = ref([])
  const activeNode = ref(null)

  // ── Instância do 3d-force-graph (shallow para evitar deep proxy no THREE.js) ──
  const graphInstance = shallowRef(null)

  // ── Estado de Seleção & Proveniência ──
  const selectedNode = ref(null)
  const nodeDetails = ref(null)
  
  // ── Efeito de Descoberta (Discovery Status) ──
  const discoveryStatus = ref(null) // null | 'searching' | 'found' | 'failed'

  // ── Saúde do Grafo ──
  const graphHealth = ref({ density: 0, conflicts: 0, active_nodes: 0 })

  // ── Links Destacados (RAG Trail & Network Activation) ──
  const highlightedLinks = ref(new Set()) // Links globais (RAG)
  const clickedNodeLinks = ref(new Set()) // Links da rede do nó ativo
  const highlightedNeighbors = ref(new Set()) // Vizinhos do nó ativo

  // ── Actions ──
  const resetHighlights = () => {
    clickedNodeLinks.value = new Set()
    highlightedNeighbors.value = new Set()
  }
  const xRayThreshold = ref(0)
  const scanLoading = ref(false)
  const pruneLoading = ref(false)
  const skeletalMode = ref(false)

  // ── FPS Monitor (F1) ──
  const showFps = ref(false)
  const currentFps = ref(0)

  // ── Sincronização ──
  const scanning = ref(false)
  const showConfirmModal = ref(false)
  const modalMode = ref('fast') // 'fast' ou 'full'

  // ── Conflitos (Agente Validador) ──
  const currentConflict = ref(null)

  // ── Paleta de Cores Cibernéticas para Comunidades (Louvain) ──
  const communityPalette = [
    '#ff00ff', // Magenta (Cyber)
    '#00ff9f', // Verde Matrix
    '#00b8ff', // Azul Elétrico
    '#ff9500', // Laranja Nuclear
    '#ff3b30', // Vermelho Pulsação
    '#af52de', // Violeta Profundo
    '#5856d6', // Índigo Indigo
    '#ffcc00'  // Ouro Solar
  ]

  // ── Actions ──
  const closeDetails = () => {
    selectedNode.value = null
    nodeDetails.value = null
  }

  const setSelectedNode = (node) => {
    selectedNode.value = node
  }

  const setNodeDetails = (details) => {
    nodeDetails.value = details
  }

  const openSource = async () => {
    if (nodeDetails.value && nodeDetails.value.path) {
      const bridge = (window.go?.core?.App) || (window.go?.main?.App)
      if (bridge?.OpenFileInEditor) {
        await bridge.OpenFileInEditor(nodeDetails.value.path)
      }
    }
  }

  const checkHealth = async () => {
    try {
      const bridge = (window.go?.core?.App) || (window.go?.main?.App)
      if (bridge?.AnalyzeGraphHealth) {
        const stats = await bridge.AnalyzeGraphHealth()
        graphHealth.value = stats
      }
    } catch (e) {
      console.error("Erro ao analisar saúde:", e)
    }
  }

  const clearGraph = () => {
    nodes.value = []
    edges.value = []
    selectedNode.value = null
    nodeDetails.value = null
    discoveryStatus.value = null
    graphHealth.value = { density: 0, conflicts: 0, active_nodes: 0 }
    highlightedLinks.value = new Set()
    clickedNodeLinks.value = new Set()
    highlightedNeighbors.value = new Set()

    // 🚀 [Mixer] Força a limpeza no motor de renderização Deck.gl
    if (graphInstance.value && graphInstance.value.graphData) {
      graphInstance.value.graphData({ nodes: [], links: [] })
    }
  }

  return {
    // Estado
    nodes, edges, graphLogs, activeNode,
    graphInstance,
    selectedNode, nodeDetails, discoveryStatus,
    graphHealth,
    highlightedLinks, clickedNodeLinks, highlightedNeighbors,
    xRayThreshold, scanLoading, pruneLoading, skeletalMode,
    showFps, currentFps,
    scanning, showConfirmModal, modalMode,
    currentConflict,
    communityPalette,
    // Actions
    closeDetails, openSource, checkHealth, resetHighlights, clearGraph,
    setSelectedNode, setNodeDetails
  }
})
