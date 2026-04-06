<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { EventsOn } from '../../wailsjs/runtime'
import * as THREE from 'three'
import ForceGraph3D from '3d-force-graph'
import { ScanVault } from '../../wailsjs/go/main/App'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()

// Recipientes de Pooling (Atrasar inicialização para garantir que THREE esteja pronto)
let sphereLowRes, sphereVirtual, sphereActive, materialCache
const getCachedMaterial = (color, opacity, intensity) => {
  if (!materialCache) materialCache = new Map()
  const key = `${color}-${opacity}-${intensity}`
  if (!materialCache.has(key)) {
    materialCache.set(key, new THREE.MeshLambertMaterial({
      color: color,
      transparent: true,
      opacity: opacity,
      emissive: color,
      emissiveIntensity: intensity
    }))
  }
  return materialCache.get(key)
}

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] },
  graphLogs: { type: Array, default: () => [] },
  activeNode: { type: String, default: null } 
})

const containerRef = ref(null)
const logContainerRef = ref(null)
const highlightedLinks = ref(new Set()) // Armazena IDs de links destacados (RAG trail)
const clickedNodeLinks = ref(new Set()) // Armazena links do nó clicado atualmente
let Graph = null
let clickedNodeTimeout = null
let moveInterval = null
const keys = { w: false, a: false, s: false, d: false, q: false, e: false }
const moveSpeed = 20 

// 🩻 X-RAY MODE & RECON STATE
const xRayThreshold = ref(0)
const scanLoading = ref(false)
const pruneLoading = ref(false)
const skeletalMode = ref(false)

// Paleta de Cores Cibernéticas para Comunidades (Louvain)
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

const selectedNode = ref(null)
const nodeDetails = ref(null)
const graphHealth = ref({ density: 0, conflicts: 0, active_nodes: 0 })

const checkHealth = async () => {
  try {
    const stats = await window.go.main.App.AnalyzeGraphHealth()
    graphHealth.value = stats
  } catch (e) {
    console.error("Erro ao analisar saúde:", e)
  }
}

const closeDetails = () => {
  selectedNode.value = null
  nodeDetails.value = null
}

const openSource = async () => {
  if (nodeDetails.value && nodeDetails.value.path) {
    await window.go.main.App.OpenFileInEditor(nodeDetails.value.path)
  }
}

// Converte os dados para o formato do 3d-force-graph (incluindo nós virtuais e filtragem X-Ray)
const getGraphData = () => {
  const nodesMap = new Map()
  
  // 1. Adicionar nós reais (Filtrando por PageRank se X-Ray ativo)
  props.nodes.forEach(n => {
    const pr = n.pagerank || 0
    // O X-Ray só filtra se o threshold for > 0
    if (xRayThreshold.value === 0 || pr >= xRayThreshold.value || n.type === 'source' || n.type === 'system') {
      nodesMap.set(n.id, { ...n })
    }
  })

  // 2. Adicionar nós virtuais a partir de conexões que não existem em 'nodes'
  // (Somente se os destinos/origens passaram no filtro X-Ray)
  props.edges.forEach(e => {
    const s = e.source.id || e.source
    const t = e.target.id || e.target
    
    if (nodesMap.has(s) || nodesMap.has(t)) {
      if (!nodesMap.has(s)) nodesMap.set(s, { id: s, name: s, virtual: true })
      if (!nodesMap.has(t)) nodesMap.set(t, { id: t, name: t, virtual: true })
    }
  })

  const links = (skeletalMode.value ? props.edges.filter(e => e.label === 'mst' || e.is_mst) : props.edges).filter(e => {
    const s = e.source.id || e.source
    const t = e.target.id || e.target
    return nodesMap.has(s) && nodesMap.has(t)
  }).map(e => ({
    source: e.source.id || e.source,
    target: e.target.id || e.target,
    ...e
  }))

  return { 
    nodes: Array.from(nodesMap.values()), 
    links 
  }
}

