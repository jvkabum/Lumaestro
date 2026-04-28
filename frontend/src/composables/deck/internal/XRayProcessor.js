import { useGraphStore } from '../../../stores/graph'
import { useOrchestratorStore } from '../../../stores/orchestrator'

/**
 * ⚡ XRayProcessor — O Analista de Infraestrutura
 * 
 * Responsável por operações de escaneamento profundo e 
 * manutenção de integridade do multiverso neural (Poda e Recon).
 */
export function useXRayProcessor() {
  const store = useGraphStore()
  const orchestrator = useOrchestratorStore()

  /**
   * Executa um escaneamento de reconhecimento no backend
   */
  const runReconScan = async () => {
    if (store.scanLoading) return
    store.scanLoading = true
    try {
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      
      const result = await bridge.RunReconScan()
      console.log("[X-Ray] Recon Scan concluído:", result)
    } catch (e) {
      console.error("[X-Ray] Erro no Recon Scan:", e)
    } finally {
      store.scanLoading = false
    }
  }

  /**
   * Poda nós com PageRank abaixo do threshold
   */
  const pruneNodes = async () => {
    const confirmed = await orchestrator.confirm({
      title: 'PODA NEURAL (X-RAY)',
      message: `Deseja remover permanentemente nós com PageRank abaixo de ${store.xRayThreshold}?\n\nIsso limpará a visualização de nós com baixa relevância sistêmica. Notas de origem (Obsidian) são protegidas e não serão deletadas.`,
      type: 'warning',
      confirmText: 'EXECUTAR PODA',
      cancelText: 'ABORTAR'
    })

    if (confirmed) {
      store.pruneLoading = true
      try {
        const bridge = (window.go && window.go.core && window.go.core.App) || 
                       (window.go && window.go.main && window.go.main.App);
        
        const result = await bridge.PruneGraph(store.xRayThreshold)
        console.log("[X-Ray] Poda concluída:", result)
      } catch (e) {
        console.error("[X-Ray] Erro na poda:", e)
      } finally {
        store.pruneLoading = false
      }
    }
  }

  return { runReconScan, pruneNodes }
}
