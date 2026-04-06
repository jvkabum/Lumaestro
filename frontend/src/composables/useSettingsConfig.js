import { nextTick } from 'vue'
import { GetConfig, SaveConfig, GetToolsStatus, ResetQdrantDB } from '../../wailsjs/go/core/App'
import { EventsOn } from '../../wailsjs/runtime'
import { useSettingsStore } from '../stores/settings'

/**
 * 🔧 useSettingsConfig — Carregar, Salvar e Gerenciar Configurações
 */
export function useSettingsConfig() {
  const store = useSettingsStore()

  const scrollToConsole = async () => {
    await nextTick()
    setTimeout(() => {
      const view = document.querySelector('.settings-view')
      if (view) {
        view.scrollTo({ top: view.scrollHeight, behavior: 'smooth' })
      }
    }, 100)
  }

  const loadConfig = async () => {
    try {
      const savedConfig = await GetConfig()
      if (savedConfig && Object.keys(savedConfig).length > 0) {
        store.config = Object.assign({}, store.config, savedConfig)
      } else {
        console.warn("Nenhuma config carregada do backend. Usando defaults.")
      }
    } catch (e) {
      alert("ERRO RARO DE COMUNICAÇÃO: " + e)
    }

    // Inicializa o estado do modo de exploração
    store.isExplorationMode = await window.go.main.App.IsExplorationMode()
  }

  const refreshStatus = async () => {
    store.status.tools = await GetToolsStatus()
  }

  const save = async () => {
    try {
      const res = await SaveConfig(store.config)
      alert(res)
      refreshStatus()
    } catch (err) {
      alert("Erro na comunicação Wails ao salvar: " + err)
    }
  }

  const initInstallerLogs = () => {
    EventsOn('installer:log', (log) => {
      store.installLogs.push(log)
      if (store.logContainer) {
        setTimeout(() => {
          store.logContainer.scrollTop = store.logContainer.scrollHeight
        }, 10)
      }
    })
  }

  // Helpers para auto-start toggles
  const isAutoStart = (agent) => {
    return (store.config.auto_start_agents || []).includes(agent)
  }

  const toggleAutoStart = async (agent) => {
    if (!store.config.auto_start_agents) {
      store.config.auto_start_agents = []
    }
    const idx = store.config.auto_start_agents.indexOf(agent)
    if (idx >= 0) {
      store.config.auto_start_agents.splice(idx, 1)
    } else {
      store.config.auto_start_agents.push(agent)
    }
    await SaveConfig(store.config)
  }

  const toggleExplorationMode = async () => {
    const res = await window.go.main.App.SetExplorationMode(store.isExplorationMode)
    console.log(res)
  }

  const handleResetDB = async () => {
    store.isResetting = true
    try {
      const res = await ResetQdrantDB()
      alert(res)
      store.showResetModal = false
      refreshStatus()
    } catch (e) {
      alert("Erro ao resetar banco: " + e)
    } finally {
      store.isResetting = false
    }
  }

  const runDiagnostic = async () => {
    store.isDiagnosing = true
    store.diagnosticResult = null
    try {
      const res = await window.go.main.App.RunVectorDiagnostic()
      store.diagnosticResult = res
    } catch (e) {
      store.diagnosticResult = { success: false, error: String(e) }
    } finally {
      store.isDiagnosing = false
    }
  }

  const getAuthLabel = (agent) => {
    if (store.config[`use_${agent}_api_key`]) {
      return 'CHAVE API ⚡'
    }
    return agent === 'claude' ? 'FAZER LOGIN (OAUTH)' : 'CONFIGURAR LOGIN'
  }

  const getAuthStyle = (agent) => {
    if (store.config[`use_${agent}_api_key`]) {
      return 'border-color: rgba(245, 158, 11, 0.4); color: #fde68a; background: rgba(245, 158, 11, 0.08);'
    }
    return 'border-color: #3b82f6;'
  }

  return {
    scrollToConsole,
    loadConfig, refreshStatus, save,
    initInstallerLogs,
    isAutoStart, toggleAutoStart,
    toggleExplorationMode,
    handleResetDB, runDiagnostic,
    getAuthLabel, getAuthStyle
  }
}