const initGraph = () => {
  if (!containerRef.value) return

  // Inicialização sob demanda das geometrias
  if (!sphereLowRes) sphereLowRes = new THREE.SphereGeometry(12, 8, 8)
  if (!sphereVirtual) sphereVirtual = new THREE.SphereGeometry(8, 6, 6)
  if (!sphereActive) sphereActive = new THREE.SphereGeometry(20, 12, 12)

  Graph = ForceGraph3D()(containerRef.value)
    .graphData(getGraphData())
    .backgroundColor('#09090b') 
    .showNavInfo(false)
    .nodeLabel(node => {
      const type = node['document-type'] || 'chunk'
      const label = node.name || node.id
      const icon = type === 'system' ? '⚙️' : (type === 'source' ? '📄' : (type === 'memory' ? '🧠' : '📝'))
      return `<div class="node-tooltip">
                <span class="type-tag ${type}">${icon} ${type.toUpperCase()}</span>
                <br/><b>${label}</b>
              </div>`
    })
    .nodeColor(node => {
      const type = node['document-type'] || 'chunk'
      if (type === 'system') return '#ffffff'
      if (type === 'source' || type === 'obsidian') return '#00f2ff' // Ciano Néon
      if (type === 'memory') return '#fcd34d' // Dourado
      return '#3b82f6' // Azul padrão
    })
    .nodeRelSize(6) // Tamanho de colisão aumentado
    .nodeOpacity(1)
    .nodeThreeObjectExtend(true)
    .linkCurvature(0.25)
    .linkColor(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      
      if (clickedNodeLinks.value.has(`${s}-${t}`) || clickedNodeLinks.value.has(`${t}-${s}`)) return '#ffffff' 
      return 'rgba(0, 242, 255, 0.6)'
    })
    .linkOpacity(0.5)
    .linkWidth(link => {
      const weight = link.weight || 1
      return Math.min(1.2 + (weight * 0.4), 4.0) 
    })
    .linkDirectionalParticles(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      if (clickedNodeLinks.value.has(`${s}-${t}`) || clickedNodeLinks.value.has(`${t}-${s}`)) return 4
      if (highlightedLinks.value.has(`${s}-${t}`) || highlightedLinks.value.has(`${t}-${s}`)) return 2
      return 0 
    })
    .linkDirectionalParticleSpeed(0.006)
    .linkDirectionalParticleWidth(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      if (clickedNodeLinks.value.has(`${s}-${t}`) || clickedNodeLinks.value.has(`${t}-${s}`)) return 2.5
      return 1.5
    })
    .onNodeClick(async node => {
      // ── Zoom no nó ──
      const distance = 60
      const distRatio = 1 + distance/Math.hypot(node.x, node.y, node.z)
      Graph.cameraPosition(
        { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio }, 
        node, 
        2000
      )

      // ── Animação de partículas nos links conectados ao nó clicado ──
      // Limpa o timeout anterior se existir
      if (clickedNodeTimeout) clearTimeout(clickedNodeTimeout)

      // Encontra todos os links conectados a este nó
      const { links } = Graph.graphData()
      clickedNodeLinks.value.clear()
      links.forEach(link => {
        const s = link.source.id || link.source
        const t = link.target.id || link.target
        if (s === node.id || t === node.id) {
          clickedNodeLinks.value.add(`${s}-${t}`)
          clickedNodeLinks.value.add(`${t}-${s}`)
        }
      })

      // Força refresh das partículas
      Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())

      // Apaga automaticamente após 5 segundos
      clickedNodeTimeout = setTimeout(() => {
        clickedNodeLinks.value.clear()
        Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
      }, 5000)

      // ── Carregar Proveniência (Auditoria) ──
      // ── Reforço Neural (Aprendizado Ativo) ──
      window.go.main.App.HandleNodeClick(node.id)

      selectedNode.value = node
      nodeDetails.value = null // Reset imediato para mostrar o loader
      nodeDetails.value = { loading: true } // Loader fluido
      
      try {
        const details = await window.go.main.App.GetNodeDetails(node.id)
        if (details) {
          nodeDetails.value = details
        } else {
          nodeDetails.value = {
            path: "Conceito Neural",
            content: `O nó '${node.id}' é uma ponte lógica criada pela IA para conectar suas ideias. Ele não possui um arquivo físico, mas serve como âncora semântica no seu grafo.`,
            source: "Inteligência Artificial",
            isVirtual: true
          }
        }
      } catch (e) {
        console.error("Erro ao buscar detalhes:", e)
        nodeDetails.value = {
          path: "Conceito Neural",
          content: `O nó '${node.id}' é uma ponte lógica criada pela IA para conectar suas ideias. Ele não possui um arquivo físico, mas serve como âncora semântica no seu grafo.`,
          source: "Inteligência Artificial",
          isVirtual: true
        }
      }
    })

  // 🚀 CONFIGURAÇÃO DE FÍSICA SOLAR (NUCLEAÇÃO E ÓRBITAS)
  Graph.d3Force('charge').strength(node => {
      // Núcleos (Sóis) repelem mais para abrir espaço, Ideias repelem menos
      const type = node['document-type'] || 'chunk'
      return (type === 'chunk' || type === 'system') ? -1000 : -200
  })

  Graph.d3Force('link').distance(link => {
      const sType = link.source['document-type'] || 'chunk'
      const tType = link.target['document-type'] || 'chunk'
      
      // Órbita Próxima: Ideias conectadas a Notas (30px)
      if (sType === 'memory' || tType === 'memory') return 30
      
      // Distância Interestelar: Notas conectadas entre si (150px)
      return 150
  }).strength(0.8)

  // Otimização de CPU: Acelera o repouso da simulação
  Graph.d3AlphaDecay(0.04)
  Graph.d3VelocityDecay(0.3)

  Graph.nodeThreeObject(node => {
    const isVirtual = node.virtual
    const isActive = node.id === props.activeNode
    const type = node['status'] === 'legacy' ? 'legacy' : (node['status'] === 'conflict' ? 'conflict' : (node['document-type'] || node['document_type'] || 'chunk'))
    
    const colors = {
      source: '#a855f7',
      page: '#22d3ee',
      chunk: '#3b82f6',
      system: '#f1f5f9',
      memory: '#f472b6',
      legacy: '#475569',
      conflict: '#ef4444',
      virtual: '#1e3a8a',
      active: '#fcd34d'
    }

    const displayColor = node.status === 'conflict' ? colors.conflict : (node.status === 'legacy' ? colors.legacy : (colors[type] || colors.chunk))
    const nodeColor = isActive ? colors.active : (isVirtual ? colors.virtual : displayColor)
    
    // Otimização: Reuso de Geometria
    const geometry = isActive ? sphereActive : (isVirtual ? sphereVirtual : sphereLowRes)
    
    // Otimização: Reuso de Material (Pool por cor/opacidade/emissão)
    const opacity = type === 'legacy' ? 0.3 : (isVirtual ? 0.2 : 0.9)
    const intensity = isActive ? 1.2 : (isVirtual ? 0 : 0.5)
    const material = getCachedMaterial(nodeColor, opacity, intensity)
    
    return new THREE.Mesh(geometry, material)
  })

  // Loop de animação e pulso (opcional se o force-graph já lidar bem)
}

// Watchers para sincronização
watch(() => [props.nodes, props.edges], () => {
  if (Graph) {
    Graph.graphData(getGraphData())
  }
}, { deep: true })

watch(() => props.activeNode, (newId) => {
  if (!Graph || !newId) return

  const node = Graph.graphData().nodes.find(n => n.id === newId)
  if (node) {
    // 1. Fly-to suave para o "Foco do Pensamento"
    Graph.cameraPosition(
      { x: node.x + 100, y: node.y + 100, z: node.z + 100 }, 
      node, 
      2000
    )
    
    // 2. Adicionar uma luz dinâmica temporária no nó ativo
    const light = new THREE.PointLight(0xfcd34d, 2, 100)
    light.position.set(node.x, node.y, node.z)
    Graph.scene().add(light)
    setTimeout(() => { Graph.scene().remove(light) }, 3000)

    // 3. Refresh visual
    Graph.nodeThreeObject(Graph.nodeThreeObject()) 
  }
})

