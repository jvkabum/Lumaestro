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
      alert("Preencha todos os campos obrigatórios do Projeto Satélite.")
      return
    }
    store.repoStatusMsg = "Aguarde... Engajando Crawlers no Repositório (Isso pode demorar dependendo da codebase)..."
    try {
      const res = await safeAddExternalProject(store.repoPathInput, store.coreNodeInput, store.includeCodeToggle)
      if (res.success) {
        alert("🪐 " + res.message)
        const cfg = await safeGetConfig()
        if (cfg) store.config = Object.assign({}, store.config, cfg)
        store.repoPathInput = ''
        store.coreNodeInput = ''
        store.includeCodeToggle = false
      } else {
        alert("Erro: " + res.error)
      }
    } catch (e) {
      const bridge = resolveBridge()
      const hint = bridge ? 'bridge ok' : 'bridge indisponível (window.go não inicializado)'
      alert("Falha Crítica ao vincular repositório: " + e + " | " + hint)
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
