import { useGraphStore } from '../stores/graph'

/**
 * 🔄 useGraphData — Transformação de Dados do Grafo
 * 
 * Converte os dados brutos (nodes/edges) para o formato esperado pelo 3d-force-graph,
 * aplicando filtragem X-Ray, criação de nós virtuais e modo esqueletal (MST).
 */
export function useGraphData() {
  const store = useGraphStore()

  /**
   * Converte os dados para o formato do 3d-force-graph 
   * (incluindo nós virtuais e filtragem X-Ray)
   */
  const getGraphData = (nodes, edges) => {
    const nodesMap = new Map()
    
    // 1. Adicionar nós reais (Filtrando por PageRank se X-Ray ativo)
    nodes.forEach(n => {
      const pr = n.pagerank || 0
      // O X-Ray só filtra se o threshold for > 0
      if (store.xRayThreshold === 0 || pr >= store.xRayThreshold || n.type === 'source' || n.type === 'system') {
        nodesMap.set(n.id, { ...n })
      }
    })

    // 2. Adicionar nós virtuais a partir de conexões que não existem em 'nodes'
    // (Somente se os destinos/origens passaram no filtro X-Ray)
    edges.forEach(e => {
      const s = e.source.id || e.source
      const t = e.target.id || e.target
      
      if (nodesMap.has(s) || nodesMap.has(t)) {
        if (!nodesMap.has(s)) nodesMap.set(s, { id: s, name: s, virtual: true })
        if (!nodesMap.has(t)) nodesMap.set(t, { id: t, name: t, virtual: true })
      }
    })

    const links = (store.skeletalMode ? edges.filter(e => e.label === 'mst' || e.is_mst) : edges).filter(e => {
      const s = e.source.id || e.source
      const t = e.target.id || e.target
      return nodesMap.has(s) && nodesMap.has(t)
    }).map(e => ({
      source: e.source.id || e.source,
      target: e.target.id || e.target,
      ...e
    }))

    const finalNodes = Array.from(nodesMap.values())

    // 3. Cálculo de Massa Gravitacional (Degree) — O(L) com Map
    // Antes: O(N×L) = ~1.5M comparações. Agora: O(L) = ~2K iterações.
    const degreeMap = new Map()
    links.forEach(l => {
      degreeMap.set(l.source, (degreeMap.get(l.source) || 0) + 1)
      degreeMap.set(l.target, (degreeMap.get(l.target) || 0) + 1)
    })
    finalNodes.forEach(node => {
      node.degree = degreeMap.get(node.id) || 0
    })

    return { 
      nodes: finalNodes, 
      links 
    }
  }

  return { getGraphData }
}