// 🩻 Watcher reativo para o Modo X-Ray
watch(xRayThreshold, () => {
  if (Graph) {
    Graph.graphData(getGraphData())
  }
})

// 🕵️‍♂️ Funções de Reconhecimento e Poda
const runReconScan = async () => {
  if (scanLoading.value) return
  scanLoading.value = true
  try {
    const result = await window.go.main.App.RunReconScan()
    console.log("[RECON] Scan concluído:", result)
  } catch (e) {
    console.error("Erro no Recon Scan:", e)
  } finally {
    scanLoading.value = false
  }
}

const pruneNodes = async () => {
  if (confirm(`Deseja remover permanentemente nós com PageRank abaixo de ${xRayThreshold.value}? (Notas de origem são protegidas)`)) {
    pruneLoading.value = true
    try {
      const result = await window.go.main.App.PruneGraph(xRayThreshold.value)
      console.log("[PODA] Resultado:", result)
    } catch (e) {
      console.error("Erro na poda:", e)
    } finally {
      pruneLoading.value = false
    }
  }
}

// Escutar Destaques de Trajetória (Context-Flow inspirado no TrustGraph)
onMounted(() => {
  window.runtime.EventsOn('graph:highlight', (linkData) => {
    // Criamos uma chave única para o link bidirecional
    const linkId1 = `${linkData.source}-${linkData.target}`
    const linkId2 = `${linkData.target}-${linkData.source}`
    
    highlightedLinks.value.add(linkId1)
    highlightedLinks.value.add(linkId2)
    
    // Forçar atualização visual das arestas (links) no motor Three.js
    if (Graph) {
      Graph.linkColor(Graph.linkColor())
      Graph.linkWidth(Graph.linkWidth())
    }

    // Efeito de Rastro: O brilho desaparece após 4 segundos (Cinemático)
    setTimeout(() => {
      highlightedLinks.value.delete(linkId1)
      highlightedLinks.value.delete(linkId2)
      if (Graph) {
        Graph.linkColor(Graph.linkColor())
        Graph.linkWidth(Graph.linkWidth())
      }
    }, 4000)
  })
})

watch(() => props.graphLogs, () => {
  nextTick(() => {
    if (logContainerRef.value) {
      logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
    }
  })
}, { deep: true })

const currentConflict = ref(null)
// O estado 'isNavigating' agora é gerenciado globalmente pela store orchestrator

