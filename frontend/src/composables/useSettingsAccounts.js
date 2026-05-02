import { GetConfig, AddIdentity, SwitchIdentity, LoginIdentity, RemoveIdentity } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'

import { useOrchestratorStore } from '../stores/orchestrator'

/**
 * 👥 useSettingsAccounts — Gestão Universal de Identidades (Multi-Provedor)
 */
export function useSettingsAccounts() {
  const store = useSettingsStore()
  const orchestrator = useOrchestratorStore()

  const handleAddAccount = async (provider) => {
    if (!store.newAccName) return
    await AddIdentity(provider, store.newAccName)
    store.newAccName = ''
    const cfg = await GetConfig()
    if (cfg) store.config = Object.assign({}, store.config, cfg)
  }

  const handleLoginAccount = async (provider, name) => {
    await LoginIdentity(provider, name)
  }

  const handleSwitchAccount = async (provider, name) => {
    // ⚡ Atualização Otimista (Optimistic UI) para reação instantânea na interface
    if (store.config && store.config.identities) {
      store.config.identities = store.config.identities.map(id => {
        if (id.provider === provider) {
          return { ...id, active: id.name === name }
        }
        return id
      })
    }

    await SwitchIdentity(provider, name)
    const cfg = await GetConfig()
    if (cfg) store.$patch({ config: cfg })

    // ⚡ Se trocou conta do Google, reinicia o ACP para aplicar as novas credenciais do Hangar
    if (provider === 'google') {
      try {
        const { StopAgentSession, StartAgentSession, GetToolsStatus } = await import('../../wailsjs/go/core/App')
        await StopAgentSession('gemini')
        await StartAgentSession('gemini')
        
        // Atualiza a verificação de autenticação na UI para a nova conta
        store.status.tools = await GetToolsStatus()
      } catch (e) {
        console.error("Falha ao reiniciar ACP da conta Google", e)
      }
    }
  }

  const handleRemoveAccount = async (provider, name) => {
    const confirmed = await orchestrator.confirm({
      title: 'DISSOLVER IDENTIDADE',
      message: `Deseja realmente remover a identidade "${name}" do Nexus? Todas as conexões neurais locais serão mantidas, mas a autenticação será revogada.`,
      type: 'warning',
      confirmText: 'DISSOLVER',
      cancelText: 'MANTER'
    })

    if (!confirmed) return
    await RemoveIdentity(provider, name)
    const cfg = await GetConfig()
    if (cfg) store.config = Object.assign({}, store.config, cfg)
  }

  return { handleAddAccount, handleLoginAccount, handleSwitchAccount, handleRemoveAccount }
}
