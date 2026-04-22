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
    gemini_model: 'auto-gemini-2.5',
    identities: [],
    claude_api_key: '',
    use_claude_api_key: false,
    groq_api_key: '',
    groq_model: 'llama-3.3-70b-versatile',
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
    },
    // LM Studio (Motor Local)
    lmstudio_url: 'http://localhost:1234',
    lmstudio_model: '',
    lmstudio_enabled: false,

    // Pool de motores ativos (blend entre provedores)
    blend_active_models: true,
    active_model_providers: ['gemini', 'claude', 'lmstudio', 'groq', 'native'],
    primary_provider: 'gemini',

    // Motor de Embeddings (vetores semânticos para Qdrant)
    embeddings_provider: 'gemini',      // 'gemini' ou 'lmstudio'
    embeddings_model: '',               // Ex: 'nomic-embed-text', 'text-embedding-nomic-embed-text-v1.5'
    embedding_dimension: 3072,         // 3072=Gemini, 768=nomic, 1536=text-embedding-ada-002

    // Motor de RAG/Ontologia (geração textual para triplas e chat semântico)
    rag_provider: 'gemini',            // 'gemini', 'lmstudio' ou 'claude'
    rag_model: '',                     // Ex: 'google/gemma-4-26b-a4b', 'claude-3-5-sonnet-latest'
    hybrid_failover_enabled: false,
    failover_priority: ['groq', 'gemini', 'native'],
    active_groq_models: [],            // 🚀 Frota de Resiliência Groq
    active_google_models: [],          // 🌟 Frota de Resiliência Google
    external_projects: [],             // 🪐 Projetos Satélite
    active_workspace: ''               // 📂 Workspace ativo (diretório do projeto alvo)
  })

  // ── Status de Ferramentas ──
  const status = ref({
    qdrant: false,
    tools: {
      gemini: false,
      claude: false,
      obsidian: false,
      claude_auth: false,
      gemini_auth: false,
      groq: false
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

  // ── LM Studio ──
  const lmModels = ref([])
  const lmTesting = ref(false)
  const lmTestResult = ref(null)
  const lmLoadingModels = ref(false)

  // ── Computed ──
  const geminiKeyCount = computed(() => {
    const raw = (config.value.gemini_api_key || '').trim()
    if (!raw) return 0
    return raw.split(',').filter(k => k.trim() !== '').length
  })

  const groqKeyCount = computed(() => {
    const raw = (config.value.groq_api_key || '').trim()
    if (!raw) return 0
    return raw.split(',').filter(k => k.trim() !== '').length
  })

  // ── Notificações (Toast) ──
  const toast = ref({ message: '', show: false, type: 'info' })
  const notify = (message, type = 'info') => {
    toast.value = { message, type, show: true }
    setTimeout(() => {
      toast.value.show = false
    }, 4000)
  }

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
    lmModels, lmTesting, lmTestResult, lmLoadingModels,
    geminiKeyCount, groqKeyCount,
    toast, notify
  }
})
