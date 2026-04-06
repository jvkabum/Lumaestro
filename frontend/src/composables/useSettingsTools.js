import { InstallTool, SetupTool } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'
import { useSettingsConfig } from './useSettingsConfig'

/**
 * 🔩 useSettingsTools — Instalação e Setup de Ferramentas (Gemini, Claude)
 */
export function useSettingsTools() {
  const store = useSettingsStore()
  const { scrollToConsole, refreshStatus } = useSettingsConfig()

  const install = async (name) => {
    try {
      store.installLogs = []
      store.installStatus = `Iniciando operação para ${name}...`
      scrollToConsole()
      const res = await InstallTool(name)
      store.installStatus = res ? res : "Operação finalizada."
    } catch (err) {
      store.installStatus = `ERRO Crítico: ${err}`
    }
    refreshStatus()
  }

  const setup = async (name) => {
    store.installStatus = `Abrindo terminal de configuração para ${name}...`
    scrollToConsole()
    const res = await SetupTool(name)
    store.installStatus = res
  }

  return { install, setup }
}