onMounted(() => {
  initGraph()
  
  // Sincroniza todos os nós conhecidos do banco de dados na partida
  if (window.go && window.go.main && window.go.main.App) {
    window.go.main.App.SyncAllNodes()
  }
  
  // Listener de Conflitos do Agente Validador
  window.runtime.EventsOn("graph:conflict", (conflict) => {
    currentConflict.value = conflict
    console.warn("⚠️ CONFLITO DETECTADO:", conflict)
  })

  // 🪐 Sincronização de Saúde (Automática após Sync)
  window.runtime.EventsOn("graph:health:update", (stats) => {
    graphHealth.value = stats
  })

  // 🪐 Sincronização: O componente agora confia inteiramente nas props reativas do Vue
  // para grandes volumes de dados (Batch Sync), evitando sobrecarga de renderização.

  // 🕸️ Ouvinte de Arestas Dinâmicas (Streaming de Conexões)
  window.runtime.EventsOn("graph:edge", (edge) => {
    if (!Graph || !edge?.source || !edge?.target) return
    
    const { nodes, links } = Graph.graphData()
    let dataChanged = false
    
    // Assegura que ambos os nós existem no grafo. Se não, cria um "nó virtual/fantasma"
    // Isso é vital porque o Go extrai conexões semânticas (conceitos puros) que não são arquivos de texto!
    let sourceNode = nodes.find(n => n.id === edge.source)
    if (!sourceNode) {
      sourceNode = { id: edge.source, name: edge.source, "document-type": "chunk", virtual: true }
      nodes.push(sourceNode)
      dataChanged = true
    }

    let targetNode = nodes.find(n => n.id === edge.target)
    if (!targetNode) {
      targetNode = { id: edge.target, name: edge.target, "document-type": "chunk", virtual: true }
      nodes.push(targetNode)
      dataChanged = true
    }
    
    // Evita duplicatas visuais (verifica pelo ID e assegura não duplicar bidirecionalmente)
    const exists = links.find(l => 
      ((l.source.id || l.source) === edge.source && (l.target.id || l.target) === edge.target) || 
      ((l.source.id || l.source) === edge.target && (l.target.id || l.target) === edge.source)
    )
    
    if (!exists) {
      links.push({
        source: edge.source,
        target: edge.target,
        weight: edge.weight || 1
      })
      dataChanged = true
    } else {
      // Se a aresta já existe, reforçamos o peso dela (Reforço Sináptico Dinâmico)
      exists.weight = (exists.weight || 1) + (edge.weight || 1)
      dataChanged = true
    }

    if (dataChanged) {
      // Atualiza o motor físico preservando as coordenadas antigas
      Graph.graphData({ nodes, links })
    }
  })

  // 🎮 Módulo de Movimentação Gamificada (WASD + QE)
  const isInputFocused = () => {
    const el = document.activeElement
    return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
  }

  const handleKeyDown = (e) => {
    if (isInputFocused()) return
    const k = e.key.toLowerCase()
    if (k in keys) {
      keys[k] = true
      if (!moveInterval) startMoving()
    }
  }

  const handleKeyUp = (e) => {
    const k = e.key.toLowerCase()
    if (k in keys) keys[k] = false
    
    // Para o loop se todas as teclas forem soltas
    if (!Object.values(keys).some(v => v)) {
      if (moveInterval) {
        cancelAnimationFrame(moveInterval)
        moveInterval = null
      }
    }
  }

  const startMoving = () => {
    const move = () => {
      if (!Graph) return
      
      const camera = Graph.camera()
      const pos = Graph.cameraPosition()
      const direction = new THREE.Vector3()
      camera.getWorldDirection(direction)
      
      const right = new THREE.Vector3().crossVectors(camera.up, direction).normalize()
      
      let dx = 0, dy = 0, dz = 0

      if (keys.w) { dx += direction.x * moveSpeed; dy += direction.y * moveSpeed; dz += direction.z * moveSpeed; }
      if (keys.s) { dx -= direction.x * moveSpeed; dy -= direction.y * moveSpeed; dz -= direction.z * moveSpeed; }
      if (keys.a) { dx += right.x * moveSpeed; dy += right.y * moveSpeed; dz += right.z * moveSpeed; }
      if (keys.d) { dx -= right.x * moveSpeed; dy -= right.y * moveSpeed; dz -= right.z * moveSpeed; }
      if (keys.q) { dy -= moveSpeed; }
      if (keys.e) { dy += moveSpeed; }

      if (dx !== 0 || dy !== 0 || dz !== 0) {
        Graph.cameraPosition({
          x: pos.x + dx,
          y: pos.y + dy,
          z: pos.z + dz
        })
      }

      moveInterval = requestAnimationFrame(move)
    }
    moveInterval = requestAnimationFrame(move)
  }

  window.addEventListener('keydown', handleKeyDown)
  window.addEventListener('keyup', handleKeyUp)

  // 🎬 Percurso Cinematográfico da IA: Anima cada hop individualmente com delay
  window.runtime.EventsOn("graph:traverse", (data) => {
    if (!Graph || !data?.hops?.length) return

    orchestrator.isNavigating = true
    const hops = data.hops
    const HOPDelay = 800 // ms entre cada hop

    hops.forEach((hop, i) => {
      setTimeout(() => {
        const { nodes } = Graph.graphData()

        // 1. Voa a câmera até o nó DESTINO (To)
        const targetNode = nodes.find(n => n.id === hop.to || n.name === hop.to)
        if (targetNode) {
          Graph.cameraPosition(
            { x: targetNode.x + 80, y: targetNode.y + 60, z: targetNode.z + 80 },
            targetNode,
            600
          )

          // 2. Explode uma luz pontual dourada no destino e faz o nó pulsar
          const light = new THREE.PointLight(0x4facfe, 3, 80)
          light.position.set(targetNode.x, targetNode.y, targetNode.z)
          Graph.scene().add(light)
          
          // Efeito de pulso no objeto 3D do nó
          const nodeObj = targetNode.__threeObj
          if (nodeObj) {
            const originalScale = nodeObj.scale.x
            nodeObj.scale.set(originalScale * 2.5, originalScale * 2.5, originalScale * 2.5)
            // Retorna ao tamanho original suavemente
            let scale = originalScale * 2.5
            const interval = setInterval(() => {
              scale -= 0.1
              if (scale <= originalScale) {
                nodeObj.scale.set(originalScale, originalScale, originalScale)
                clearInterval(interval)
              } else {
                nodeObj.scale.set(scale, scale, scale)
              }
            }, 30)
          }

          setTimeout(() => Graph.scene().remove(light), 1200)
        }

        // 3. Acende partículas no link deste hop
        const linkKey1 = `${hop.from}-${hop.to}`
        const linkKey2 = `${hop.to}-${hop.from}`
        clickedNodeLinks.value.add(linkKey1)
        clickedNodeLinks.value.add(linkKey2)
        Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())

        // 4. Remove a partícula deste hop após 3s
        setTimeout(() => {
          clickedNodeLinks.value.delete(linkKey1)
          clickedNodeLinks.value.delete(linkKey2)
          Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
        }, 3000)

        // 5. Marca como "fim da travessia" no último hop
        if (i === hops.length - 1) {
          setTimeout(() => { orchestrator.isNavigating = false }, 1500)
        }
      }, i * HOPDelay)
    })
  })

  // Resize handler
  window.addEventListener('resize', () => {
    if (Graph && containerRef.value) {
      Graph.width(containerRef.value.clientWidth)
      Graph.height(containerRef.value.clientHeight)
    }
  })

  onUnmounted(() => {
    window.removeEventListener('keydown', handleKeyDown)
    window.removeEventListener('keyup', handleKeyUp)
    if (moveInterval) cancelAnimationFrame(moveInterval)
  })
})

const resolveConflict = async (decision) => {
  if (!currentConflict.value) return
  
  const c = currentConflict.value
  console.log("Resolvendo conflito com decisão:", decision)
  
  try {
    // 🛠️ Chamada RPC para o Agente Validador no Go
    await window.go.main.App.ResolveConflict(
      decision, 
      c.subject, 
      c.predicate, 
      c.old_id, 
      c.new, 
      c.session_id
    )
    currentConflict.value = null
  } catch (err) {
    console.error("Falha ao resolver conflito:", err)
  }
}

onUnmounted(() => {
  if (Graph) Graph._destructor() // Limpeza de memória
})

// -- NOVA LÓGICA DE SINCRONIZAÇÃO TOTAL --
const showConfirmModal = ref(false)
const modalMode = ref('fast') // 'fast' ou 'full'

const handleFullSync = () => {
  modalMode.value = 'full'
  showConfirmModal.value = true
}

const handleFastSync = () => {
  modalMode.value = 'fast'
  showConfirmModal.value = true
}

const confirmSync = () => {
  showConfirmModal.value = false
  if (modalMode.value === 'full') {
    executeFullSync()
  } else {
    triggerScan()
  }
}

const executeFullSync = async () => {
  showConfirmModal.value = false
  scanning.value = true
  try {
    // Chama o método atômico no Go que limpa e reindexa em sequência garantida
    await window.go.main.App.FullSync()
  } catch (e) {
    console.error("Erro na sincronização:", e)
  } finally {
    scanning.value = false
    if (Graph) Graph.zoomToFit(800)
  }
}

