import { GetConfig, AddExternalProject, SelectDirectory } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'

/**
 * 🪐 useSettingsProjects — Configuração de Repositórios Satélites (RAG Radial)
 */
export function useSettingsProjects() {
  const store = useSettingsStore()

  const resolveBridge = () => {
    if (typeof window === 'undefined') return null
    const coreBridge = window?.go?.core?.App
    const mainBridge = window?.go?.main?.App
    return coreBridge || mainBridge || null
  }

  const safeGetConfig = async () => {
    try {
      return await GetConfig()
    } catch (_) {
      const bridge = resolveBridge()
      if (bridge && typeof bridge.GetConfig === 'function') {
        return await bridge.GetConfig()
      }
      return null
    }
  }

  const safeAddExternalProject = async (repoPath, coreNode, includeCode) => {
    try {
      return await AddExternalProject(repoPath, coreNode, includeCode)
    } catch (err) {
      const bridge = resolveBridge()
      if (bridge && typeof bridge.AddExternalProject === 'function') {
        return await bridge.AddExternalProject(repoPath, coreNode, includeCode)
      }
      throw err
    }
  }

  const safeSelectDirectory = async () => {
    try {
      return await SelectDirectory()
    } catch (err) {
      const bridge = resolveBridge()
      if (bridge && typeof bridge.SelectDirectory === 'function') {
        return await bridge.SelectDirectory()
      }
      throw err
    }
  }

  const handleAddProject = async () => {
    if (!store.repoPathInput || !store.coreNodeInput) {
      store.notify("Campos obrigatórios ausentes. Verifique o caminho e o núcleo radial do projeto.", "error")
      return
    }
    store.repoStatusMsg = "Aguarde... Engajando Crawlers no Repositório (isso pode demorar dependendo do tamanho da codebase)..."
    try {
      console.log("[Projects] Tentando vincular:", store.repoPathInput)
      const res = await safeAddExternalProject(store.repoPathInput, store.coreNodeInput, store.includeCodeToggle)
      
      if (res.success) {
        console.log("[Projects] Sucesso:", res.message)
        store.notify("🪐 Sinfonia sincronizada: Projeto Satélite integrado ao enxame.", "success")
        
        // Limpa os inputs imediatamente
        store.repoPathInput = ''
        store.coreNodeInput = ''
        store.includeCodeToggle = false

        // Recarrega a configuração global para atualizar a lista na UI
        const cfg = await safeGetConfig()
        if (cfg) {
          console.log("[Projects] Nova configuração carregada. Projetos:", cfg.external_projects?.length)
          store.config = cfg
        }
      } else {
        console.error("[Projects] Falha na Órbita:", res.error)
        store.notify("Falha na Órbita: " + res.error, "error")
      }
    } catch (e) {
      console.error("[Projects] Erro Crítico:", e)
      store.notify("Erro Crítico ao vincular repositório: " + e, "error")
    }
    store.repoStatusMsg = ''
  }

  const handleSelectDirectory = async () => {
    try {
      const dir = await safeSelectDirectory()
      if (dir && dir.trim() !== '') {
        store.repoPathInput = dir
      }
    } catch (e) {
      console.warn("Navegador de pastas cancelado ou erro:", e)
    }
  }

  return { handleAddProject, handleSelectDirectory }
}
