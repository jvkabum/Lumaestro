import { useGraphStore } from '../../../stores/graph'
import { toRaw } from 'vue'

/**
 * 🧪 DataTransformer — O Refinador de Malha
 * 
 * Responsável por filtrar e transformar os dados brutos do grafo
 * antes de serem entregues ao motor Deck.gl. 
 * Implementa X-Ray, Modo Esqueleto e filtragem de nós virtuais.
 */
export function useDataTransformer() {
  const store = useGraphStore()

  /**
   * Filtra e prepara os dados baseados no estado da UI (X-Ray, Skeletal)
   */
  const transform = (nodes, edges) => {
    let filteredNodes = [...nodes]
    let filteredLinks = [...edges]

    // 1. Filtro X-Ray (PageRank Threshold)
    if (store.xRayThreshold > 0) {
      filteredNodes = filteredNodes.filter(n => {
        // Notas de origem (source/obsidian) e Sistemas são SEMPRE visíveis
        const type = n['document-type'] || 'chunk'
        if (type === 'source' || type === 'system' || type === 'obsidian') return true
        
        const pr = n.pagerank || 0
        return pr >= store.xRayThreshold
      })
    }

    // 2. Modo Esqueleto (Oculta nós virtuais sem conexões manuais)
    if (store.skeletalMode) {
      filteredNodes = filteredNodes.filter(n => !n.virtual)
    }

    // 3. Sincronização de Links (Remove links órfãos após filtragem de nós)
    const nodeIds = new Set(filteredNodes.map(n => String(n.id)))
    filteredLinks = filteredLinks.filter(l => {
        const sid = typeof l.source === 'object' ? String(l.source.id) : String(l.source)
        const tid = typeof l.target === 'object' ? String(l.target.id) : String(l.target)
        return nodeIds.has(sid) && nodeIds.has(tid)
    })

    return { 
        nodes: filteredNodes, 
        links: filteredLinks 
    }
  }

  return { transform }
}
