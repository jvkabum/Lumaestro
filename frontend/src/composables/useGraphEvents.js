import * as THREE from 'three'
import { useGraphStore } from '../stores/graph'
import { useOrchestratorStore } from '../stores/orchestrator'

/**
 * 🌐 useGraphEvents — Eventos Wails & Streaming do Grafo
 * 
 * Responsável por:
 * - EventsOn('graph:highlight') → Trail de RAG (partículas brilhantes nas arestas)
 * - EventsOn('graph:conflict') → Pop-up de conflitos semânticos
 * - EventsOn('graph:health:update') → Sincronização automática de saúde
 * - EventsOn('graph:edge') → Streaming de arestas dinâmicas (reforço sináptico)
 * - EventsOn('graph:traverse') → Percurso cinematográfico da IA (hop a hop)
 * - Resize handler para responsividade
 * - Resolução de conflitos (agente validador)
 */
export function useGraphEvents() {
  const store = useGraphStore()
  const orchestrator = useOrchestratorStore()

  /**
   * Registra todos os listeners de eventos do Wails no grafo
   * @param {Ref} containerRef - ref do container DOM
   */
  const registerGraphEvents = (containerRef) => {
    const Graph = store.graphInstance

    // 🌟 Destaques de Trajetória (Context-Flow inspirado no TrustGraph)
    window.runtime.EventsOn('graph:highlight', (linkData) => {
      const linkId1 = `${linkData.source}-${linkData.target}`
      const linkId2 = `${linkData.target}-${linkData.source}`
      
      store.highlightedLinks.add(linkId1)
      store.highlightedLinks.add(linkId2)
      
      // Forçar atualização visual das arestas no motor Three.js
      if (Graph) {
        Graph.linkColor(Graph.linkColor())
        Graph.linkWidth(Graph.linkWidth())
      }

      // Efeito de Rastro: O brilho desaparece após 4 segundos (Cinemático)
      setTimeout(() => {
        store.highlightedLinks.delete(linkId1)
        store.highlightedLinks.delete(linkId2)
        if (Graph) {
          Graph.linkColor(Graph.linkColor())
          Graph.linkWidth(Graph.linkWidth())
        }
      }, 4000)
    })

    // ⚠️ Listener de Conflitos do Agente Validador
    window.runtime.EventsOn("graph:conflict", (conflict) => {
      store.currentConflict = conflict
      console.warn("⚠️ CONFLITO DETECTADO:", conflict)
    })

    // 🪐 Sincronização de Saúde (Automática após Sync)
    window.runtime.EventsOn("graph:health:update", (stats) => {
      store.graphHealth = stats
    })

    // 🕸️ Ouvinte de Arestas Dinâmicas (Batched — evita restart de simulação por aresta)
    let edgeBatchTimeout = null
    const pendingEdges = []

    const flushPendingEdges = () => {
      if (!Graph || pendingEdges.length === 0) return
      const { nodes, links } = Graph.graphData()
      let dataChanged = false

      for (const edge of pendingEdges) {
        if (!edge?.source || !edge?.target) continue

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

        const exists = links.find(l => 
          ((l.source.id || l.source) === edge.source && (l.target.id || l.target) === edge.target) || 
          ((l.source.id || l.source) === edge.target && (l.target.id || l.target) === edge.source)
        )

        if (!exists) {
          links.push({ source: edge.source, target: edge.target, weight: edge.weight || 1 })
          dataChanged = true
        } else {
          exists.weight = (exists.weight || 1) + (edge.weight || 1)
          dataChanged = true
        }
      }

      pendingEdges.length = 0 // Limpa o buffer

      if (dataChanged) {
        Graph.graphData({ nodes, links })
      }
    }

    window.runtime.EventsOn("graph:edge", (edge) => {
      pendingEdges.push(edge)
      if (!edgeBatchTimeout) {
        edgeBatchTimeout = setTimeout(() => {
          flushPendingEdges()
          edgeBatchTimeout = null
        }, 300) // Acumula por 300ms antes de aplicar
      }
    })

    // 🎬 Percurso Cinematográfico da IA: Anima cada hop individualmente com delay
    window.runtime.EventsOn("graph:traverse", (data) => {
      if (!Graph || !data?.hops?.length) return

      orchestrator.isNavigating = true
      const hops = data.hops
      const HOPDelay = 800

      hops.forEach((hop, i) => {
        setTimeout(() => {
          const { nodes } = Graph.graphData()

          // 1. Voa a câmera até o nó DESTINO (To)
          const targetNode = nodes.find(n => n.id === hop.to || n.name === hop.to)
          if (targetNode) {
            Graph.cameraPosition(
              { x: targetNode.x + 180, y: targetNode.y + 120, z: targetNode.z + 180 }, // Offset aumentado para zoom mais suave
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
          store.clickedNodeLinks.add(linkKey1)
          store.clickedNodeLinks.add(linkKey2)
          Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())

          // 4. Remove a partícula deste hop após 3s
          setTimeout(() => {
            store.clickedNodeLinks.delete(linkKey1)
            store.clickedNodeLinks.delete(linkKey2)
            Graph.linkDirectionalParticles(Graph.linkDirectionalParticles())
          }, 3000)

          // 5. Marca como "fim da travessia" no último hop
          if (i === hops.length - 1) {
            setTimeout(() => { orchestrator.isNavigating = false }, 1500)
          }
        }, i * HOPDelay)
      })
    })

    // 📐 Resize handler
    const handleResize = () => {
      if (Graph && containerRef.value) {
        Graph.width(containerRef.value.clientWidth)
        Graph.height(containerRef.value.clientHeight)
      }
    }
    window.addEventListener('resize', handleResize)

    // Retorna cleanup (para onUnmounted)
    return () => {
      window.removeEventListener('resize', handleResize)
    }
  }

  /**
   * Resolve um conflito semântico com a decisão do usuário
   */
  const resolveConflict = async (decision) => {
    if (!store.currentConflict) return
    
    const c = store.currentConflict
    console.log("Resolvendo conflito com decisão:", decision)
    
    // 🚀 Feedback visual imediato: fecha o modal antes da chamada de rede
    store.currentConflict = null

    try {
      // Tenta chamar no pacote modular 'core' (novo) com fallback para 'main' (legado)
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      
      if (bridge && bridge.ResolveConflict) {
        await bridge.ResolveConflict(
          decision, 
          c.subject, 
          c.predicate, 
          c.old_id, 
          c.new, 
          c.session_id
        )
      } else {
        console.error("Função ResolveConflict não encontrada na bridge Wails");
      }
    } catch (err) {
      console.error("Falha ao resolver conflito no backend:", err)
      // Se falhar drasticamente, poderíamos restaurar o conflito para o usuário tentar denovo
      // store.currentConflict = c
    }
  }

  return { registerGraphEvents, resolveConflict }
}
