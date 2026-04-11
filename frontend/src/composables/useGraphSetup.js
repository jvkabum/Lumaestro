import * as THREE from 'three'
import * as d3 from 'd3'
import ForceGraph3D from '3d-force-graph'
import { useGraphStore } from '../stores/graph'
import { useGraphData } from './useGraphData'
import { toRaw } from 'vue'

/**
 * 🚀 useGraphSetup — Motor de Inicialização do Grafo 3D
 * 
 * Responsável por:
 * - Pooling de geometrias THREE.js (SphereGeometry reutilizáveis)
 * - Cache de materiais por cor/opacidade/emissão
 * - Criação e configuração completa do ForceGraph3D
 * - Física solar (nucleação e órbitas)
 * - Renderização customizada de nós (nodeThreeObject)
 */
export function useGraphSetup() {
  const store = useGraphStore()
  const { getGraphData } = useGraphData()

  const escapeHtml = (value) => String(value || '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')

  const clamp = (value, limit = 220) => {
    const text = String(value || '').trim()
    if (text.length <= limit) return text
    return text.slice(0, limit - 3).trim() + '...'
  }

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

  /**
   * Inicializa o grafo 3D no container DOM fornecido
   */
  const initGraph = (containerRef, nodes, edges, activeNode) => {
    if (!containerRef) return

    // Inicialização sob demanda das geometrias
    if (!sphereLowRes) sphereLowRes = new THREE.SphereGeometry(12, 8, 8)
    if (!sphereVirtual) sphereVirtual = new THREE.SphereGeometry(8, 6, 6)
    if (!sphereActive) sphereActive = new THREE.SphereGeometry(20, 12, 12)

    // 🛡️ Prevenção de 'Blackout': Garantir dimensões reais antes de instanciar
    if (containerRef.clientWidth === 0 || containerRef.clientHeight === 0) {
        console.warn("[NeuralGraph] Container sem dimensões. Tentando redimensionamento forçado.")
        containerRef.style.width = '100%'
        containerRef.style.height = '100%'
    }

    // Variáveis locais para cache de física (NUNCA usar 'store' aqui para evitar Vue Proxies que destroem a CPU)
    let tempCharge = null;
    let tempCollide = null;

    const Graph = ForceGraph3D()(containerRef)
      .graphData(getGraphData(nodes, edges))
      .width(containerRef.clientWidth || 800)
      .height(containerRef.clientHeight || 600)
      .backgroundColor('#050505') // Levemente mais claro para diagnosticar se o canvas existe
      .showNavInfo(false)
      .nodeLabel(node => {
        const type = node['document-type'] || 'chunk'
        const label = node.name || node.id
        const icon = type === 'system' ? '⚙️' : (type === 'source' ? '📄' : (type === 'memory' ? '🧠' : '📝'))
        const summary = clamp(node.summary || node.content || 'Sem resumo disponível.')
        const purpose = clamp(node['what-it-does'] || node.purpose || 'Usado pelo RAG para recuperação de contexto semântico.', 180)

        return `<div class="node-tooltip">
                  <span class="type-tag ${type}">${icon} ${type.toUpperCase()}</span>
                  <br/><b>${label}</b>
                  <br/><div style="margin-top:6px; max-width: 340px; line-height:1.35; color:#e2e8f0;"><b>Resumo:</b> ${escapeHtml(summary)}</div>
                  <div style="margin-top:4px; max-width: 340px; line-height:1.35; color:#cbd5e1;"><b>O que faz:</b> ${escapeHtml(purpose)}</div>
                </div>`
      })
      .nodeColor(node => {
        const type = node['document-type'] || 'chunk'
        if (type === 'system') return '#ffffff'
        if (type === 'source' || type === 'obsidian') return '#00f2ff' // Ciano Néon
        if (type === 'memory') return '#fcd34d' // Dourado
        return '#3b82f6' // Azul padrão
      })
      .nodeRelSize(8) // Tamanho de presença aumentado para evitar aspecto de 'poeira'
      .nodeOpacity(1)
      .nodeThreeObjectExtend(true)
      .linkCurvature(0.25)
      .linkColor(link => {
        const s = link.source.id || link.source
        const t = link.target.id || link.target
        const clickedLinks = toRaw(store.clickedNodeLinks)
        if (clickedLinks.has(`${s}-${t}`) || clickedLinks.has(`${t}-${s}`)) return '#ffffff' 
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
        const clickedLinks = toRaw(store.clickedNodeLinks)
        const hlLinks = toRaw(store.highlightedLinks)
        if (clickedLinks.has(`${s}-${t}`) || clickedLinks.has(`${t}-${s}`)) return 4
        if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return 2
        return 0 
      })
      .linkDirectionalParticleSpeed(0.006)
      .linkDirectionalParticleWidth(link => {
        const s = link.source.id || link.source
        const t = link.target.id || link.target
        const clickedLinks = toRaw(store.clickedNodeLinks)
        if (clickedLinks.has(`${s}-${t}`) || clickedLinks.has(`${t}-${s}`)) return 2.5
        return 1.5
      })
      .onNodeClick(node => focusNode(node))
      // Delega o arrasto para o 3d-force-graph nativo, que já faz o pinning (node.fx) fluidamente sem recriar as Forças 60x por segundo.

    // 🚀 CONFIGURAÇÃO DE FÍSICA SOLAR (NUCLEAÇÃO E ÓRBITAS EXPANDIDAS)
    Graph.d3Force('charge').strength(node => {
        // Repulsão Dinâmica calibrada para 765+ nós (reduzida 3x para convergência rápida)
        const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (node.degree || 0)
        const baseRepulsion = -400
        return baseRepulsion - (importance * 60)
    })

    Graph.d3Force('link').distance(link => {
        const sType = link.source['document-type'] || 'chunk'
        const tType = link.target['document-type'] || 'chunk'
        
        // Órbita Próxima: Ideias conectadas a Notas (80px)
        if (sType === 'memory' || tType === 'memory') return 80
        
        // Distância Interestelar Expandida: Notas conectadas entre si (350px)
        // Aumentado de 150 para 350 para dar 'respiro' ao conhecimento denso
        return 350
    }).strength(0.7)

    // Força de Colisão: Impede que as esferas se sobreponham fisicamente
    Graph.d3Force('collide', d3.forceCollide(node => {
        const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (node.degree || 0)
        return (1 + Math.pow(importance, 0.5) * 4) + 10 // Raio de colisão + margem
    }))

    // Otimização de CPU: Convergência rápida para 765+ nós
    Graph.d3AlphaDecay(0.08)       // 2x mais rápido para esfriar
    Graph.d3VelocityDecay(0.45)    // Mais amortecimento = menos oscilação
    Graph.warmupTicks(100)         // Layout inicial: 100 ticks síncronos
    Graph.cooldownTicks(300)       // Para a simulação após 300 ticks
    // 📏 VOLUME / TAMANHO DOS NÓS: Restaura o InstancedMesh (1 Draw Call na GPU vs 700+)
    Graph.nodeVal(node => {
      const isActive = node.id === activeNode
      const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (node.degree || 0)
      const baseScale = 1 + Math.pow(importance, 0.5) * 0.4
      const finalScale = isActive ? baseScale * 1.5 : baseScale
      // Retorna o volume (Raio ao cubo) para a GPU dimensionar nativamente
      return Math.pow(finalScale, 3) 
    })
    Graph.nodeResolution(12) // Mantém esferas redondinhas para todo mundo

    // Salva a instância na store
    store.graphInstance = Graph

    // 💫 ESTÉTICA PREMIUM: Rotação e Controles
    Graph.controls().autoRotate = true
    Graph.controls().autoRotateSpeed = 0.5
    Graph.controls().enableDamping = true
    Graph.controls().dampingFactor = 0.1

    // Log de Integridade de Cena
    setTimeout(() => {
        console.log(`[NeuralGraph] Objetos na Cena: ${Graph.scene().children.length}`)
    }, 2000)

    // 📐 Monitor de Redimensionamento Reativo (Robustez para Wails/Flex)
    const resizeObserver = new ResizeObserver(() => {
        if (containerRef.clientWidth > 0 && containerRef.clientHeight > 0) {
            Graph.width(containerRef.clientWidth)
            Graph.height(containerRef.clientHeight)
            console.log(`[NeuralGraph] Resized: ${containerRef.clientWidth}x${containerRef.clientHeight}`)
        }
    })
    resizeObserver.observe(containerRef)

    // 🎯 AUTO-ZOOM INTELIGENTE (REATIVO AO CRESCIMENTO)
    let lastNodeCount = 0
    let firstZoomDone = false

    let tickCounter = 0
    Graph.onEngineTick(() => {
        tickCounter++
        // Throttle: Só verifica a cada 60 ticks (~1s) em vez de cada frame
        if (tickCounter % 60 !== 0) return
        const currentCount = Graph.graphData().nodes.length
        if (currentCount > 0 && (currentCount > lastNodeCount + 50 || (!firstZoomDone && currentCount > 0))) {
            console.log(`[NeuralGraph] Crescimento detectado (${currentCount} nós). Re-enquadrando visao...`)
            Graph.zoomToFit(1200, 300)
            lastNodeCount = currentCount
            firstZoomDone = true
        }
    })

    // Fallback de Zoom (caso a simulação demore muito para estabilizar)
    setTimeout(() => {
        if (!firstZoomDone && Graph.graphData().nodes.length > 0) {
            Graph.zoomToFit(800, 150)
            firstZoomDone = true
        }
    }, 5000)

    return Graph
  }

  /**
   * 🎯 focusNode — Centraliza, dá zoom e abre detalhes de um nó programaticamente
   */
  const focusNode = async (node) => {
    const Graph = store.graphInstance
    if (!Graph || !node) return

    // ── Zoom no nó ──
    const distance = 250 // Aumentado de 80 para 250 para evitar zoom 'colado' no nó
    const distRatio = 1 + distance / Math.hypot(node.x || 1, node.y || 1, node.z || 1)
    
    Graph.cameraPosition(
      { 
        x: (node.x || 0) * distRatio, 
        y: (node.y || 0) * distRatio, 
        z: (node.z || 0) * distRatio 
      }, 
      node, 
      2000
    )

    // ── Animação de partículas nos links conectados ──
    if (store._clickedNodeTimeout) clearTimeout(store._clickedNodeTimeout)

    const { links } = Graph.graphData()
    store.clickedNodeLinks.clear()
    links.forEach(link => {
      const s = link.source.id || link.source
      const t = link.target.id || link.target
      if (s === node.id || t === node.id) {
        store.clickedNodeLinks.add(`${s}-${t}`)
        store.clickedNodeLinks.add(`${t}-${s}`)
      }
    })

    Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
    store._clickedNodeTimeout = setTimeout(() => {
      store.clickedNodeLinks.clear()
      Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
    }, 5000)

    // Bridge Wails
    const bridge = (window.go && window.go.core && window.go.core.App) || 
                   (window.go && window.go.main && window.go.main.App);

    if (bridge && bridge.HandleNodeClick) {
      bridge.HandleNodeClick(node.id)
    }

    store.selectedNode = node
    store.nodeDetails = { loading: true }
    
    try {
      let details = null;
      if (bridge && bridge.GetNodeDetails) {
         details = await bridge.GetNodeDetails(node.id)
      }

      if (details) {
        store.nodeDetails = details
      } else {
        store.nodeDetails = {
          path: node.path || "Conceito Neural",
          content: node.content || `O nó '${node.id}' é uma ponte lógica criada pela IA para conectar suas ideias.`,
          source: "Inteligência Artificial",
          isVirtual: true
        }
      }
    } catch (e) {
      console.error("Erro ao buscar detalhes:", e)
      store.nodeDetails = {
        path: "Erro de Sincronização",
        content: `Não foi possível recuperar os detalhes do nó '${node.id}'.`,
        source: "Sistema",
        isVirtual: true
      }
    }
  }

  return { initGraph, getCachedMaterial, focusNode }
}
