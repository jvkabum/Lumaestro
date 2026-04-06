import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

/**
 * ⚙️ SETTINGS STORE — Estado Centralizado das Configurações do Lumaestro
 */
export const useSettingsStore = defineStore('settings', () => {
  // ── Configuração Principal ──
  const config = ref({
    obsidian_vault_path: '',
    qdrant_url: '',
    qdrant_api_key: '',
    gemini_api_key: '',
    use_gemini_api_key: false,
    gemini_accounts: [],
    claude_api_key: '',
    use_claude_api_key: false,
    active_agent: 'gemini',
    auto_start_agents: [],
    agent_language: 'Português do Brasil',
    graph_depth: 1,
    graph_neighbor_limit: 5,
    graph_context_limit: 4000,
    security: {
      allow_read: false,
      allow_write: false,
      allow_create: false,
      allow_delete: false,
      allow_move: false,
      allow_run_commands: false,
      full_machine_access: false
    }
  })

  // ── Status de Ferramentas ──
  const status = ref({
    qdrant: false,
    tools: {
      gemini: false,
      claude: false,
      obsidian: false,
      claude_auth: false,
      gemini_auth: false
    }
  })

  // ── Terminal / Logs de Instalação ──
  const installLogs = ref([])
  const installStatus = ref('')
  const logContainer = ref(null)

  // ── Aba Ativa ──
  const activeTab = ref('geral')

  // ── Diagnóstico Vetorial ──
  const isDiagnosing = ref(false)
  const diagnosticResult = ref(null)

  // ── Modo Exploração Neural ──
  const isExplorationMode = ref(false)

  // ── Reset DB ──
  const showResetModal = ref(false)
  const isResetting = ref(false)

  // ── MCP ──
  const mcpName = ref('')
  const mcpCommand = ref('')
  const mcpServers = ref('')
  const showMcpList = ref(false)

  // ── Contas Gemini ──
  const newAccName = ref('')

  // ── Repositórios Satélite ──
  const repoPathInput = ref('')
  const coreNodeInput = ref('')
  const includeCodeToggle = ref(false)
  const repoStatusMsg = ref('')

  // ── Computed ──
  const geminiKeyCount = computed(() => {
    const raw = (config.value.gemini_api_key || '').trim()
    if (!raw) return 0
    return raw.split(',').filter(k => k.trim() !== '').length
  })

  return {
    config, status,
    installLogs, installStatus, logContainer,
    activeTab,
    isDiagnosing, diagnosticResult,
    isExplorationMode,
    showResetModal, isResetting,
    mcpName, mcpCommand, mcpServers, showMcpList,
    newAccName,
    repoPathInput, coreNodeInput, includeCodeToggle, repoStatusMsg,
    geminiKeyCount
  }
})
