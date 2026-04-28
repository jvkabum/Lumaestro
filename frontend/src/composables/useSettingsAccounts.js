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
    if (cfg) store.config = cfg
  }

  const handleLoginAccount = async (provider, name) => {
    await LoginIdentity(provider, name)
  }

  const handleSwitchAccount = async (provider, name) => {
    await SwitchIdentity(provider, name)
    const cfg = await GetConfig()
    if (cfg) store.config = cfg
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
    if (cfg) store.config = cfg
  }

  return { handleAddAccount, handleLoginAccount, handleSwitchAccount, handleRemoveAccount }
}
