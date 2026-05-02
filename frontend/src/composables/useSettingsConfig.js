import { nextTick } from 'vue'
import { GetConfig, SaveConfig, GetToolsStatus, ResetQdrantDB, IsExplorationMode, SetExplorationMode, RunVectorDiagnostic } from '../../wailsjs/go/core/App'
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
      store.notify("ERRO DE COMUNICAÇÃO: " + e, 'error')
    }

    // Inicializa o estado do modo de exploração
    store.isExplorationMode = await IsExplorationMode()
  }

  const refreshStatus = async () => {
    store.status.tools = await GetToolsStatus()
  }

  const save = async () => {
    try {
      const res = await SaveConfig(store.config)
      store.notify(res, 'success')
      refreshStatus()
    } catch (err) {
      store.notify("Erro ao salvar: " + err, 'error')
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
    let enabled = false
    if (idx >= 0) {
      store.config.auto_start_agents.splice(idx, 1)
      enabled = false
    } else {
      store.config.auto_start_agents.push(agent)
      enabled = true
    }

    // LM Studio usa o mesmo toggle para definir ativação do motor.
    if (agent === 'lmstudio') {
      store.config.lmstudio_enabled = enabled
    }

    await SaveConfig(store.config)
  }

  const toggleExplorationMode = async () => {
    const res = await SetExplorationMode(store.isExplorationMode)
    console.log(res)
  }

  const handleResetDB = async () => {
    store.isResetting = true
    try {
      const res = await ResetQdrantDB()
      store.notify(res, 'success')
      store.showResetModal = false
      refreshStatus()
    } catch (e) {
      store.notify("Erro ao resetar banco: " + e, 'error')
    } finally {
      store.isResetting = false
    }
  }

  const runDiagnostic = async () => {
    store.isDiagnosing = true
    store.diagnosticResult = null
    try {
      const res = await RunVectorDiagnostic()
      store.diagnosticResult = res
    } catch (e) {
      store.diagnosticResult = { success: false, error: String(e) }
    } finally {
      store.isDiagnosing = false
    }
  }

  const getAuthLabel = (tool) => {
    const isAuth = store.status.tools[tool + '_auth'] || store.config[`use_${tool}_api_key`]
    if (tool === 'gemini') return isAuth ? 'LOGIN ATIVADO ✓' : 'CONFIGURAR LOGIN'
    if (tool === 'claude') return isAuth ? 'CONECTADO ✓' : 'FAZER LOGIN (OAUTH)'
    return 'CONFIGURAR LOGIN'
  }

  const getAuthStyle = (tool) => {
    const isAuth = store.status.tools[tool + '_auth'] || store.config[`use_${tool}_api_key`]
    if (tool === 'gemini') {
      return isAuth 
        ? 'background: rgba(16, 185, 129, 0.1); border-color: rgba(16, 185, 129, 0.3); color: #10b981;' 
        : 'background: rgba(59, 130, 246, 0.05); border-color: rgba(59, 130, 246, 0.3); color: #60a5fa;'
    }
    if (tool === 'claude') {
      return isAuth 
        ? 'background: rgba(16, 185, 129, 0.1); border-color: rgba(16, 185, 129, 0.3); color: #10b981;' 
        : 'background: rgba(249, 115, 22, 0.05); border-color: rgba(249, 115, 22, 0.3); color: #fdba74;'
    }
    return ''
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
