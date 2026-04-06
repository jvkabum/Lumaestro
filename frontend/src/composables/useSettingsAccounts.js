import { GetConfig, AddGeminiAccount, SwitchGeminiAccount, LoginGeminiAccount } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'

/**
 * 👥 useSettingsAccounts — Multi-Conta Gemini (OAuth)
 */
export function useSettingsAccounts() {
  const store = useSettingsStore()

  const handleAddAccount = async () => {
    if (!store.newAccName) return
    await AddGeminiAccount(store.newAccName)
    store.newAccName = ''
    const cfg = await GetConfig()
    if (cfg) store.config = cfg
  }

  const handleLoginAccount = async (name) => {
    await LoginGeminiAccount(name)
  }

  const handleSwitchAccount = async (name) => {
    await SwitchGeminiAccount(name)
    const cfg = await GetConfig()
    if (cfg) store.config = cfg
  }

  return { handleAddAccount, handleLoginAccount, handleSwitchAccount }
}
