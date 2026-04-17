import { useGraphStore } from '../../../stores/graph'
import { useOrchestratorStore } from '../../../stores/orchestrator'

/**
 * 🌉 BridgeDriver — O Diplomata do Multiverso
 * 
 * Responsável por escutar os eventos do backend (Wails) e disparar 
 * as ações correspondentes no motor gráfico Deck.gl.
 */
export function useBridgeDriver() {
  const store = useGraphStore()
  const orchestrator = useOrchestratorStore()

  const registerGraphEvents = ({ focusNode, updateGraph }) => {
    
    // 1. Destaques de Trajetória (Trail)
    window.runtime.EventsOn('graph:highlight', (linkData) => {
      const linkId1 = `${linkData.source}-${linkData.target}`
      const linkId2 = `${linkData.target}-${linkData.source}`
      store.highlightedLinks.add(linkId1)
      store.highlightedLinks.add(linkId2)
      
      setTimeout(() => {
        store.highlightedLinks.delete(linkId1)
        store.highlightedLinks.delete(linkId2)
      }, 4000)
    })

    // 2. Conflitos Semânticos
    window.runtime.EventsOn("graph:conflict", (conflict) => {
      store.currentConflict = conflict
    })

    // 3. Saúde do Grafo
    window.runtime.EventsOn("graph:health:update", (stats) => {
      store.graphHealth = stats
    })

    // 4. Percurso Cinematográfico (Traverse)
    window.runtime.EventsOn("graph:traverse", (data) => {
      if (!data?.hops?.length) return
      orchestrator.isNavigating = true
      
      const HOPDelay = 800
      data.hops.forEach((hop, i) => {
        setTimeout(() => {
          // Foca no destino usando o Pilot
          focusNode(hop.to)

          const linkKey1 = `${hop.from}-${hop.to}`
          const linkKey2 = `${hop.to}-${hop.from}`
          store.clickedNodeLinks.add(linkKey1)
          store.clickedNodeLinks.add(linkKey2)

          setTimeout(() => {
            store.clickedNodeLinks.delete(linkKey1)
            store.clickedNodeLinks.delete(linkKey2)
          }, 3000)

          if (i === data.hops.length - 1) {
            setTimeout(() => { orchestrator.isNavigating = false }, 1500)
          }
        }, i * HOPDelay)
      })
    })

    // 🕸️ Streaming de Arestas (Batched)
    let edgeBatchTimeout = null
    const pendingEdges = []
    
    // NOTA: O streaming direto de dados alterou na versão Deck.gl para 
    // fluir via props do componente pai, mas mantemos o listener para compatibilidade
    window.runtime.EventsOn("graph:edge", (edge) => {
       console.log("[Bridge] Streaming de aresta detectado (Re-sync necessário):", edge)
    })

    return () => {
      // Cleanup de eventos Wails aqui se necessário
    }
  }

  /**
   * Resolve um conflito semântico com a decisão do usuário
   */
  const resolveConflict = async (decision) => {
    if (!store.currentConflict) return
    const c = store.currentConflict
    store.currentConflict = null

    try {
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      
      if (bridge && bridge.ResolveConflict) {
        await bridge.ResolveConflict(
          decision, c.subject, c.predicate, c.old_id, c.new, c.session_id
        )
      }
    } catch (err) {
      console.error("[Bridge] Falha ao resolver conflito:", err)
    }
  }

  return { registerGraphEvents, resolveConflict }
}
