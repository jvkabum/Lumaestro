import { GetConfig } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'

/**
 * 🪐 useSettingsProjects — Configuração de Repositórios Satélites (RAG Radial)
 */
export function useSettingsProjects() {
  const store = useSettingsStore()

  const handleAddProject = async () => {
    if (!store.repoPathInput || !store.coreNodeInput) {
      alert("Preencha todos os campos obrigatórios do Projeto Satélite.")
      return
    }
    store.repoStatusMsg = "Aguarde... Engajando Crawlers no Repositório (Isso pode demorar dependendo da codebase)..."
    try {
      const res = await window.go.main.App.AddExternalProject(store.repoPathInput, store.coreNodeInput, store.includeCodeToggle)
      if (res.success) {
        alert("🪐 " + res.message)
        const cfg = await GetConfig()
        if (cfg) store.config = Object.assign({}, store.config, cfg)
        store.repoPathInput = ''
        store.coreNodeInput = ''
        store.includeCodeToggle = false
      } else {
        alert("Erro: " + res.error)
      }
    } catch (e) {
      alert("Falha Crítica ao vincular repositório: " + e)
    }
    store.repoStatusMsg = ''
  }

  const handleSelectDirectory = async () => {
    try {
      const dir = await window.go.main.App.SelectDirectory()
      if (dir && dir.trim() !== '') {
        store.repoPathInput = dir
      }
    } catch (e) {
      console.warn("Navegador de pastas cancelado ou erro:", e)
    }
  }

  return { handleAddProject, handleSelectDirectory }
}