const scanning = ref(false)
const triggerScan = async () => {
  if (scanning.value) return
  scanning.value = true
  try {
    await ScanVault()
  } catch (error) {
    console.error("Erro no Scan:", error)
  } finally {
    scanning.value = false
  }
}

watch(skeletalMode, () => {
  if (Graph) Graph.graphData(getGraphData())
})
</script>

<template>
  <div class="graph-wrapper animate-fade-in">
    <!-- MODAL DE CONFIRMAÇÃO DINÂMICO -->
    <div v-if="showConfirmModal" class="premium-modal-overlay">
      <div class="premium-modal-content">
        <div class="modal-icon">{{ modalMode === 'full' ? '⚙️' : '🚀' }}</div>
        <h3 class="modal-title">{{ modalMode === 'full' ? 'Reindexação Forçada' : 'Sincronização Inteligente' }}</h3>
        
        <div class="modal-body">
          <p v-if="modalMode === 'full'" class="modal-text">
            Deseja forçar uma varredura completa de todos os <strong>{{ graphHealth.active_nodes || nodes.length }} arquivos</strong>?<br/>
            <span class="warning-sub">Isso reconstrói o cache de auditoria e garante 100% de integridade. Use apenas se notar dados faltando.</span>
          </p>
          <p v-else class="modal-text">
            Deseja iniciar a sincronização incremental?<br/>
            <span class="info-sub">O Maestro buscará apenas notas <strong>novas ou modificadas</strong>. É o método mais rápido e econômico.</span>
          </p>
        </div>

        <div class="modal-actions">
           <button @click="showConfirmModal = false" class="btn-cancel">CANCELAR</button>
           <button @click="confirmSync" class="btn-confirm" :class="modalMode">
             {{ modalMode === 'full' ? 'INICIAR FAXINA' : 'SINCRONIZAR AGORA' }}
           </button>
        </div>
      </div>
    </div>

    <!-- Container para o Grafo 3D (WebGL) -->
    <div ref="containerRef" class="main-canvas"></div>
    
    <!-- Controles & Console de Logs (Painel de Pensamento Vidrado) -->
    <div class="graph-ui glass">
      <div class="ui-header">
        <span class="pulse" :class="{ 'ai-active': orchestrator.isNavigating }"></span>
        <h3>Conhecimento Obsidian 3D</h3>
        <span v-if="orchestrator.isNavigating" class="ai-status-label animate-pulse">IA RACIOCINANDO...</span>
      </div>
      
      <div class="ui-actions">
        <div class="sync-controls">
          <button @click="handleFastSync" class="action-btn main-sync" :class="{'scanning-btn': scanning}" title="Sincronização Rápida">
            <span v-if="!scanning">🚀</span><span v-else class="spin">⏳</span>
            <span>SINCRONIZAR</span>
          </button>
          <button @click="handleFullSync" class="action-btn icon-only-btn" :class="{'scanning-btn': scanning}" title="Sincronização Total">
            <span>⚙️</span>
          </button>
        </div>
        <div class="stat-item">
          <span class="val">{{ graphHealth.active_nodes || nodes.length }}</span>
          <span class="lab">NOTAS</span>
        </div>
      </div>

      <!-- 🩻 CONTROLES X-RAY & RECON (FASE 23) -->
      <div class="xray-panel glass">
        <div class="xray-header">
          <span class="xray-icon">🩻</span>
          <span>MODO X-RAY</span>
          <span class="xray-val">{{ (xRayThreshold * 100).toFixed(0) }}</span>
        </div>
        <input type="range" min="0" max="1" step="0.01" v-model.number="xRayThreshold" class="xray-slider" />
        
        <div class="recon-actions">
           <button @click="runReconScan" class="recon-btn" :disabled="scanLoading" title="Scan Proativo">
             <span v-if="!scanLoading">🕵️ RECON</span>
             <span v-else class="spin">⏳</span>
           </button>
           <button @click="pruneNodes" class="prune-btn" :disabled="pruneLoading" title="Poda Neural">
             <span v-if="!pruneLoading">🧹 PODA</span>
             <span v-else class="spin">⏳</span>
           </button>
           <button @click="skeletalMode = !skeletalMode" :class="['recon-btn', { active: skeletalMode }]" title="Modo Esqueleto (MST)">
             <span v-if="!skeletalMode">🩻 MST</span>
             <span v-else>👁️ FULL</span>
           </button>
        </div>
      </div>

      <!-- HUD DE SAÚDE DO GRAFO (HEALTH MONITOR) -->
      <div class="graph-health-hud">
        <div class="health-info">
          <div class="health-stat">
            <span class="label">DENSIDADE</span>
            <span class="value">{{ (graphHealth.density * 100).toFixed(0) }}%</span>
          </div>
          <div class="health-stat" :class="{'has-conflicts': graphHealth.conflicts > 0}">
            <span class="label">CONFLITOS</span>
            <span class="value">{{ graphHealth.conflicts }}</span>
          </div>
        </div>
        <button @click="checkHealth" class="health-btn" title="Analisar Integridade">🛡️ CHECK</button>
      </div>

      <!-- O CONSOLE VIVO DO RACIOCÍNIO IA -->
      <div class="graph-logs-console" ref="logContainerRef" v-if="graphLogs.length > 0">
        <div v-for="(log, idx) in graphLogs" :key="idx" class="log-entry">
          <span class="log-text">{{ log }}</span>
        </div>
      </div>
    </div> <!-- Fim da graph-ui -->

    <!-- POP-UP DE VALIDAÇÃO (AGENTE DA VERDADE) -->
    <div v-if="currentConflict" class="conflict-overlay">
      <div class="conflict-modal glass">
        <div class="conflict-header">
          <span class="alert-icon">⚠️</span>
          <h4>Contradição Semântica</h4>
        </div>
        <p>A IA detectou uma divergência sobre <b>{{ currentConflict.subject }}</b>:</p>
        <div class="conflict-options">
          <div class="opt old" @click="resolveConflict('old')">
            <span class="lab">PASSADO</span>
            <span class="val">{{ currentConflict.old }}</span>
          </div>
          <div class="opt new" @click="resolveConflict('new')">
            <span class="lab">PRESENTE</span>
            <span class="val">{{ currentConflict.new }}</span>
          </div>
        </div>
        <p class="hint">Escolha a verdade ativa. A outra será marcada como legado.</p>
      </div>
    </div>

    <!-- PAINEL DE PROVENIÊNCIA (AUDITORIA) -->
    <transition name="slide-fade">
      <aside v-if="selectedNode" class="provenance-panel glass">
        <header class="panel-header">
          <div class="header-content">
            <div class="source-icon">🔎</div>
            <h3>Proveniência</h3>
          </div>
          <button @click="closeDetails" class="close-btn">×</button>
        </header>

        <div class="panel-body">
          <!-- Estado: Carregando -->
          <div v-if="!nodeDetails || nodeDetails.loading" class="loading-provenance">
            <div class="spinner"></div>
            <span>Sintonizando Base...</span>
          </div>

          <!-- Estado: Sucesso ou Erro (com conteúdo) -->
          <div v-else class="details-content">
            <div class="provenance-metadata">
              <div class="meta-item">
                <span class="lab">DOCUMENTO ORIGEM</span>
                <div class="val-box">{{ nodeDetails?.path || 'Escaneando...' }}</div>
              </div>
              
              <div class="meta-item">
                <span class="lab">TRECHO FUNDAMENTADO (CHUNK)</span>
                <div class="content-box glass">
                   {{ nodeDetails?.content || 'Aguardando recuperação semântica...' }}
                </div>
              </div>
            </div>

            <button v-if="nodeDetails && nodeDetails.path && !nodeDetails.isVirtual && nodeDetails.path !== 'Conceito Neural'" 
                    @click="openSource" class="open-btn premium-btn">
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

