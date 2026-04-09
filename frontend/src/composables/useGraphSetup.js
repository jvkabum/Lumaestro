import * as THREE from 'three'
import ForceGraph3D from '3d-force-graph'
import { useGraphStore } from '../stores/graph'
import { useGraphData } from './useGraphData'

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

    const Graph = ForceGraph3D()(containerRef)
      .graphData(getGraphData(nodes, edges))
      .backgroundColor('#09090b') 
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
      .nodeRelSize(6) // Tamanho de colisão aumentado
      .nodeOpacity(1)
      .nodeThreeObjectExtend(true)
      .linkCurvature(0.25)
      .linkColor(link => {
        const s = link.source.id || link.source
        const t = link.target.id || link.target
        
        if (store.clickedNodeLinks.has(`${s}-${t}`) || store.clickedNodeLinks.has(`${t}-${s}`)) return '#ffffff' 
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
        if (store.clickedNodeLinks.has(`${s}-${t}`) || store.clickedNodeLinks.has(`${t}-${s}`)) return 4
        if (store.highlightedLinks.has(`${s}-${t}`) || store.highlightedLinks.has(`${t}-${s}`)) return 2
        return 0 
      })
      .linkDirectionalParticleSpeed(0.006)
      .linkDirectionalParticleWidth(link => {
        const s = link.source.id || link.source
        const t = link.target.id || link.target
        if (store.clickedNodeLinks.has(`${s}-${t}`) || store.clickedNodeLinks.has(`${t}-${s}`)) return 2.5
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
        if (store._clickedNodeTimeout) clearTimeout(store._clickedNodeTimeout)

        // Encontra todos os links conectados a este nó
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

        // Força refresh das partículas
        Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())

        // Apaga automaticamente após 5 segundos
        store._clickedNodeTimeout = setTimeout(() => {
          store.clickedNodeLinks.clear()
          Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
        }, 5000)

        // ── Reforço Neural (Aprendizado Ativo) ──
        window.go.main.App.HandleNodeClick(node.id)

        store.selectedNode = node
        store.nodeDetails = null
        store.nodeDetails = { loading: true }
        
        try {
          const details = await window.go.main.App.GetNodeDetails(node.id)
          if (details) {
            store.nodeDetails = details
          } else {
            store.nodeDetails = {
              path: "Conceito Neural",
              content: `O nó '${node.id}' é uma ponte lógica criada pela IA para conectar suas ideias. Ele não possui um arquivo físico, mas serve como âncora semântica no seu grafo.`,
              source: "Inteligência Artificial",
              isVirtual: true
            }
          }
        } catch (e) {
          console.error("Erro ao buscar detalhes:", e)
          store.nodeDetails = {
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
      const isActive = node.id === activeNode
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

    // Salva a instância na store
    store.graphInstance = Graph
    return Graph
  }

  return { initGraph, getCachedMaterial }
}
