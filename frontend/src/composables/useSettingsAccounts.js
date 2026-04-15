import { GetConfig, AddIdentity, SwitchIdentity, LoginIdentity, RemoveIdentity } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'

/**
 * 👥 useSettingsAccounts — Gestão Universal de Identidades (Multi-Provedor)
 */
export function useSettingsAccounts() {
  const store = useSettingsStore()

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
    if (!confirm(`Deseja realmente remover a identidade "${name}"?`)) return
    await RemoveIdentity(provider, name)
    const cfg = await GetConfig()
    if (cfg) store.config = cfg
  }

  return { handleAddAccount, handleLoginAccount, handleSwitchAccount, handleRemoveAccount }
}