<style scoped>
.graph-wrapper {
  width: 100%;
  height: 100vh;
  background: var(--bg-dark);
  position: relative;
  overflow: hidden;
  pointer-events: auto; /* Garante que o wrapper receba eventos */
}

.main-canvas {
  width: 100%;
  height: 100%;
  position: absolute;
  top: 0;
  left: 0;
  z-index: 2;
  pointer-events: auto; /* Indispensável para o mouse/zoom do force-graph */
}

/* PROVENANCE UI STYLES */
.provenance-panel {
  position: absolute;
  top: 20px;
  right: 20px;
  width: 380px;
  max-height: calc(100% - 40px);
  z-index: 1000;
  border-radius: 20px;
  border: 1px solid rgba(59, 130, 246, 0.4);
  display: flex;
  flex-direction: column;
  box-shadow: 0 10px 40px rgba(0,0,0,0.6);
  backdrop-filter: blur(25px);
  background: rgba(15, 23, 42, 0.7);
  animation: slideIn 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.panel-header {
  padding: 20px;
  border-bottom: 1px solid rgba(255,255,255,0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-content h3 {
  margin: 0;
  font-size: 0.9rem;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: var(--primary);
}

.close-btn {
  background: none;
  border: none;
  color: white;
  font-size: 1.5rem;
  cursor: pointer;
  opacity: 0.6;
  transition: opacity 0.2s;
}

.close-btn:hover {
  opacity: 1;
}

.panel-body {
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.info-group label {
  display: block;
  font-size: 0.7rem;
  font-weight: 800;
  color: var(--text-dim);
  margin-bottom: 8px;
  letter-spacing: 1.5px;
}

.source-path {
  font-family: 'Fira Code', monospace;
  font-size: 0.8rem;
  color: #94a3b8;
  word-break: break-all;
  background: rgba(0,0,0,0.2);
  padding: 10px;
  border-radius: 8px;
}

.content-preview {
  font-size: 0.9rem;
  line-height: 1.6;
  color: #e2e8f0;
  background: rgba(255,255,255,0.03);
  padding: 15px;
  border-radius: 12px;
  border: 1px solid rgba(255,255,255,0.05);
}

.btn-open-source {
  padding: 14px;
  background: var(--primary);
  border: none;
  border-radius: 12px;
  color: white;
  font-weight: 800;
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.3s;
  box-shadow: 0 4px 20px rgba(59, 130, 246, 0.3);
  margin-top: 10px;
}

.btn-open-source:hover {
  transform: translateY(-2px);
  background: #2563eb;
  box-shadow: 0 8px 30px rgba(59, 130, 246, 0.5);
}

.loading-provenance {
  padding: 40px;
  text-align: center;
  color: var(--primary);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

@keyframes slideIn {
  from { opacity: 0; transform: translateX(30px); }
  to { opacity: 1; transform: translateX(0); }
}

.slide-fade-enter-active, .slide-fade-leave-active {
  transition: all 0.3s ease;
}

.slide-fade-enter-from, .slide-fade-leave-to {
  transform: translateX(20px);
  opacity: 0;
}

/* Tooltip customizado para o space 3D */
:deep(.node-tooltip) {
  padding: 8px 12px;
  background: rgba(15, 23, 42, 0.95);
  border: 1px solid rgba(59, 130, 246, 0.3);
  border-radius: 10px;
  color: white;
  font-family: 'Outfit', sans-serif;
  font-size: 11px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.8);
  backdrop-filter: blur(4px);
}

:deep(.type-tag) {
  font-size: 8px;
  font-weight: 800;
  padding: 2px 6px;
  border-radius: 4px;
  margin-bottom: 4px;
  display: inline-block;
  letter-spacing: 1px;
}

:deep(.type-tag.source) { background: rgba(168, 85, 247, 0.2); color: #a855f7; border: 1px solid #a855f7; }
:deep(.type-tag.page) { background: rgba(34, 211, 238, 0.2); color: #22d3ee; border: 1px solid #22d3ee; }
:deep(.type-tag.chunk) { background: rgba(59, 130, 246, 0.2); color: #3b82f6; border: 1px solid #3b82f6; }
:deep(.type-tag.system) { background: rgba(248, 250, 252, 0.2); color: #f8fafc; border: 1px solid #f8fafc; }
:deep(.type-tag.memory) { background: rgba(244, 114, 182, 0.2); color: #f472b6; border: 1px solid #f472b6; }
:deep(.type-tag.legacy) { background: rgba(71, 85, 105, 0.2); color: #94a3b8; border: 1px solid #94a3b8; }
:deep(.type-tag.conflict) { background: rgba(239, 68, 68, 0.2); color: #ef4444; border: 1px solid #ef4444; }

/* Conflict Modal UI */
.conflict-overlay {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0,0,0,0.7);
  backdrop-filter: blur(8px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.conflict-modal {
  background: rgba(15, 23, 42, 0.95);
  padding: 2rem;
  border-radius: 24px;
  border: 1px solid rgba(239, 68, 68, 0.3);
  max-width: 450px;
  width: 90%;
  text-align: center;
  box-shadow: 0 20px 40px rgba(0,0,0,0.4);
}

.modal-body {
  text-align: center;
  margin: 15px 0 25px;
}

.warning-sub {
  display: block;
  font-size: 0.75rem;
  color: #ff9800;
  margin-top: 8px;
  font-style: italic;
  line-height: 1.4;
}

.info-sub {
  display: block;
  font-size: 0.75rem;
  color: #4facfe;
  margin-top: 8px;
  opacity: 0.8;
  line-height: 1.4;
}

.btn-confirm.full {
  background: linear-gradient(135deg, #ff416c 0%, #ff4b2b 100%) !important;
  color: white !important;
}

.btn-confirm.fast {
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%) !important;
  color: #0d1117 !important;
}

.conflict-header {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-bottom: 1.5rem;
}

.conflict-header h4 {
  margin: 0;
  color: #ef4444;
  text-transform: uppercase;
  letter-spacing: 2px;
}

.conflict-options {
  display: flex;
  gap: 1rem;
  margin: 1.5rem 0;
}

.opt {
  flex: 1;
  padding: 1.5rem;
  background: rgba(255,255,255,0.03);
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: 16px;
  cursor: pointer;
  transition: all 0.3s;
}

.opt:hover {
  background: rgba(255,255,255,0.08);
  border-color: var(--primary);
  transform: translateY(-4px);
}

.opt.new:hover { border-color: #f472b6; }

.opt .lab {
  display: block;
  font-size: 0.6rem;
  opacity: 0.5;
  margin-bottom: 8px;
}

.opt .val {
  font-weight: bold;
  font-size: 1.1rem;
}

.hint {
  font-size: 0.7rem;
  opacity: 0.6;
}

/* UI Panel */
.graph-ui {
  position: absolute;
  top: 2rem;
  left: 2rem;
  z-index: 10;
  padding: 1.2rem;
  border-radius: 20px;
  min-width: 280px;
  width: max-content;
  border: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ui-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ui-header h3 {
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 2px;
  text-transform: uppercase;
  color: var(--primary);
  margin: 0;
}

.pulse {
  width: 6px;
  height: 6px;
  background: var(--primary);
  border-radius: 50%;
  box-shadow: 0 0 8px var(--primary);
  display: inline-block;
  transition: all 0.3s ease;
}

.pulse.ai-active {
  background: #f472b6;
  box-shadow: 0 0 12px #f472b6, 0 0 20px rgba(244, 114, 182, 0.4);
  transform: scale(1.5);
}

.ai-status-label {
  font-size: 0.6rem;
  font-weight: 800;
  color: #f472b6;
  margin-left: auto;
  letter-spacing: 1px;
  text-shadow: 0 0 8px rgba(244, 114, 182, 0.5);
}

.animate-pulse {
  animation: pulse-op 1.5s infinite;
}

@keyframes pulse-op {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.ui-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.5rem;
}

.sync-controls {
  display: flex;
  gap: 6px;
  background: rgba(255, 255, 255, 0.03);
  padding: 4px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.main-sync {
  flex: 1;
  background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%) !important;
  color: #0d1117 !important;
  font-weight: 800 !important;
  padding: 8px 16px !important;
  min-width: 140px;
}

.main-sync:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 15px rgba(79, 172, 254, 0.4);
}

.icon-only-btn {
  width: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.05) !important;
}

.icon-only-btn:hover {
  background: rgba(255, 255, 255, 0.1) !important;
  color: #4facfe;
}

.action-btn {
  border: none;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  font-size: 0.75rem;
  letter-spacing: 0.5px;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

/* GRAPH HEALTH HUD */
.graph-health-hud {
  margin-top: 10px;
  background: rgba(0,0,0,0.3);
  border-radius: 12px;
  padding: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border: 1px solid rgba(255,255,255,0.05);
}

.health-info {
  display: flex;
  gap: 15px;
}

.health-stat {
  display: flex;
  flex-direction: column;
}

.health-stat .label {
  font-size: 0.5rem;
  font-weight: 800;
  color: var(--text-dim);
  letter-spacing: 1px;
}

.health-stat .value {
  font-size: 0.8rem;
  font-weight: 900;
  color: #4ade80;
}

.has-conflicts .value {
  color: #ef4444;
  animation: pulse-red 1s infinite;
}

.health-btn:hover { background: rgba(59, 130, 246, 0.2); }

/* --- MODAL PREMIUM --- */
.premium-modal-overlay {
  position: absolute;
  top: 0; left: 0; width: 100%; height: 100%;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(8px);
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  animation: fadeIn 0.3s ease;
}

.premium-modal-content {
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid rgba(59, 130, 246, 0.3);
  padding: 2.5rem;
  border-radius: 20px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5), 0 0 40px rgba(59, 130, 246, 0.1);
  text-align: center;
  max-width: 400px;
  transform: translateY(0);
  animation: slideUp 0.3s ease;
}

.modal-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
}

.modal-title {
  color: #fff;
  font-family: 'Outfit', sans-serif;
  font-size: 1.4rem;
  font-weight: 800;
  margin: 0 0 0.5rem 0;
  letter-spacing: 1px;
}

.modal-text {
  color: var(--p-text-dim, #94a3b8);
  font-size: 0.9rem;
  line-height: 1.5;
  margin-bottom: 2rem;
}

.modal-text strong {
  color: #ef4444;
}

.modal-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
}

.btn-cancel {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  padding: 0.8rem 1.5rem;
  border-radius: 12px;
  cursor: pointer;
  font-weight: 700;
  transition: 0.2s;
  letter-spacing: 1px;
  font-size: 0.8rem;
}
.btn-cancel:hover { background: rgba(255, 255, 255, 0.1); }

.btn-confirm {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
  border: none;
  color: #fff;
  padding: 0.8rem 1.5rem;
  border-radius: 12px;
  cursor: pointer;
  font-weight: 700;
  transition: 0.3s;
  letter-spacing: 1px;
  font-size: 0.8rem;
  box-shadow: 0 10px 20px rgba(59, 130, 246, 0.3);
}
.btn-confirm:hover {
  transform: translateY(-2px);
  box-shadow: 0 15px 25px rgba(59, 130, 246, 0.4);
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
@keyframes slideUp {
  from { opacity: 0; transform: translateY(20px); }
  to { opacity: 1; transform: translateY(0); }
}

.health-btn {
  background: rgba(59, 130, 246, 0.2);
  border: 1px solid rgba(59, 130, 246, 0.4);
  color: #60a5fa;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 0.55rem;
  font-weight: 800;
  cursor: pointer;
}

@keyframes pulse-red {
  0% { opacity: 0.6; }
  50% { opacity: 1; }
  100% { opacity: 0.6; }
}
.virtual-badge {
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.3);
  color: #4facfe;
  padding: 12px;
  border-radius: 12px;
  font-size: 0.7rem;
  font-weight: 800;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin-top: 20px;
}
.val {
  font-size: 1.2rem;
  font-weight: 900;
  color: white;
  line-height: 1;
}

.lab {
  font-size: 0.55rem;
  font-weight: 800;
  color: var(--text-dim);
  letter-spacing: 1px;
}

/* Background Imersivo */
.graph-bg {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: radial-gradient(circle at center, rgba(59, 130, 246, 0.05) 0%, transparent 70%);
  pointer-events: none;
  z-index: 1;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.animate-fade-in {
  animation: fadeIn 1s ease-out;
}

@keyframes spinFast {
  100% { transform: rotate(360deg); }
}

.spin {
  display: inline-block;
  animation: spinFast 1s linear infinite;
}

.scanning-btn {
  opacity: 0.7;
  pointer-events: none;
  border-color: var(--primary);
}

/* 🧠 Efeitos do Raciocínio (Cérebro Artificial Vivo) */
.edge-flow {
  stroke-dasharray: 4 4;
  animation: dashFlow 2s linear infinite;
}

@keyframes dashFlow {
  to { stroke-dashoffset: -20; }
}

/* ⚡ Pulso de Sinapse (Energia nos Caminhos) */
.edge-active {
  stroke: #fcd34d !important;
  stroke-width: 3 !important;
  stroke-opacity: 1 !important;
  stroke-dasharray: 8 4 !important;
  animation: dashFlow 0.5s linear infinite !important;
  filter: drop-shadow(0 0 5px #fcd34d);
  transition: stroke 0.3s, stroke-width 0.3s;
}

/* ⚙️ Console Visual Lateral */
.graph-logs-console {
  margin-top: 15px;
  max-height: 180px;
  overflow-y: auto;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  scroll-behavior: smooth;
}

.graph-logs-console::-webkit-scrollbar { width: 4px; }
.graph-logs-console::-webkit-scrollbar-thumb { background: rgba(59, 130, 246, 0.5); border-radius: 4px; }

.log-entry {
  font-family: Consolas, 'Fira Code', monospace;
  font-size: 0.6rem;
  color: rgba(255,255,255,0.6);
  border-left: 2px solid rgba(59, 130, 246, 0.5);
  padding-left: 6px;
  line-height: 1.4;
  word-break: break-all;
}

/* 🌀 Pulsação de Nó Ativo */
.pulse-ring {
  animation: pulse-ring 1.5s cubic-bezier(0.215, 0.61, 0.355, 1) infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.5); opacity: 0.5; }
  100% { transform: scale(1); opacity: 1; }
}
/* 🩻 ESTILOS X-RAY & RECON */
.xray-panel {
  background: rgba(15, 23, 42, 0.4);
  padding: 12px;
  border-radius: 14px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.xray-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 0.6rem;
  font-weight: 800;
  letter-spacing: 1px;
  color: #94a3b8;
}

.xray-icon { filter: drop-shadow(0 0 5px #3b82f6); }

.xray-val {
  margin-left: auto;
  color: #4facfe;
  font-family: 'Fira Code', monospace;
}

.xray-slider {
  width: 100%;
  accent-color: #4facfe;
  cursor: pointer;
}

.recon-actions {
  display: flex;
  gap: 8px;
}

.recon-btn, .prune-btn {
  flex: 1;
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(59, 130, 246, 0.3);
  color: #fff;
  padding: 6px;
  border-radius: 8px;
  font-size: 0.55rem;
  font-weight: 800;
  cursor: pointer;
  transition: all 0.2s;
}

.recon-btn:hover { background: rgba(59, 130, 246, 0.2); border-color: #4facfe; }
.prune-btn:hover { background: rgba(239, 68, 68, 0.2); border-color: #ef4444; color: #ef4444; }

.recon-btn.active {
  background: rgba(59, 130, 246, 0.4);
  border-color: #4facfe;
  box-shadow: 0 0 10px rgba(79, 172, 254, 0.4);
}

.recon-btn:disabled, .prune-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

@keyframes spin { 100% { transform: rotate(360deg); } }
.spin { display: inline-block; animation: spin 1s linear infinite; }
</style>
