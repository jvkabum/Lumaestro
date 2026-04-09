import { InstallTool, SetupTool, SaveConfig } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'
import { useSettingsConfig } from './useSettingsConfig'

/**
 * 🔩 useSettingsTools — Instalação e Setup de Ferramentas (Gemini, Claude, LM Studio)
 */
export function useSettingsTools() {
  const store = useSettingsStore()
  const { scrollToConsole, refreshStatus } = useSettingsConfig()

  const install = async (name) => {
    // LM Studio não precisa ser "instalado", só configurado
    if (name === 'lmstudio') {
      try {
        if (store.config.lmstudio_url && String(store.config.lmstudio_url).trim() !== '') {
          store.config.lmstudio_enabled = true
        }
        store.installStatus = 'Salvando configuração do LM Studio...'
        scrollToConsole()
        const res = await SaveConfig(store.config)
        store.installStatus = res || 'Configuração do LM Studio salva com sucesso!'
      } catch (err) {
        store.installStatus = `ERRO ao salvar: ${err}`
      }
      refreshStatus()
      return
    }

    // Para Gemini e Claude, instala normalmente
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
