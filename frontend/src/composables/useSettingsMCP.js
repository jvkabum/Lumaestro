import { AddMCPServer, ListMCPServers } from '../../wailsjs/go/core/App'
import { useSettingsStore } from '../stores/settings'
import { useSettingsConfig } from './useSettingsConfig'

/**
 * 🔌 useSettingsMCP — Configuração de Model Context Protocol
 */
export function useSettingsMCP() {
  const store = useSettingsStore()
  const { scrollToConsole } = useSettingsConfig()

  const addMCPServer = async () => {
    if (!store.mcpName || !store.mcpCommand) {
      store.notify("Preencha o Nome e o Comando para configurar o servidor MCP.", "error")
      return
    }
    store.installLogs = []
    store.installStatus = `Instalando servidor MCP: ${store.mcpName}...`
    scrollToConsole()
    const res = await AddMCPServer(store.mcpName, store.mcpCommand)
    store.installStatus = "Instalação do MCP Finalizada."
    store.mcpName = ''
    store.mcpCommand = ''
    store.notify("Configuração MCP: " + res, "success")
  }

  const listMCPServers = async () => {
    const res = await ListMCPServers()
    store.mcpServers = res
    store.showMcpList = true
  }

  return { addMCPServer, listMCPServers }
}
