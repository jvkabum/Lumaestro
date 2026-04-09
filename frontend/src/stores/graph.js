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

  // ── Saúde do Grafo ──
  const graphHealth = ref({ density: 0, conflicts: 0, active_nodes: 0 })

  // ── Links Destacados (RAG Trail & Click Animation) ──
  const highlightedLinks = ref(new Set())
  const clickedNodeLinks = ref(new Set())

  // ── 🩻 X-RAY MODE & RECON ──
  const xRayThreshold = ref(0)
  const scanLoading = ref(false)
  const pruneLoading = ref(false)
  const skeletalMode = ref(false)

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

  const openSource = async () => {
    if (nodeDetails.value && nodeDetails.value.path) {
      await OpenFileInEditor(nodeDetails.value.path)
    }
  }

  const checkHealth = async () => {
    try {
      const stats = await AnalyzeGraphHealth()
      graphHealth.value = stats
    } catch (e) {
      console.error("Erro ao analisar saúde:", e)
    }
  }

  return {
    // Estado
    nodes, edges, graphLogs, activeNode,
    graphInstance,
    selectedNode, nodeDetails,
    graphHealth,
    highlightedLinks, clickedNodeLinks,
    xRayThreshold, scanLoading, pruneLoading, skeletalMode,
    scanning, showConfirmModal, modalMode,
    currentConflict,
    communityPalette,
    // Actions
    closeDetails, openSource, checkHealth,
  }
})
