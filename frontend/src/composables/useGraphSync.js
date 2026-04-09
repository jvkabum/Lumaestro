import { ScanVault } from '../../wailsjs/go/core/App'
import { useGraphStore } from '../stores/graph'

/**
 * 🔄 useGraphSync — Sincronização e Varredura do Vault
 * 
 * Responsável por:
 * - Sincronização rápida (incremental) via ScanVault
 * - Sincronização total (FullSync) com reindexação forçada
 * - Modal de confirmação dinâmico
 * - SyncAllNodes na inicialização
 */
export function useGraphSync() {
  const store = useGraphStore()

  const handleFullSync = () => {
    store.modalMode = 'full'
    store.showConfirmModal = true
  }

  const handleFastSync = () => {
    store.modalMode = 'fast'
    store.showConfirmModal = true
  }

  const confirmSync = () => {
    store.showConfirmModal = false
    if (store.modalMode === 'full') {
      executeFullSync()
    } else {
      triggerScan()
    }
  }

  const executeFullSync = async () => {
    store.showConfirmModal = false
    store.scanning = true
    try {
      await FullSync()
    } catch (e) {
      console.error("Erro na sincronização:", e)
    } finally {
      store.scanning = false
      if (store.graphInstance) store.graphInstance.zoomToFit(800)
    }
  }

  const triggerScan = async () => {
    if (store.scanning) return
    store.scanning = true
    try {
      await ScanVault()
    } catch (error) {
      console.error("Erro no Scan:", error)
    } finally {
      store.scanning = false
    }
  }

  /**
   * Sincroniza todos os nós conhecidos do banco na partida
   */
  const syncAllOnStartup = () => {
    SyncAllNodes()
  }

  return {
    handleFullSync,
    handleFastSync,
    confirmSync,
    executeFullSync,
    triggerScan,
    syncAllOnStartup
  }
}
