<script setup>
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { EventsOn } from '../../wailsjs/runtime'
import * as THREE from 'three'
import ForceGraph3D from '3d-force-graph'
import { ScanVault } from '../../wailsjs/go/main/App'

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] },
  graphLogs: { type: Array, default: () => [] },
  activeNode: { type: String, default: null } 
})

const containerRef = ref(null)
const logContainerRef = ref(null)
const highlightedLinks = ref(new Set()) // Armazena IDs de links destacados
let Graph = null

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

// Converte os dados para o formato do 3d-force-graph (incluindo nós virtuais)
const getGraphData = () => {
  const nodesMap = new Map()
  
  // 1. Adicionar nós reais
  props.nodes.forEach(n => {
    nodesMap.set(n.id, { ...n })
  })

  // 2. Adicionar nós virtuais a partir de conexões que não existem em 'nodes'
  props.edges.forEach(e => {
    const s = e.source.id || e.source
    const t = e.target.id || e.target
    
    if (!nodesMap.has(s)) nodesMap.set(s, { id: s, name: s, virtual: true })
    if (!nodesMap.has(t)) nodesMap.set(t, { id: t, name: t, virtual: true })
  })

  const links = props.edges.map(e => ({
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
    .nodeRelSize(1) // Escala base 1 para usar as geometrias customizadas precisamente
    .nodeOpacity(0.9)
    .linkColor(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      
      // Se o link está na trilha de raciocínio, acende em verde néon
      if (highlightedLinks.value.has(`${s}-${t}`) || highlightedLinks.value.has(`${t}-${s}`)) {
        return '#4ade80' 
      }
      return 'rgba(59, 130, 246, 0.3)'
    })
    .linkWidth(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      if (highlightedLinks.value.has(`${s}-${t}`) || highlightedLinks.value.has(`${t}-${s}`)) {
        return 2.5 // Espessura dupla para destaque
      }
      return 0.5
    })
    .linkDirectionalParticles(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      return (highlightedLinks.value.has(`${s}-${t}`) || highlightedLinks.value.has(`${t}-${s}`)) ? 5 : 1
    })
    .linkDirectionalParticleSpeed(0.005)
    .linkDirectionalParticleWidth(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      return (highlightedLinks.value.has(`${s}-${t}`) || highlightedLinks.value.has(`${t}-${s}`)) ? 3 : 1
    })
    .onNodeClick(async node => {
      // Zoom no nó ao clicar
      const distance = 60
      const distRatio = 1 + distance/Math.hypot(node.x, node.y, node.z)
      Graph.cameraPosition(
        { x: node.x * distRatio, y: node.y * distRatio, z: node.z * distRatio }, 
        node, 
        2000
      )

      // Carregar Proveniência (Auditoria)
      selectedNode.value = node
      try {
        const details = await window.go.main.App.GetNodeDetails(node.id)
        nodeDetails.value = details
      } catch (e) {
        console.error("Erro ao buscar detalhes:", e)
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
  }).strength(1)

  Graph.nodeThreeObject(node => {
    const isVirtual = node.virtual
    const isActive = node.id === props.activeNode
    
    // Lógica consolidada de tipos e status
    const type = node['status'] === 'legacy' ? 'legacy' : (node['status'] === 'conflict' ? 'conflict' : (node['document-type'] || node['document_type'] || 'chunk'))
    
    const colors = {
      source: '#a855f7', // Roxo (Original)
      page: '#22d3ee',   // Ciano (Página)
      chunk: '#3b82f6',  // Azul Neon
      system: '#f1f5f9', // Platina (Sistema)
      memory: '#f472b6', // Rosa (Sinapse de Chat)
      legacy: '#475569', // Cinza (Inativo/Legado)
      conflict: '#ef4444', // Vermelho (Contradição/Alerta)
      virtual: '#1e3a8a',// Azul Escuro (Fantasma)
      active: '#fcd34d'  // Ouro (Foco)
    }

    // Se o nó estiver em conflito ou for legado, sobrepõe a cor
    const displayColor = node.status === 'conflict' ? colors.conflict : (node.status === 'legacy' ? colors.legacy : (colors[type] || colors.chunk))
    const nodeColor = isActive ? colors.active : (isVirtual ? colors.virtual : displayColor)
    
    // Esferas com tamanhos diferentes por importância (NÚCLEOS vs IDEIAS)
    let radius = 1.2 // Tamanho base para Ideias
    if (isActive) radius = 5.0
    else if (type === 'chunk' || type === 'system') radius = 4.0 // Os "Sóis" do conhecimento
    else if (type === 'source') radius = 3.0
    else if (isVirtual) radius = 0.8

    const geometry = new THREE.SphereGeometry(radius)
    const material = new THREE.MeshStandardMaterial({
      color: nodeColor,
      transparent: true,
      opacity: type === 'legacy' ? 0.3 : (isVirtual ? 0.3 : 0.9),
      metalness: 0.8,
      roughness: 0.1
    })
    
    const mesh = new THREE.Mesh(geometry, material)
    
    // Brilho Neon (Emissivo)
    if (!isVirtual) {
       mesh.material.emissive = new THREE.Color(nodeColor)
       mesh.material.emissiveIntensity = isActive ? 1.5 : 0.6
    }

    return mesh
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
onMounted(() => {
  initGraph()
  
  // Listener de Conflitos do Agente Validador
  window.runtime.EventsOn("graph:conflict", (conflict) => {
    currentConflict.value = conflict
    console.warn("⚠️ CONFLITO DETECTADO:", conflict)
  })

  // Resize handler
  window.addEventListener('resize', () => {
    if (Graph && containerRef.value) {
      Graph.width(containerRef.value.clientWidth)
      Graph.height(containerRef.value.clientHeight)
    }
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

const resetZoom = () => {
  if (Graph) Graph.zoomToFit(800)
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
    }
}
</script>

<template>
  <div class="graph-wrapper animate-fade-in">
    <!-- Container para o Grafo 3D (WebGL) -->
    <div ref="containerRef" class="main-canvas"></div>
    
    <!-- Controles & Console de Logs (Painel de Pensamento Vidrado) -->
    <div class="graph-ui glass">
      <div class="ui-header">
        <span class="pulse"></span>
        <h3>Conhecimento Obsidian 3D</h3>
      </div>
      
      <div class="ui-actions">
        <div style="display: flex; gap: 8px;">
          <button @click="resetZoom" class="action-btn" title="Centralizar">🎯 <span>RESET</span></button>
          <button @click="triggerScan" class="action-btn" :class="{'scanning-btn': scanning}" title="Forçar Index"><span v-if="!scanning">🔄</span><span v-else class="spin">⏳</span><span>SCAN</span></button>
        </div>
        <div class="stat-item">
          <span class="val">{{ nodes.length }}</span>
          <span class="lab">NOTAS</span>
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
          <div v-if="nodeDetails" class="details-content">
            <div class="info-group">
              <label>DOCUMENTO ORIGEM</label>
              <p class="source-path">{{ nodeDetails.path || "Memória de Chat" }}</p>
            </div>

            <div class="info-group">
              <label>TRECHO FUNDAMENTADO (CHUNK)</label>
              <div class="content-preview">
                {{ nodeDetails.content || "Fato atomizado sem conteúdo textual." }}
              </div>
            </div>

            <button v-if="nodeDetails.path" @click="openSource" class="btn-open-source">
              ABRIR ARQUIVO FONTE ✨
            </button>
          </div>
          <div v-else class="loading-provenance">
            <span>Buscando linhagem...</span>
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
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
}

.conflict-modal {
  padding: 2rem;
  border-radius: 24px;
  border: 1px solid rgba(239, 68, 68, 0.3);
  max-width: 450px;
  width: 90%;
  text-align: center;
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
}

.ui-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1.5rem;
}

.action-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: white;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 0.6rem;
  font-weight: 800;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  transition: all 0.3s;
}

.action-btn:hover {
  background: var(--primary);
  border-color: var(--primary);
  transform: translateY(-2px);
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
</style>
