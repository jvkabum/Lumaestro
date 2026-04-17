import { useGraphStore } from '../../stores/graph'
import { useSyncManager } from './internal/SyncManager'
import { useXRayProcessor } from './internal/XRayProcessor'
import { useBridgeDriver } from './engine/BridgeDriver'

/**
 * 🕹️ useGraphActions — O Painel de Controle Remoto
 * 
 * Composable sem estado (stateless) para execução de comandos de usuário.
 * Pode ser importado em qualquer lugar da UI sem disparar ciclos de vida do motor.
 */
export function useGraphActions() {
  const store = useGraphStore()
  const { executeFullSync, triggerScan } = useSyncManager()
  const { runReconScan, pruneNodes } = useXRayProcessor()
  const { resolveConflict } = useBridgeDriver()

  // ── Handlers de Sincronização ──
  const handleFullSync = () => { 
    store.modalMode = 'full'; 
    store.showConfirmModal = true; 
  }
  
  const handleFastSync = () => { 
    store.modalMode = 'fast'; 
    store.showConfirmModal = true; 
  }
  
  const confirmSync = () => {
    store.showConfirmModal = false;
    if (store.modalMode === 'full') executeFullSync()
    else triggerScan()
  }

  return {
    handleFullSync,
    handleFastSync,
    confirmSync,
    runReconScan,
    pruneNodes,
    resolveConflict
  }
}
