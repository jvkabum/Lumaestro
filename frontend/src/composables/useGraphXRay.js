import { useGraphStore } from '../stores/graph'

/**
 * 🩻 useGraphXRay — Modo X-Ray, Recon Scan e Poda Neural
 * 
 * Responsável por:
 * - Recon Scan (varredura proativa)
 * - Poda Neural (remoção de nós com baixo PageRank)
 * - Controle do threshold X-Ray (filtragem reativa)
 */
export function useGraphXRay() {
  const store = useGraphStore()

  /**
   * Executa o Recon Scan proativo
   */
  const runReconScan = async () => {
    if (store.scanLoading) return
    store.scanLoading = true
    try {
      const result = await RunReconScan()
      console.log("[RECON] Scan concluído:", result)
    } catch (e) {
      console.error("Erro no Recon Scan:", e)
    } finally {
      store.scanLoading = false
    }
  }

  /**
   * Poda nós com PageRank abaixo do threshold (com confirmação)
   */
  const pruneNodes = async () => {
    if (confirm(`Deseja remover permanentemente nós com PageRank abaixo de ${store.xRayThreshold}? (Notas de origem são protegidas)`)) {
      store.pruneLoading = true
      try {
        const result = await PruneGraph(store.xRayThreshold)
        console.log("[PODA] Resultado:", result)
      } catch (e) {
        console.error("Erro na poda:", e)
      } finally {
        store.pruneLoading = false
      }
    }
  }

  return { runReconScan, pruneNodes }
}
