import { useGraphStore } from '../../../stores/graph'

/**
 * 🔄 SyncManager — O Guardião da Persistência
 * 
 * Responsável por garantir que o estado do grafo no frontend 
 * esteja em sincronia com o banco de dados local do backend.
 */
export function useSyncManager() {
  const store = useGraphStore()

  /**
   * Sincroniza configurações globais e saúde do grafo na partida
   */
  const syncAllOnStartup = async () => {
    try {
      // Bridge Wails
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      
      // 🩺 Saúde do Grafo (nome correto: AnalyzeGraphHealth)
      if (bridge && bridge.AnalyzeGraphHealth) {
        try {
          const stats = await bridge.AnalyzeGraphHealth()
          if (stats) store.graphHealth = stats
        } catch (e) {
          console.warn("[Sync] Saúde do grafo indisponível:", e)
        }
      }

      // 🧠 [ESSENCIAL] Solicita ao backend o envio em lote de todos os nós persistidos
      // Usa LoadFastGraph por padrão, pois não depende do Qdrant estar 100% online
      if (bridge && bridge.LoadFastGraph) {
        await bridge.LoadFastGraph()
        console.log("[Sync] Gatilho LoadFastGraph disparado.")
      }

      // 🔄 [FALLBACK VITAL] Se LoadFastGraph não trouxe dados (cache vazio),
      // força SyncAllNodes que busca direto do Qdrant
      if (bridge && bridge.SyncAllNodes) {
        // Pequeno delay para dar tempo ao LoadFastGraph emitir
        await new Promise(r => setTimeout(r, 2000))
        if (store.nodes.length === 0) {
          console.log("[Sync] ⚠️ Cache vazio. Acionando SyncAllNodes como fallback...")
          await bridge.SyncAllNodes()
          console.log("[Sync] Gatilho SyncAllNodes (fallback) disparado.")
        }
      }
    } catch (err) {
      console.error("[Sync] Falha na sincronização inicial:", err)
    }
  }

  /**
   * Executa uma sincronização total com reindexação forçada
   */
  const executeFullSync = async () => {
    store.scanning = true
    try {
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      await bridge.FullSync()
      if (bridge && bridge.SyncAllNodes) {
        await bridge.SyncAllNodes()
      }
    } catch (e) {
      console.error("[Sync] Erro na sincronização total:", e)
    } finally {
      store.scanning = false
    }
  }

  /**
   * Dispara uma varredura incremental rápida
   */
  const triggerScan = async () => {
    if (store.scanning) return
    store.scanning = true
    try {
      const bridge = (window.go && window.go.core && window.go.core.App) || 
                     (window.go && window.go.main && window.go.main.App);
      await bridge.ScanVault()
    } catch (error) {
      console.error("[Sync] Erro no ScanVault:", error)
    } finally {
      store.scanning = false
    }
  }

  return { syncAllOnStartup, executeFullSync, triggerScan }
}
