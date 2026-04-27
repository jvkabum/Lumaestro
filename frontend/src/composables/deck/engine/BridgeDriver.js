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

    // ✨ 4. EFEITO DE DESCOBERTA (node:active)
    // Disparado quando o RAG/IA identifica o neurônio mais relevante para a conversa.
    let discoveryAbort = null // 🛡️ Token de cancelamento para zoom anterior
    
    window.runtime.EventsOn("node:active", (nodeId) => {
      if (!nodeId) return
      
      // 🛡️ CANCELAMENTO: Se há um zoom anterior em progresso, aborta antes de iniciar o novo
      if (discoveryAbort) {
        discoveryAbort.cancelled = true
        console.log(`[Efeito de Descoberta] 🚫 Zoom anterior cancelado (novo alvo recebido)`)
      }
      const abortToken = { cancelled: false }
      discoveryAbort = abortToken
      
      // Limpeza de ID (remove aspas se o backend enviou com %q)
      const cleanId = String(nodeId).replace(/^["']|["']$/g, '').trim()
      console.log(`[Efeito de Descoberta] 🧠 RAG identificou: "${cleanId}" (Bruto: "${nodeId}")`)
      
      // 📡 Feedback: Indica que a busca está em andamento
      store.discoveryStatus = 'searching'
      console.log(`[BridgeDriver] 📡 Status: SEARCHING para "${cleanId}"`)

      // Delay inicial curto para o debounce de renderização (150ms) agir
      setTimeout(() => {
        if (abortToken.cancelled) return
        
        const tryDiscoveryFocus = (attempt = 1) => {
          // 🛡️ Verifica cancelamento antes de cada tentativa
          if (abortToken.cancelled) {
            console.log(`[Efeito de Descoberta] 🚫 Tentativa ${attempt} abortada (novo alvo ativo)`)
            return
          }
          
          // Usa a versão robusta (ID original ou fuzzy-match)
          const node = store.graphInstance?.focusNodeById(cleanId)
          
          if (node) {
            console.log(`[Efeito de Descoberta] ✅ No "${cleanId}" ENCONTRADO na tentativa ${attempt}! Focando...`)
            store.discoveryStatus = 'found'
            
            // Limpa o status após 3s
            setTimeout(() => { 
              if (store.discoveryStatus === 'found') store.discoveryStatus = null 
            }, 3000)
            
            // 📖 Aciona a Anatomia do Nó (Abre o painel de Proveniência)
            store.selectedNode = node
            store.nodeDetails = { loading: true, path: '', content: '', isVirtual: false }

            const bridge = (window.go?.core?.App) || (window.go?.main?.App)
            if (bridge && bridge.GetNeuralNodeContext) {
              bridge.GetNeuralNodeContext(nodeId).then(res => {
                if (abortToken.cancelled) return // Não atualiza se já foi cancelado
                if (res && res.success !== false) {
                  store.nodeDetails = {
                    loading: false,
                    path: res.path || 'Memória Virtual',
                    content: res.content || res.summary || 'Sem metadados',
                    isVirtual: res.document_type === 'memory',
                    semanticNeighbors: res.semantic_neighbors || []
                  }
                  
                  // Destaque de conexões
                  store.highlightedLinks.clear()
                  store.clickedNodeLinks.clear()
                  if (res.related_edges) {
                    res.related_edges.forEach(edgeId => store.clickedNodeLinks.add(edgeId))
                  }
                }
              }).catch(e => console.error("[Efeito de Descoberta] Erro ao buscar contexto:", e))
            }
          } else if (attempt < 8) {
            // Se o batch de 5000 arestas for muito grande, o motor pode levar alguns segundos adicionais.
            const nextDelay = attempt * 1200
            console.warn(`[Efeito de Descoberta] ⚠️ Nó "${cleanId}" ainda não visível no Deck.gl. Tentativa ${attempt}/8... (Aguardando ${nextDelay}ms)`)
            setTimeout(() => tryDiscoveryFocus(attempt + 1), nextDelay)
          } else {
            console.error(`[Efeito de Descoberta] ❌ Nó "${cleanId}" não localizado no motor gráfico após 8 tentativas críticas. Verifique se o ID existe no nodeMap.`)
            store.discoveryStatus = 'failed'
            // Limpa o status de falha após 5s
            setTimeout(() => { 
              if (store.discoveryStatus === 'failed') store.discoveryStatus = null 
            }, 5000)
          }
        }

        tryDiscoveryFocus()
      }, 500)
    })

    // 🎬 [Mixer] Zoom Cinematográfico Manual (Eventos disparados pela UI Vue/Chat)
    window.addEventListener("cinematic:zoom", (e) => {
      const nodeId = e.detail;
      if (!nodeId) return;
      console.log("[Bridge] 🎬 Sinal de Zoom Manual (Vue) recebido para:", nodeId);
      // Usa o motor robusto para localizar e focar
      store.graphInstance?.focusNodeById(nodeId);
    });

    // 5. Percurso Cinematográfico (Traverse) — Animação hop-by-hop
    window.runtime.EventsOn("graph:traverse", (data) => {
      if (!data?.hops?.length) return
      orchestrator.isNavigating = true
      
      const HOPDelay = 800
      data.hops.forEach((hop, i) => {
        setTimeout(() => {
          // Usa focusNodeById para resolver string → objeto com coordenadas
          store.graphInstance?.focusNodeById(hop.to)

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
    
    // ☢️ RESET TOTAL: Limpa a tela imediatamente ao receber o sinal do backend
    window.runtime.EventsOn("graph:clear", () => {
       console.log("[Bridge] ☢️ Sinal de Reset Total recebido. Limpando Grafo...");
       store.clearGraph();
    });

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
