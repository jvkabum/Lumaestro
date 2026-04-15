<script setup>
import { onMounted, ref, watch, computed } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useSettingsConfig } from '../composables/useSettingsConfig'
import { useSettingsTools } from '../composables/useSettingsTools'
import { useSettingsMCP } from '../composables/useSettingsMCP'
import { useSettingsAccounts } from '../composables/useSettingsAccounts'

// ── Store Pinia ──
const store = useSettingsStore()

// ── Composables ──
const { 
  loadConfig, refreshStatus, save, initInstallerLogs, 
  isAutoStart, toggleAutoStart, toggleExplorationMode,
  handleResetDB, runDiagnostic, getAuthLabel, getAuthStyle
} = useSettingsConfig()

const { install, setup } = useSettingsTools()
const { addMCPServer, listMCPServers } = useSettingsMCP()
const { handleAddAccount, handleLoginAccount, handleSwitchAccount, handleRemoveAccount } = useSettingsAccounts()

// ── IDENTIDADE MULTI-PROVEDOR ──
const selectedAccountProvider = ref('google')
const accountProviders = [
  { id: 'google', label: 'Google (Gemini)', icon: '💎', color: '#3b82f6' },
  { id: 'claude', label: 'Claude', icon: '🟠', color: '#f97316' },
  { id: 'openai', label: 'OpenAI (GPT)', icon: '🟢', color: '#10b981' },
  { id: 'qwen', label: 'Qwen', icon: '🟣', color: '#8b5cf6' }
]

const filteredIdentities = computed(() => {
  if (!store.config || !store.config.identities) return []
  return store.config.identities.filter(id => id.provider === selectedAccountProvider.value)
})

// ── LM Studio ──
const pickDefaultEmbeddingModel = (models) => {
  if (!Array.isArray(models) || models.length === 0) return ''
  const preferred = models.find((m) => /(embed|embedding|nomic|bge|e5|gte)/i.test(m))
  return preferred || ''
}

const detectEmbeddingDimension = async () => {
  if (store.config.embeddings_provider !== 'lmstudio') return
  const model = (store.config.embeddings_model || store.config.lmstudio_model || '').trim()
  if (!model) return

  try {
    const dim = await window.go.core.App.DetectLMStudioEmbeddingDimension(model)
    if (Number(dim) > 0) {
      store.config.embedding_dimension = Number(dim)
    } else {
      store.notify(`O modelo "${model}" não respondeu no endpoint de embeddings do LM Studio.`, 'error')
    }
  } catch (e) {
    store.notify('Falha ao detectar dimensão do embedding: ' + e, 'error')
  }
}

const loadLMModels = async () => {
  store.lmLoadingModels = true
  store.lmModels = []
  try {
    const models = await window.go.core.App.ListLMStudioModels()
    store.lmModels = models || []
    if (store.lmModels.length > 0 && !store.config.lmstudio_model) {
      store.config.lmstudio_model = store.lmModels[0]
    }

    if (store.config.embeddings_provider === 'lmstudio' && !store.config.embeddings_model) {
      const embModel = pickDefaultEmbeddingModel(store.lmModels)
      if (embModel) {
        store.config.embeddings_model = embModel
      }
    }

    if (store.config.rag_provider === 'lmstudio' && !store.config.rag_model) {
      store.config.rag_model = store.config.lmstudio_model || store.lmModels[0] || ''
    }

    await detectEmbeddingDimension()
  } catch (e) {
    store.notify('Erro ao conectar ao LM Studio: ' + e, 'error')
  } finally {
    store.lmLoadingModels = false
  }
}

const testLMStudio = async () => {
  store.lmTesting = true
  store.lmTestResult = null
  try {
    const url = store.config.lmstudio_url || 'http://localhost:1234'
    const model = store.config.lmstudio_model || ''
    const result = await window.go.core.App.TestLMStudioModel(url, model)
    store.lmTestResult = result
  } catch (e) {
    store.lmTestResult = { success: false, error: String(e) }
  } finally {
    store.lmTesting = false
  }
}

const providerLabel = (provider) => {
  if (provider === 'lmstudio') return 'LM Studio'
  if (provider === 'claude') return 'Claude'
  return 'Gemini'
}

const isProviderActive = (provider) => {
  return (store.config.active_model_providers || []).includes(provider)
}

// ── GROQ FLEET MANAGER ──
const availableGroqModels = [
  { id: 'llama-3.3-70b-versatile', label: 'Cérebro Superior', icon: '🧠', tier: 'Top Tier' },
  { id: 'openai/gpt-oss-120b', label: 'Gigante OSS (120B)', icon: '🐘', tier: 'Ultra Scale' },
  { id: 'qwen/qwen3-32b', label: 'Especialista JSON', icon: '💎', tier: 'Reasoning' },
  { id: 'moonshotai/kimi-k2-instruct', label: 'Raciocínio Longo', icon: '🧠', tier: 'Expert' },
  { id: 'moonshotai/kimi-k2-instruct-0905', label: 'Snapshot Kimi', icon: '📦', tier: 'Extra Quota' },
  { id: 'meta-llama/llama-4-scout-17b-16e-instruct', label: 'Cavalo de Batalha', icon: '🐎', tier: 'Volume' },
  { id: 'openai/gpt-oss-20b', label: 'Reserva de Elite', icon: '🛡️', tier: 'Safety' },
  { id: 'allam-2-7b', label: 'Volume Adicional (7B)', icon: '📦', tier: 'Utility' },
  { id: 'llama-3.1-8b-instant', label: 'Motor de Jato', icon: '⚡', tier: 'Instant' },
  { id: 'groq/compound', label: 'Experimental', icon: '🧪', tier: 'Research' },
  { id: 'groq/compound-mini', label: 'Experimental Mini', icon: '🧪', tier: 'Research' }
]

const isModelActive = (modelId) => {
  if (!store.config.active_groq_models) return true
  return store.config.active_groq_models.includes(modelId)
}

const toggleGroqModel = (modelId) => {
  if (!store.config.active_groq_models) {
    store.config.active_groq_models = availableGroqModels.map(m => m.id)
  }
  const idx = store.config.active_groq_models.indexOf(modelId)
  if (idx > -1) {
    if (store.config.active_groq_models.length > 1) {
      store.config.active_groq_models.splice(idx, 1)
    } else {
      store.notify("Pelo menos um modelo deve permanecer ativo para a resiliência.", 'info')
    }
  } else {
    store.config.active_groq_models.push(modelId)
  }
}

// ── GOOGLE FLEET MANAGER ──
const availableGoogleModels = [
  { id: 'gemini-3.1-flash-lite-preview', label: 'Flash 3.1 (Lite)', icon: '🚀', tier: 'Speed' },
  { id: 'gemini-2.5-flash', label: 'Capitão Flash 2.5', icon: '🏆', tier: 'Premium' },
  { id: 'gemini-3-flash-preview', label: 'Modern Flash 3', icon: '⚖️', tier: 'Preview' },
  { id: 'gemini-2.5-flash-lite', label: 'Escala de Volume', icon: '📦', tier: 'Quota' },
  { id: 'gemma-4-31b-it', label: 'O Tanque (31B)', icon: '🛡️', tier: 'Resilient' },
  { id: 'gemma-4-26b-a4b-it', label: 'Reserva Tática', icon: '🐘', tier: 'Gemma' }
]

const isGoogleModelActive = (modelId) => {
  if (!store.config.active_google_models) return true
  return store.config.active_google_models.includes(modelId)
}

const toggleGoogleModel = (modelId) => {
  if (!store.config.active_google_models) {
    store.config.active_google_models = availableGoogleModels.map(m => m.id)
  }
  const idx = store.config.active_google_models.indexOf(modelId)
  if (idx > -1) {
    if (store.config.active_google_models.length > 1) {
      store.config.active_google_models.splice(idx, 1)
    } else {
        store.notify("Pelo menos um modelo deve permanecer ativo para a resiliência.", 'info')
    }
  } else {
    store.config.active_google_models.push(modelId)
  }
}

const toggleProvider = (provider) => {
  if (!store.config.active_model_providers) {
    store.config.active_model_providers = []
  }

  const list = store.config.active_model_providers
  const idx = list.indexOf(provider)

  if (idx >= 0) {
    if (list.length === 1) return
    list.splice(idx, 1)
    if (!list.includes(store.config.primary_provider)) {
      store.config.primary_provider = list[0]
    }
    return
  }

  list.push(provider)
}

const setPrimaryProvider = (provider) => {
  store.config.primary_provider = provider
  if (!isProviderActive(provider)) {
    toggleProvider(provider)
  }
}

const pendingProvider = ref('')
const showProviderModal = ref(false)

const confirmProviderChange = (e) => {
  // Aceita tanto o evento do <select> quanto o valor direto do card
  const newProv = e?.target?.value || e
  if (newProv === store.config.embeddings_provider) return
  
  pendingProvider.value = newProv
  showProviderModal.value = true
}

const movePriority = (idx) => {
  const arr = [...store.config.failover_priority]
  const item = arr.splice(idx, 1)[0]
  arr.push(item)
  store.config.failover_priority = arr
  save()
}

const resetPriority = () => {
  store.config.failover_priority = ['groq', 'gemini', 'native']
  save()
}

const removePriority = (idx) => {
  if (store.config.failover_priority.length <= 1) return
  const arr = [...store.config.failover_priority]
  arr.splice(idx, 1)
  store.config.failover_priority = arr
  save()
}

const addPriority = (name) => {
  if (store.config.failover_priority.includes(name)) return
  const arr = [...store.config.failover_priority]
  arr.push(name)
  store.config.failover_priority = arr
  save()
}

const availableFailoverProviders = ref([])
watch(() => store.config.failover_priority, (newVal) => {
  const all = ['groq', 'gemini', 'native']
  availableFailoverProviders.value = all.filter(p => !newVal.includes(p))
}, { immediate: true })

const applyProviderChange = async () => {
  store.config.embeddings_provider = pendingProvider.value
  showProviderModal.value = false
  
  if (store.config.embeddings_provider === 'gemini') {
    store.config.embedding_dimension = 3072
    store.config.embeddings_model = ''
  } else if (store.config.embeddings_provider === 'lmstudio') {
    if (store.lmModels.length === 0) {
      loadLMModels()
    } else {
      if (!store.config.embeddings_model) {
        const embModel = pickDefaultEmbeddingModel(store.lmModels)
        if (embModel) store.config.embeddings_model = embModel
      }
      detectEmbeddingDimension()
    }
  } else if (store.config.embeddings_provider === 'native') {
    store.config.embedding_dimension = 1024
    store.config.embeddings_model = 'qwen3-0.6b-embedding.gguf'
  }
  
  // 🔥 SALVA e notifica o backend imediatamente
  await save()
  
  // Após aplicar a mudança, abre o modal de reset do Qdrant
  setTimeout(() => {
    store.showResetModal = true
  }, 300)
}

const cancelProviderChange = () => {
  showProviderModal.value = false
  pendingProvider.value = ''
  const selectEl = document.getElementById('embedding-provider-select')
  if (selectEl) selectEl.value = store.config.embeddings_provider
}

// Carrega modelos ao trocar para LM Studio no motor de RAG
const onRAGProviderChange = () => {
  if (store.config.rag_provider === 'lmstudio') {
    if (store.lmModels.length === 0) {
      loadLMModels()
    } else if (!store.config.rag_model) {
      store.config.rag_model = store.config.lmstudio_model || store.lmModels[0] || ''
    }
  }
}

// ── Lifecycle ──
onMounted(() => {
  loadConfig()
  refreshStatus()
  initInstallerLogs()
})

// Quando abrir a aba MODELOS, carrega automaticamente os modelos do LM Studio
// se qualquer um dos provedores já estiver configurado como lmstudio
watch(() => store.activeTab, (tab) => {
  if (tab === 'modelos' && store.lmModels.length === 0) {
    const cfg = store.config
    if (cfg.embeddings_provider === 'lmstudio' || cfg.rag_provider === 'lmstudio') {
      loadLMModels()
    }
  }
})
</script>

<template>
    <!-- NEXUS TOAST NOTIFICATION -->
    <transition name="toast">
      <div v-if="store.toast.show" 
           class="nexus-toast animate-toast-in" 
           :class="'toast-' + store.toast.type"
           @click="store.toast.show = false">
        <div class="toast-icon">
          <span v-if="store.toast.type === 'success'">💎</span>
          <span v-else-if="store.toast.type === 'error'">⚠️</span>
          <span v-else>💠</span>
        </div>
        <div class="toast-content">
          <div class="toast-title">{{ store.toast.type === 'success' ? 'Sincronizado' : store.toast.type === 'error' ? 'Alerta de Célula' : 'Nexus Info' }}</div>
          <div class="toast-msg">{{ store.toast.message }}</div>
        </div>
      </div>
    </transition>

    <main class="settings-view animate-fade-up">
    <div class="settings-header">
      <div class="brand-badge pulse-aura">LUMAESTRO PREMIER</div>
      <h1 class="gradient-text">Orquestração de IAs</h1>
      <p class="subtitle">Configurações globais e gerenciamento de contas.</p>
    </div>

    <div class="tabs-nav-glass">
      <!-- NAVEGAÇÃO DE ABAS ATUALIZADA -->
      <button v-for="tab in ['geral', 'qdrant', 'chaves', 'motores', 'rag', 'google', 'groq', 'contas', 'seguranca', 'mcp']" 
              :key="tab"
              @click="store.activeTab = tab" 
              :class="{ 'active': store.activeTab === tab }" 
              class="tab-btn-premium">
        {{ tab === 'contas' ? 'CONTAS' : tab === 'groq' ? 'GROQ LPU' : tab === 'rag' ? 'RAG/ONTOLOGIA' : tab.toUpperCase() }}
      </button>
    </div>

    <div class="content-viewport">
      <!-- ABA GERAL -->
      <section v-if="store.activeTab === 'geral'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Base da Sinfonia</h2>
        
        <div class="form-grid">
          <div class="premium-form-group">
            <label>Idioma Nativo do Agente</label>
            <select v-model="store.config.agent_language" class="maestro-input">
              <option value="Português do Brasil">Português (Brasil)</option>
              <option value="English">English</option>
              <option value="Español">Español</option>
              <option value="Français">Français</option>
              <option value="Deutsch">Deutsch</option>
              <option value="Italiano">Italiano</option>
              <option value="日本語 (Japanese)">日本語 (Japonês)</option>
            </select>
          </div>

          <div class="premium-form-group">
            <label>Caminho do Obsidian Vault</label>
            <input v-model="store.config.obsidian_vault_path" type="text" class="maestro-input" placeholder="C:\Users\...\Obsidian" />
          </div>
        </div>

        <div class="premium-form-group">
          <label>Alcance da Teia (Vizinhos): <span class="highlight-val">{{ store.config.graph_neighbor_limit }}</span></label>
          <input v-model.number="store.config.graph_neighbor_limit" type="range" min="1" max="25" step="1" class="maestro-range" />
        </div>

        <!-- SEÇÃO NEURAL -->
        <div class="sec-card neural-sec" style="margin-top: 1.5rem; margin-bottom: 2rem; border-color: rgba(139, 92, 246, 0.3); background: rgba(139, 92, 246, 0.05); padding: 1.2rem 1.6rem; box-shadow: 0 10px 40px rgba(0,0,0,0.2);">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 0.95rem; color: #fff;">🧠 Modo de Exploração Neural</h5>
              <p style="margin: 8px 0 0; font-size: 0.78rem; color: var(--p-text-dim); line-height: 1.5;">
                Ativado: Mostra resultados brutos (similaridade pura).<br/>
                Desativado: Prioriza notas que você acessa com frequência (Sinapses Fortes).
              </p>
           </div>
           <div class="sec-toggle-wrapper" @click="store.isExplorationMode = !store.isExplorationMode; toggleExplorationMode()" style="align-self: center;">
             <span v-if="store.isExplorationMode" class="sec-label-active" style="color: #a78bfa;">PURA (BRUTA) 🔍</span>
             <span v-else class="sec-label-blocked" style="color: #f9a8d4; opacity: 0.9;">SINÁPTICA 🧠</span>
             <div class="maestro-switch" :class="{'on': store.isExplorationMode}" :style="store.isExplorationMode ? 'border-color: #8b5cf6; box-shadow: 0 0 12px rgba(139, 92, 246, 0.4); background: rgba(139, 92, 246, 0.2);' : ''">
               <div class="maestro-switch-thumb" :style="store.isExplorationMode ? 'background: #a78bfa;' : ''"></div>
             </div>
           </div>
        </div>

        <div style="height: 1px; background: rgba(148,163,184,0.1); margin: 2.5rem 0 1.5rem;"></div>

        <div class="premium-form-group" style="margin-bottom: 2rem;">
          <label style="color: var(--primary); font-weight: 800; letter-spacing: 1px;">MOTOR PRIMÁRIO (ORQUESTRADOR ACP)</label>
          <p style="font-size: 0.75rem; color: var(--p-text-dim); margin-bottom: 1rem;">
             Selecione o "vetor de combustível" principal que alimenta o cérebro do Lumaestro.
          </p>
          <select v-model="store.config.primary_provider" class="maestro-input" style="border-color: rgba(59,130,246,0.3);" @change="setPrimaryProvider(store.config.primary_provider)">
            <option value="gemini">Gemini (Google DeepMind)</option>
            <option value="claude">Claude (Anthropic)</option>
            <option value="groq">Groq LPU (Ultra Latency)</option>
            <option value="lmstudio">Local Expert (LM Studio)</option>
            <option value="native">Local Nativo (Lumaestro Hybrid)</option>
          </select>
        </div>

        <button @click="save" class="btn-glow-blue">SALVAR ALTERAÇÕES GERAIS</button>

        <div class="danger-zone-compact" style="margin-top: 2rem; padding: 1.2rem; border-top: 1px solid rgba(239, 68, 68, 0.1);">
           <h3 style="color: #ef4444; font-size: 0.7rem; letter-spacing: 2px; margin-bottom: 0.6rem;">CUIDADO: ZONA DE PERIGO</h3>
           <p style="color: var(--p-text-dim); font-size: 0.7rem; margin-bottom: 0.8rem;">Deseja apagar todos os vetores e memórias do banco de dados?</p>
           <button @click="store.showResetModal = true" class="btn-reset-db">EXPURGAR BANCO VETORIAL (RESET)</button>
        </div>
      </section>

      <!-- ABA QDRANT (MEMÓRIA VETORIAL) -->
      <section v-if="store.activeTab === 'qdrant'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Memória Vetorial (Qdrant)</h2>
        <p class="subtitle-maestro" style="color: var(--p-text-dim); margin-bottom: 1.5rem; font-size: 0.9rem;">
          Configure o banco de dados que armazena o conhecimento de longo prazo e as conexões semânticas da IA.
        </p>
        
        <div class="premium-form-group">
          <label>URL do Qdrant Cloud (Instância)</label>
          <input v-model="store.config.qdrant_url" type="text" class="maestro-input" placeholder="http://qdrant-seu-id.sslip.io" />
        </div>

        <div class="premium-form-group">
          <label>Qdrant API Key (Coolify)</label>
          <input v-model="store.config.qdrant_api_key" type="password" class="maestro-input" placeholder="••••••••" />
        </div>

        <!-- SELETOR DE MOTOR DE EMBEDDINGS (direto na aba Qdrant) -->
        <div style="margin: 2rem 0; padding: 1.5rem; border-radius: 16px; border: 1px solid rgba(59,130,246,0.15); background: rgba(59,130,246,0.03);">
          <h3 style="margin: 0 0 0.5rem; font-size: 0.85rem; font-weight: 800; letter-spacing: 1px; color: #94a3b8; text-transform: uppercase;">🔬 Motor de Embeddings</h3>
          <p style="color: var(--p-text-dim); font-size: 0.8rem; margin-bottom: 1rem; line-height: 1.5;">
            Escolha se os vetores semânticos serão gerados na <strong style="color: #60a5fa;">Nuvem (Gemini)</strong> ou <strong style="color: #10b981;">Localmente (llama.cpp)</strong>.
          </p>

          <div style="display: flex; gap: 12px; flex-wrap: wrap;">
            <!-- Card NUVEM -->
            <div 
              @click="store.config.embeddings_provider !== 'gemini' ? confirmProviderChange('gemini') : null"
              style="flex: 1; min-width: 200px; padding: 1.2rem; border-radius: 14px; cursor: pointer; transition: all 0.3s; border: 2px solid; display: flex; flex-direction: column; gap: 8px;"
              :style="store.config.embeddings_provider === 'gemini' 
                ? 'border-color: #3b82f6; background: rgba(59,130,246,0.1); box-shadow: 0 0 20px rgba(59,130,246,0.15);' 
                : 'border-color: rgba(255,255,255,0.06); background: rgba(0,0,0,0.2);'"
            >
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 1.5rem;">☁️</span>
                <div>
                  <div style="font-weight: 900; font-size: 0.9rem; color: #fff;">Nuvem (Gemini)</div>
                  <div style="font-size: 0.7rem; color: #94a3b8;">gemini-embedding-2 · 3072 dim · Multimídia</div>
                </div>
              </div>
              <div v-if="store.config.embeddings_provider === 'gemini'" style="font-size: 0.65rem; font-weight: 900; color: #3b82f6; letter-spacing: 1px;">✓ ATIVO</div>
            </div>

            <!-- Card LOCAL -->
            <div 
              @click="store.config.embeddings_provider !== 'native' ? confirmProviderChange('native') : null"
              style="flex: 1; min-width: 200px; padding: 1.2rem; border-radius: 14px; cursor: pointer; transition: all 0.3s; border: 2px solid; display: flex; flex-direction: column; gap: 8px;"
              :style="store.config.embeddings_provider === 'native' 
                ? 'border-color: #10b981; background: rgba(16,185,129,0.1); box-shadow: 0 0 20px rgba(16,185,129,0.15);' 
                : 'border-color: rgba(255,255,255,0.06); background: rgba(0,0,0,0.2);'"
            >
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 1.5rem;">🖥️</span>
                <div>
                  <div style="font-weight: 900; font-size: 0.9rem; color: #fff;">Local (llama.cpp)</div>
                  <div style="font-size: 0.7rem; color: #94a3b8;">Qwen3-Embedding-0.6B-GGUF · 1024 dim · Offline</div>
                </div>
              </div>
              <div v-if="store.config.embeddings_provider === 'native'" style="font-size: 0.65rem; font-weight: 900; color: #10b981; letter-spacing: 1px;">✓ ATIVO</div>
            </div>
          </div>

          <small style="display: block; margin-top: 0.8rem; color: var(--p-text-dim); font-size: 0.72rem; line-height: 1.4;">
            ⚠️ Trocar o motor exige um <b>Reset do Banco Qdrant</b> (dimensão vetorial muda). O modo Local não processa fotos/vídeos.
          </small>
        </div>

        <button @click="save" class="btn-glow-blue" style="width: 100%; margin-bottom: 1rem;">SALVAR CONFIGURAÇÃO VETORIAL</button>

        <!-- PAINEL DE DIAGNÓSTICO VETORIAL -->
        <div class="diagnostic-panel-premium glass-panel" style="margin-top: 2rem; border: 1px solid rgba(59, 130, 246, 0.2);">
          <div class="diag-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
            <div>
              <h3 style="margin: 0; color: #fff; font-size: 1.1rem;">Integridade Vetorial</h3>
              <p style="margin: 0; font-size: 0.8rem; color: var(--p-text-dim);">Valide o pipeline Gemini + Qdrant Cloud</p>
            </div>
            <button @click="runDiagnostic" :disabled="store.isDiagnosing" class="btn-diag" style="padding: 0.6rem 1.2rem; border-radius: 12px; background: rgba(59, 130, 246, 0.1); border: 1px solid var(--primary); color: #fff; cursor: pointer;">
              <span v-if="!store.isDiagnosing">⚡ EXECUTAR STRESS TEST</span>
              <span v-else>⏳ PROCESSANDO...</span>
            </button>
          </div>

          <div v-if="store.diagnosticResult" class="diag-results animate-fade-in" style="background: rgba(0,0,0,0.3); padding: 1.5rem; border-radius: 15px;">
            <div v-if="store.diagnosticResult.success" class="res-success">
               <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; margin-bottom: 1rem;">
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">GEMINI EMBED</span>
                    <b style="color: #4ade80;">{{ store.diagnosticResult.embed_ms }}ms</b>
                  </div>
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">QDRANT UPSERT</span>
                    <b style="color: #4ade80;">{{ store.diagnosticResult.qdrant_ms }}ms</b>
                  </div>
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">TOTAL CICLO</span>
                    <b style="color: var(--primary);">{{ store.diagnosticResult.total_ms }}ms</b>
                  </div>
               </div>
               <div class="vector-preview">
                  <label style="font-size: 0.7rem; color: var(--p-text-dim);">VETOR GERADO (DUMP 5-DIM):</label>
                  <code style="display: block; background: #000; padding: 0.8rem; border-radius: 10px; font-size: 0.8rem; color: #3b82f6; margin-top: 0.5rem; border: 1px solid rgba(59, 130, 246, 0.3);">
                    {{ store.diagnosticResult.vector_preview }}...
                  </code>
               </div>
            </div>
            <div v-else class="res-error" style="color: #ef4444; font-size: 0.9rem;">
              ❌ ERRO NO DIAGNÓSTICO: {{ store.diagnosticResult.error }}
            </div>
          </div>
        </div>

        <div class="danger-zone-compact" style="margin-top: 2rem; padding: 1.5rem; border: 1px solid rgba(239, 68, 68, 0.1); border-radius: 12px; background: rgba(239, 68, 68, 0.02);">
           <h3 style="color: #ef4444; font-size: 0.7rem; letter-spacing: 2px; margin-bottom: 0.5rem;">ZONA DE PURGA</h3>
           <p style="color: var(--p-text-dim); font-size: 0.7rem; margin-bottom: 1rem;">Deseja apagar todos os vetores deste banco? Esta ação é irreversível.</p>
           <button @click="store.showResetModal = true" class="btn-reset-db" style="padding: 10px 20px; font-size: 0.7rem;">RESETAR BANCO QDRANT</button>
        </div>
      </section>

      <!-- ABA CHAVES (INJEÇÃO DE CHAVES DIRETAS) -->
      <section v-if="store.activeTab === 'chaves'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Conexões Legadas / Manuais</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Gerencie injeções diretas de tokens para provedores manuais ou integrações via bypass.
        </p>
        
        <div class="premium-form-group">
          <label>Claude API Key (Anthropic)</label>
          <input v-model="store.config.claude_api_key" type="password" class="maestro-input" placeholder="sk-ant-..." :disabled="!store.config.use_claude_api_key" />
        </div>

        <div class="sec-card" style="margin-bottom: 2.5rem; padding: 1.5rem 2.5rem; border-color: rgba(217, 119, 6, 0.2);">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #fff;">Claude API Mode</h5>
              <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Habilitar injeção direta de chave para modelos Sonnet/Haiku.</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="store.config.use_claude_api_key" />
              <span class="m-slider-sec" style="background: rgba(217, 119, 6, 0.1);"></span>
           </label>
        </div>

        <button @click="save" class="btn-glow-blue" style="width: 100%;">SALVAR CHAVES MANUAIS</button>
      </section>

      <!-- ABA GOOGLE (GEMINI NEXUS) -->
      <section v-if="store.activeTab === 'google'" class="glass-panel animate-slide-up" style="border-color: rgba(59, 130, 246, 0.2);">
        <h2 class="section-title" style="color: #60a5fa;">Google Gemini Nexus 💎</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Infraestrutura nativa do Google. Gerencie pools de chaves e o sistema de failover automático.
        </p>

        <div class="premium-form-group">
          <label style="display: flex; align-items: center; justify-content: space-between;">
            <span>Gemini API Keys (Pool de Failover)</span>
            <span v-if="store.geminiKeyCount > 0" style="background: rgba(59, 130, 246, 0.15); color: #3b82f6; padding: 3px 10px; border-radius: 8px; font-size: 0.65rem; font-weight: 900; letter-spacing: 1px;">
              {{ store.geminiKeyCount }} CHAVE{{ store.geminiKeyCount > 1 ? 'S' : '' }} ATIVA{{ store.geminiKeyCount > 1 ? 'S' : '' }} 🔑
            </span>
          </label>
          <textarea 
            v-model="store.config.gemini_api_key" 
            class="maestro-input" 
            placeholder="AIzaSy..., AIzaSy..."
            rows="3"
            style="resize: vertical; font-family: monospace; font-size: 0.85rem; line-height: 1.6; border-color: rgba(59, 130, 246, 0.3);"
          ></textarea>
        </div>

        <div class="premium-form-group" style="margin-top: 2rem;">
          <label>Modelo Gemini de Prioridade (Boot)</label>
          <select v-model="store.config.gemini_model" class="maestro-input" style="border-color: rgba(59, 130, 246, 0.3);">
            <option v-for="model in availableGoogleModels" :key="model.id" :value="model.id">
              {{ model.icon }} {{ model.id }} ({{ model.label }})
            </option>
          </select>
        </div>

        <div class="sec-card" style="margin-top: 2rem; margin-bottom: 2.5rem; padding: 1.2rem 1.6rem; border-color: rgba(59, 130, 246, 0.2);">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 0.95rem; color: #fff;">Modo Autônomo API</h5>
              <p style="margin: 8px 0 0; font-size: 0.78rem; color: var(--p-text-dim);">Usar chave legada em vez de Sessões de Contas (OAuth).</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="store.config.use_gemini_api_key" />
              <span class="m-slider-sec" style="background: rgba(59, 130, 246, 0.1);"></span>
           </label>
        </div>

        <div class="premium-form-group">
          <label style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem;">
            <span>Gerenciador da Frota Google (Cascata)</span>
            <span style="font-size: 0.6rem; color: #3b82f6; font-weight: 900; letter-spacing: 1px;">SINFORNIA GOOGLE ATIVA 🛡️</span>
          </label>
          
          <div class="google-fleet-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 15px;">
            <div v-for="model in availableGoogleModels" :key="model.id" 
                 class="fleet-item-card" 
                 :class="{ 'active': isGoogleModelActive(model.id) }"
                 @click="toggleGoogleModel(model.id)"
                 style="background: rgba(255,255,255,0.03); border: 1px solid rgba(59, 130, 246, 0.1); border-radius: 12px; padding: 15px; cursor: pointer; transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1); position: relative; overflow: hidden; height: 100px; display: flex; flex-direction: column; justify-content: space-between;">
              <div class="fleet-item-inner" style="position: relative; z-index: 2; height: 100%; display: flex; flex-direction: column; justify-content: space-between;">
                <div style="display: flex; align-items: flex-start; justify-content: space-between;">
                  <span style="font-size: 1.4rem;">{{ model.icon }}</span>
                  <div style="display: flex; flex-direction: column; align-items: flex-end; gap: 6px;">
                    <div class="maestro-switch mini" :class="{ 'on': isGoogleModelActive(model.id) }">
                      <div class="maestro-switch-thumb"></div>
                    </div>
                    <div style="font-size: 0.5rem; background: rgba(59, 130, 246, 0.2); color: #3b82f6; padding: 2px 6px; border-radius: 4px; font-weight: 900; letter-spacing: 1px; text-transform: uppercase;">{{ model.tier }}</div>
                  </div>
                </div>
                <div>
                  <div style="font-weight: 800; font-size: 0.75rem; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-bottom: 2px; opacity: 0.9;">{{ model.id }}</div>
                  <div style="font-size: 0.6rem; color: var(--p-text-dim); font-weight: 600; letter-spacing: 0.5px;">{{ model.label }}</div>
                </div>
              </div>
              <div v-if="isGoogleModelActive(model.id)" style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; background: radial-gradient(circle at top right, rgba(59, 130, 246, 0.1) 0%, transparent 70%); pointer-events: none;"></div>
            </div>
          </div>
        </div>

        <button @click="save" class="btn-glow-blue" style="margin-top: 2rem; width: 100%;">SALVAR CONFIGURAÇÃO GOOGLE</button>
      </section>

      <!-- ABA GROQ (TURBO LPU) -->
      <section v-if="store.activeTab === 'groq'" class="glass-panel animate-slide-up" style="border-color: rgba(245, 158, 11, 0.2);">
        <h2 class="section-title" style="color: #f59e0b;">Groq Turbo LPU 🏎️</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Infraestrutura de inferência ultra-rápida. Use modelos de 70B com latência quase zero.
        </p>

        <div class="premium-form-group">
          <label style="display: flex; align-items: center; justify-content: space-between;">
            <span>Groq API Keys (Pool de Rotação)</span>
            <span v-if="store.groqKeyCount > 0" style="background: rgba(245, 158, 11, 0.15); color: #f59e0b; padding: 3px 10px; border-radius: 8px; font-size: 0.65rem; font-weight: 900; letter-spacing: 1px;">
              {{ store.groqKeyCount }} CHAVE{{ store.groqKeyCount > 1 ? 'S' : '' }} ATIVA{{ store.groqKeyCount > 1 ? 'S' : '' }} 🏎️
            </span>
          </label>
          <textarea 
            v-model="store.config.groq_api_key" 
            class="maestro-input" 
            placeholder="gsk_..., gsk_..."
            rows="3"
            style="resize: vertical; font-family: monospace; font-size: 0.85rem; line-height: 1.6; border-color: rgba(245, 158, 11, 0.3);"
          ></textarea>
          <small style="color: var(--p-text-dim); font-size: 0.7rem; margin-top: 8px; display: block;">
            Separe várias chaves por vírgula para ativar a **Rotação Automática** em caso de Rate Limit.
          </small>
        </div>

        <div class="premium-form-group" style="margin-top: 2rem;">
          <label>Modelo Groq de Prioridade (Boot)</label>
          <select v-model="store.config.groq_model" class="maestro-input" style="border-color: rgba(245, 158, 11, 0.3);">
            <option v-for="model in availableGroqModels" :key="model.id" :value="model.id">
              {{ model.icon }} {{ model.id }} ({{ model.label }})
            </option>
          </select>
        </div>

        <div class="premium-form-group" style="margin-top: 2.5rem;">
          <label style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem;">
            <span>Gerenciador da Frota de Resiliência (Cascata)</span>
            <span style="font-size: 0.6rem; color: #f59e0b; font-weight: 900; letter-spacing: 1px;">MODO GUERRA TOTAL ATIVO 🛡️</span>
          </label>
          
          <div class="groq-fleet-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(210px, 1fr)); gap: 15px;">
            <div v-for="model in availableGroqModels" :key="model.id" 
                 class="fleet-item-card" 
                 :class="{ 'active': isModelActive(model.id) }"
                 @click="toggleGroqModel(model.id)"
                 style="background: rgba(255,255,255,0.03); border: 1px solid rgba(245, 158, 11, 0.1); border-radius: 12px; padding: 15px; cursor: pointer; transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1); position: relative; overflow: hidden; height: 100px; display: flex; flex-direction: column; justify-content: space-between;">
              
              <div class="fleet-item-inner" style="position: relative; z-index: 2; height: 100%; display: flex; flex-direction: column; justify-content: space-between;">
                <div style="display: flex; align-items: flex-start; justify-content: space-between;">
                  <span style="font-size: 1.4rem;">{{ model.icon }}</span>
                  <div style="display: flex; flex-direction: column; align-items: flex-end; gap: 6px;">
                    <div class="maestro-switch mini" :class="{ 'on': isModelActive(model.id) }">
                      <div class="maestro-switch-thumb"></div>
                    </div>
                    <div style="font-size: 0.5rem; background: rgba(245, 158, 11, 0.2); color: #f59e0b; padding: 2px 6px; border-radius: 4px; font-weight: 900; letter-spacing: 1px; text-transform: uppercase;">{{ model.tier }}</div>
                  </div>
                </div>
                
                <div>
                  <div style="font-weight: 800; font-size: 0.75rem; color: #fff; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-bottom: 2px; opacity: 0.9;">{{ model.id.split('/').pop() }}</div>
                  <div style="font-size: 0.6rem; color: var(--p-text-dim); font-weight: 600; letter-spacing: 0.5px;">{{ model.label }}</div>
                </div>
              </div>

              <!-- Glow Effect -->
              <div v-if="isModelActive(model.id)" style="position: absolute; top: 0; left: 0; width: 100%; height: 100%; background: radial-gradient(circle at top right, rgba(245, 158, 11, 0.1) 0%, transparent 70%); pointer-events: none;"></div>
            </div>
          </div>
          <small style="color: var(--p-text-dim); font-size: 0.7rem; margin-top: 15px; display: block; line-height: 1.5;">
            Clique nos cards para ativar/desativar modelos na cascata. O Lumaestro percorrerá **apenas** os modelos ativos quando as cotas estourarem.
          </small>
        </div>

        <div class="sec-card" style="margin-top: 2rem; padding: 1.5rem; border: 1px solid rgba(245, 158, 11, 0.1); background: rgba(245, 158, 11, 0.02);">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 0.95rem; color: #fff;">Status da LPU</h5>
              <div style="display: flex; gap: 15px; margin-top: 10px;">
                <div class="stat-box" style="background: rgba(0,0,0,0.2); padding: 10px; border-radius: 8px; min-width: 100px; text-align: center;">
                  <div style="font-size: 0.6rem; color: #94a3b8;">POOL</div>
                  <div style="font-weight: 900; color: #f59e0b;">{{ store.groqKeyCount }}</div>
                </div>
                <div class="stat-box" style="background: rgba(0,0,0,0.2); padding: 10px; border-radius: 8px; min-width: 100px; text-align: center;">
                  <div style="font-size: 0.6rem; color: #94a3b8;">ROTAÇÃO</div>
                  <div style="font-weight: 900; color: #4ade80;">ATIVO ✓</div>
                </div>
              </div>
           </div>
        </div>

        <button @click="save" class="btn-glow-blue" style="margin-top: 2rem; width: 100%; filter: hue-rotate(200deg);">SALVAR CONFIGURAÇÃO GROQ</button>
      </section>

      <!-- ABA MOTORES (O CÉREBRO) -->
      <section v-if="store.activeTab === 'motores'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Hub de Motores e Orquestração</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Estação de controle dos núcleos de inteligência. Acompanhe a disponibilidade binária e inicie os daemons em background.
        </p>

        <div class="engine-cards-stack">
           <div v-for="tool in ['gemini', 'claude', 'lmstudio']" :key="tool" class="profile-card engine-showcase-card" :class="tool">
              <div class="engine-glow-backdrop"></div>
              
              <div style="position: relative; z-index: 2; height: 100%; display: flex; flex-direction: column;">
                <div style="display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 1.5rem;">
                   <div style="display: flex; align-items: center; gap: 1rem;">
                      <div class="avatar-glow maestro-engine-icon" :style="tool === 'gemini' ? 'background: linear-gradient(135deg, #3b82f6, #8b5cf6)' : tool === 'claude' ? 'background: linear-gradient(135deg, #f97316, #ea580c)' : tool === 'groq' ? 'background: linear-gradient(135deg, #f59e0b, #d97706)' : 'background: linear-gradient(135deg, #10b981, #059669)'">
                         {{ tool === 'gemini' ? '⚡' : tool === 'claude' ? '🦾' : tool === 'groq' ? '🏎️' : '🤖' }}
                      </div>
                      <div>
                        <h4 style="margin: 0; font-weight: 900; color: #fff; font-size: 1.3rem; letter-spacing: 2px;">{{ tool === 'lmstudio' ? 'LM STUDIO' : tool.toUpperCase() }}</h4>
                        <div v-if="tool === 'gemini' || tool === 'claude'" class="engine-status-badge" :style="store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? '' : 'border-color: rgba(245, 158, 11, 0.3); background: rgba(245, 158, 11, 0.05); color: #f59e0b;') : 'border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.05);'">
                          <span class="status-dot" :style="store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? '' : 'background: #f59e0b; box-shadow: none;') : 'background: #ef4444; box-shadow: none;'"></span> 
                          {{ store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? 'SISTEMA PRONTO' : 'NÃO AUTENTICADO') : 'NÃO INSTALADO' }}
                        </div>
                        <div v-else-if="tool === 'groq'" class="engine-status-badge" style="border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05); color: #10b981;">
                          <span class="status-dot" style="background: #10b981;"></span>
                          SISTEMA PRONTO
                        </div>
                        <div v-else-if="tool === 'groq'" class="engine-status-badge" :style="store.groqKeyCount > 0 ? 'border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05); color: #10b981;' : 'border-color: rgba(245, 158, 11, 0.3); background: rgba(245, 158, 11, 0.05); color: #f59e0b;'">
                          <span class="status-dot" :style="store.groqKeyCount > 0 ? 'background: #10b981;' : 'background: #f59e0b; box-shadow: none;'"></span> 
                          {{ store.groqKeyCount > 0 ? 'POOL CONFIGURADO' : 'AGUARDANDO CHAVE' }}
                        </div>
                        <div v-else class="engine-status-badge" :style="(store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05);' : 'border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.05);'">
                          <span class="status-dot" :style="(store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'background: #10b981;' : 'background: #ef4444; box-shadow: none;'"></span> 
                          {{ (store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'CONFIGURADO ✓' : 'DESABILITADO' }}
                        </div>
                      </div>
                   </div>
                   
                   <!-- Auto-Start Switch -->
                   <div v-if="tool !== 'groq'" class="auto-boot-container" @click="toggleAutoStart(tool)" title="Inicia o motor automaticamente assim que você abre o Lumaestro" style="flex-shrink: 0;">
                     <div style="display: flex; align-items: center; gap: 8px; justify-content: flex-end;">
                       <span style="font-size: 0.65rem; color: var(--p-text-dim); font-weight: 900; letter-spacing: 1px; white-space: nowrap;">AUTO-BOOT</span>
                       <div class="maestro-switch" :class="{ 'on': isAutoStart(tool) }">
                         <div class="maestro-switch-thumb"></div>
                       </div>
                     </div>
                     <span v-if="isAutoStart(tool)" style="font-size: 0.55rem; color: #3b82f6; font-weight: bold; opacity: 0.9; align-self: flex-end; white-space: nowrap; margin-top: 4px;">LIGA SOZINHO ⚡</span>
                     <span v-else style="font-size: 0.55rem; color: #64748b; font-weight: bold; opacity: 0.8; align-self: flex-end; white-space: nowrap; margin-top: 4px;">PARTIDA MANUAL ✋</span>
                   </div>
                </div>
                
                <p style="color: #cbd5e1; font-size: 0.85rem; margin-bottom: 2.5rem; line-height: 1.6; font-weight: 300; flex-grow: 1;">
                   <template v-if="tool === 'gemini'">
                     Motor de Inteligência Central. Responsável pela execução de rotinas autônomas e retenção de contexto contínuo (ACP) em background.
                   </template>
                   <template v-else-if="tool === 'claude'">
                     Motor Analítico Avançado. Infraestrutura secundária focada em modelagem pesada, testes lógicos e geração de códigos complexos.
                   </template>
                   <template v-else-if="tool === 'groq'">
                     Motor de Inferência Turbo LPU. Near-instantaneous cloud execution para modelos complexos de 70B e 32B. Perfeito para RAG em tempo real.
                   </template>
                   <template v-else>
                     Motor Local OpenAI-compatível. Execute modelos privados sem custo de API. Conecte ao LM Studio para usar Llama, Mistral e outras LLMs.
                   </template>
                </p>

                <!-- LM Studio Specific Config -->
                <div v-if="tool === 'lmstudio'" style="display: flex; flex-direction: column; gap: 1rem; margin-bottom: 1.5rem; padding-bottom: 1.5rem; border-bottom: 1px solid rgba(255,255,255,0.05);">
                  <div style="display: flex; gap: 8px; align-items: center;">
                    <input v-model="store.config.lmstudio_url" type="text" class="maestro-input" placeholder="http://localhost:1234" style="flex: 1; padding: 8px 12px; font-size: 0.8rem;" />
                    <button @click="loadLMModels" :disabled="store.lmLoadingModels" style="background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #10b981; border-radius: 8px; padding: 6px 14px; font-size: 0.7rem; cursor: pointer; white-space: nowrap;">
                      {{ store.lmLoadingModels ? '⏳' : '🔄' }} MODELOS
                    </button>
                  </div>
                  <select v-if="store.lmModels.length > 0" v-model="store.config.lmstudio_model" class="maestro-input" style="padding: 8px 12px; font-size: 0.8rem;">
                    <option value="">-- Padrão do LM Studio --</option>
                    <option v-for="m in store.lmModels" :key="m" :value="m">{{ m }}</option>
                  </select>
                  <input v-else v-model="store.config.lmstudio_model" type="text" class="maestro-input" placeholder="ID do modelo" style="padding: 8px 12px; font-size: 0.8rem;" />
                </div>

                <div style="display: flex; gap: 12px; margin-top: auto;">
                   <button v-if="tool !== 'groq'" @click="install(tool)" class="unit-btn-solid" style="flex: 1.5;">
                     {{ tool === 'lmstudio' ? 'SALVAR CONFIG' : 'SINCRONIZAR' }}
                   </button>
                   <button v-if="tool !== 'lmstudio' && tool !== 'groq' && store.status.tools[tool]" @click="setup(tool)" class="unit-btn-glow" :style="getAuthStyle(tool)" style="flex: 1;">
                      {{ getAuthLabel(tool) }}
                   </button>
                   <button v-if="tool === 'groq'" @click="store.activeTab = 'groq'" class="unit-btn-glow" style="background: rgba(245, 158, 11, 0.1); border-color: rgba(245, 158, 11, 0.3); color: #f59e0b; flex: 1;">
                      RECONFIGURAR 🏎️
                   </button>
                   <button v-if="tool === 'lmstudio'" @click="testLMStudio" :disabled="store.lmTesting" style="background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #10b981; border-radius: 8px; padding: 8px 16px; font-size: 0.75rem; font-weight: 900; cursor: pointer; flex: 1;">
                      {{ store.lmTesting ? '⏳ TESTANDO' : '⚡ TESTAR' }}
                   </button>
                </div>

                <!-- Test Result Feedback -->
                <div v-if="tool === 'lmstudio' && store.lmTestResult" style="margin-top: 1rem; padding-top: 1rem; border-top: 1px solid rgba(255,255,255,0.05); font-size: 0.75rem;">
                  <div v-if="store.lmTestResult.success" style="color: #4ade80;">
                    ✅ OK ({{ store.lmTestResult.latency_ms }}ms) — {{ store.lmTestResult.capabilities.join(', ') }}
                  </div>
                  <div v-else style="color: #ef4444;">
                    ❌ {{ store.lmTestResult.error || 'Falha ao conectar' }}
                  </div>
                </div>
              </div>
           </div>
        </div>
      </section>

      <!-- ABA RAG/ONTOLOGIA (POOL ATIVO) -->
      <!-- ABA RAG/ONTOLOGIA (EXCLUSIVO) -->
      <section v-if="store.activeTab === 'rag'" class="glass-panel animate-slide-up">
        <h2 class="section-title">RAG / Ontologia (Extrator de Triplas)</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Configure o cérebro semântico que processa suas notas, encontra conexões neurais e constrói o Grafo de Conhecimento.
        </p>





        <!-- SELETOR DE MOTOR DE RAG (Cards Visuais) -->
        <div style="margin: 2rem 0; padding: 1.5rem; border-radius: 16px; border: 1px solid rgba(139, 92, 246, 0.15); background: rgba(139, 92, 246, 0.03); transition: all 0.4s;"
             :style="store.config.hybrid_failover_enabled ? 'opacity: 0.3; filter: grayscale(1); pointer-events: none;' : ''">
          <div style="display: flex; gap: 12px; flex-wrap: wrap;">
            <!-- Card NUVEM (Gemini) -->
            <div 
              @click="store.config.rag_provider = 'gemini'; save()"
              style="flex: 1; min-width: 200px; padding: 1.2rem; border-radius: 14px; cursor: pointer; transition: all 0.3s; border: 2px solid; display: flex; flex-direction: column; gap: 8px;"
              :style="store.config.rag_provider === 'gemini' 
                ? 'border-color: #8b5cf6; background: rgba(139, 92, 246, 0.1); box-shadow: 0 0 20px rgba(139, 92, 246, 0.15);' 
                : 'border-color: rgba(255,255,255,0.06); background: rgba(0,0,0,0.2);'"
            >
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 1.5rem;">🌌</span>
                <div>
                  <div style="font-weight: 900; font-size: 0.9rem; color: #fff;">Gemini (Nuvem)</div>
                  <div style="font-size: 0.7rem; color: #94a3b8;">Flash Lite 3.1 · Híbrido · Rápido</div>
                </div>
              </div>
              <div v-if="store.config.rag_provider === 'gemini'" style="font-size: 0.65rem; font-weight: 900; color: #8b5cf6; letter-spacing: 1px;">✓ ATIVO</div>
            </div>

            <!-- Card HÍBRIDO LOCAL (Native) -->
            <div 
              @click="store.config.rag_provider = 'native'; save()"
              style="flex: 1; min-width: 200px; padding: 1.2rem; border-radius: 14px; cursor: pointer; transition: all 0.3s; border: 2px solid; display: flex; flex-direction: column; gap: 8px;"
              :style="store.config.rag_provider === 'native' 
                ? 'border-color: #ec4899; background: rgba(236, 72, 153, 0.1); box-shadow: 0 0 20px rgba(236, 72, 153, 0.15);' 
                : 'border-color: rgba(255,255,255,0.06); background: rgba(0,0,0,0.2);'"
            >
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 1.5rem;">🛰️</span>
                <div>
                  <div style="font-weight: 900; font-size: 0.9rem; color: #fff;">LLM Local (Lumaestro)</div>
                  <div style="font-size: 0.7rem; color: #94a3b8;">Claude Distilled (Qwen 3.5 Reasoning)</div>
                </div>
              </div>
              <div v-if="store.config.rag_provider === 'native'" style="font-size: 0.65rem; font-weight: 900; color: #ec4899; letter-spacing: 1px;">✓ EXPERT ATIVO</div>
            </div>

            <!-- Card TURBO LPU (Groq) -->
            <div 
              @click="store.config.rag_provider = 'groq'; save()"
              style="flex: 1; min-width: 200px; padding: 1.2rem; border-radius: 14px; cursor: pointer; transition: all 0.3s; border: 2px solid; display: flex; flex-direction: column; gap: 8px;"
              :style="store.config.rag_provider === 'groq' 
                ? 'border-color: #f59e0b; background: rgba(245, 158, 11, 0.1); box-shadow: 0 0 20px rgba(245, 158, 11, 0.15);' 
                : 'border-color: rgba(255,255,255,0.06); background: rgba(0,0,0,0.2);'"
            >
              <div style="display: flex; align-items: center; gap: 10px;">
                <span style="font-size: 1.5rem;">🏎️</span>
                <div>
                  <div style="font-weight: 900; font-size: 0.9rem; color: #fff;">Groq Turbo LPU</div>
                  <div style="font-size: 0.7rem; color: #94a3b8;">Llama 3.3 70B · Resiliência Ativa</div>
                </div>
              </div>
              <div v-if="store.config.rag_provider === 'groq'" style="font-size: 0.65rem; font-weight: 900; color: #f59e0b; letter-spacing: 1px;">✓ TURBO ATIVO</div>
            </div>
          </div>
        </div>

        <!-- NEXUS SHIELD PROTOCOL (FAILOVER HÍBRIDO) -->
        <div class="glass-panel" style="margin-top: 2rem; border-color: rgba(16, 185, 129, 0.2); background: rgba(5, 15, 15, 0.6); padding: 2rem; border-radius: 24px; box-shadow: 0 20px 50px rgba(0,0,0,0.3);">
          <div style="display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 2rem;">
            <div style="display: flex; align-items: center; gap: 18px;">
              <div style="background: rgba(16, 185, 129, 0.1); padding: 12px; border-radius: 16px; border: 1px solid rgba(16, 185, 129, 0.2); box-shadow: 0 0 20px rgba(16, 185, 129, 0.1);">
                <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="#10b981" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                </svg>
              </div>
              <div>
                <h3 style="margin: 0; font-size: 1.35rem; font-weight: 900; color: #fff; letter-spacing: -0.5px;">Nexus Shield Protocol</h3>
                <div style="display: flex; align-items: center; gap: 8px; margin-top: 4px;">
                  <span v-if="store.config.hybrid_failover_enabled" style="width: 8px; height: 8px; background: #10b981; border-radius: 50%; box-shadow: 0 0 10px #10b981; animation: pulse 2s infinite;"></span>
                  <span :style="{ color: store.config.hybrid_failover_enabled ? '#10b981' : '#64748b' }" style="font-size: 0.75rem; font-weight: 900; letter-spacing: 1.5px; text-transform: uppercase;">
                    {{ store.config.hybrid_failover_enabled ? 'Modo Sobrevivência Ativo' : 'Shield em Standby' }}
                  </span>
                </div>
              </div>
            </div>
            
            <div class="sec-toggle-wrapper" @click="store.config.hybrid_failover_enabled = !store.config.hybrid_failover_enabled; save()" style="cursor: pointer;">
              <div class="maestro-switch" :class="{ 'on': store.config.hybrid_failover_enabled }" style="width: 50px; height: 26px;">
                <div class="maestro-switch-thumb" 
                     style="width: 18px; height: 18px; transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);" 
                     :style="store.config.hybrid_failover_enabled ? 'transform: translateX(28px); background: #3b82f6; box-shadow: 0 0 10px rgba(59, 130, 246, 0.5);' : 'transform: translateX(0);'"></div>
              </div>
            </div>
          </div>
          
          <p style="color: #94a3b8; font-size: 0.88rem; line-height: 1.6; margin-bottom: 2rem; max-width: 600px;">
            O Shield monitora falhas de API e esgotamento de cotas. Em caso de queda, o Lumaestro aciona automaticamente o próximo motor da hierarquia abaixo.
          </p>

          <!-- LISTA DE MOTORES NEXUS -->
          <div style="display: flex; flex-direction: column; gap: 12px; transition: all 0.4s;" 
               :style="!store.config.hybrid_failover_enabled ? 'opacity: 0.3; filter: grayscale(1); pointer-events: none;' : ''">
             
             <div v-for="(prov, idx) in store.config.failover_priority" :key="prov" 
                  class="animate-slide-up"
                  style="display: flex; align-items: center; gap: 20px; padding: 4px 0;">
                
                <span style="font-size: 0.75rem; font-weight: 800; color: #10b981; min-width: 30px; opacity: 0.6;">#{{ idx + 1 }}</span>
                
                <div @click="movePriority(idx)" 
                     style="flex: 1; display: flex; align-items: center; justify-content: space-between; padding: 16px 24px; background: rgba(15, 23, 42, 0.8); border: 1px solid rgba(255,255,255,0.05); border-radius: 16px; cursor: pointer; transition: all 0.2s; box-shadow: 0 4px 12px rgba(0,0,0,0.1);"
                     onmouseover="this.style.borderColor='rgba(16, 185, 129, 0.3)'; this.style.transform='translateX(4px)';"
                     onmouseout="this.style.borderColor='rgba(255,255,255,0.05)'; this.style.transform='translateX(0)';"
                >
                  <div style="display: flex; align-items: center; gap: 16px;">
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" :stroke="prov === 'groq' ? '#f59e0b' : prov === 'gemini' ? '#3b82f6' : '#ec4899'" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
                    </svg>
                    <span style="font-size: 0.95rem; font-weight: 900; letter-spacing: 1px; color: #fff;">
                      {{ prov === 'native' ? 'LOCAL (DAEMON)' : prov.toUpperCase() }}
                    </span>
                  </div>
                  <div style="display: flex; align-items: center; gap: 4px;">
                     <span style="color: #10b981; font-weight: 900; font-size: 1.2rem; opacity: 0.5;">→</span>
                  </div>
                </div>

                <button v-if="store.config.failover_priority.length > 1" 
                        @click="removePriority(idx)" 
                        style="background: transparent; border: none; color: #334155; cursor: pointer; padding: 10px; font-size: 1.2rem; transition: color 0.2s;"
                        onmouseover="this.style.color='#ef4444'"
                        onmouseout="this.style.color='#334155'"
                >
                  ✕
                </button>
             </div>

             <!-- FOOTER ACTION -->
             <div style="margin-top: 1rem; border-top: 1px solid rgba(255,255,255,0.05); padding-top: 1.5rem;">
                <button @click="resetPriority" style="width: 100%; height: 50px; background: rgba(255,255,255,0.02); border: 1px dashed rgba(16, 185, 129, 0.2); border-radius: 14px; color: #64748b; font-size: 0.75rem; font-weight: 900; letter-spacing: 2px; cursor: pointer; transition: all 0.3s; display: flex; align-items: center; justify-content: center; gap: 10px;"
                        onmouseover="this.style.background='rgba(16, 185, 129, 0.05)'; this.style.color='#10b981';"
                        onmouseout="this.style.background='rgba(255,255,255,0.02)'; this.style.color='#64748b';">
                   RESTAURAR PADRÕES 🛡️
                </button>
             </div>
          </div>
        </div>

        <!-- CONFIGURAÇÕES ESPECÍFICAS (Apenas Modelos Locais requiridos) -->
        <div v-if="store.config.rag_provider === 'lmstudio'" class="premium-form-group" style="margin-top: 2rem;">
          <label style="color: #ef4444;">Modelo de chat RAG (LM Studio)</label>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <input v-model="store.config.rag_model" placeholder="Ex: google/gemma-4-26b-a4b" class="maestro-input" style="flex: 1; border-color: rgba(239, 68, 68, 0.3);" />
            <select v-if="store.lmModels.length > 0" v-model="store.config.rag_model" class="maestro-input" style="max-width: 220px; border-color: rgba(239, 68, 68, 0.3);">
              <option value="">-- selecionar do LM Studio --</option>
              <option v-for="m in store.lmModels" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>
          <p style="font-size: 0.7rem; color: #94a3b8; margin-top: 8px;">*Obrigatório o modelo estar rodando no servidor local.</p>
        </div>

        <button @click="save" class="btn-glow-blue" style="width: 100%; margin-top: 3rem;">SALVAR CONFIGURAÇÃO DE RAG</button>
      </section>

      <!-- ABA IDENTIDADES (MULTIME-PROVEDOR) -->
      <section v-if="store.activeTab === 'contas'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Gerenciador de Contas</h2>
        <p class="subtitle-maestro" style="color: var(--p-text-dim); margin-bottom: 25px; font-size: 0.9rem;">
          Gerencie múltiplas contas e perfis para alternar quotas e contextos entre projetos.
        </p>

        <!-- SELETOR DE PROVEDOR (NEXUS PILLS) -->
        <div class="provider-selector-wrap">
           <button v-for="p in accountProviders" :key="p.id" 
                   @click="selectedAccountProvider = p.id"
                   class="provider-pill"
                   :class="{ active: selectedAccountProvider === p.id }"
                   :style="selectedAccountProvider === p.id ? { '--brand-color': `var(--brand-${p.id})`, '--brand-glow': `var(--brand-${p.id})44` } : {}">
             <span>{{ p.icon }}</span>
             {{ p.label }}
           </button>
        </div>

        <!-- CAIXA DE CRIAÇÃO NEXUS -->
        <div class="identity-creation-nexus" :style="{ '--brand-color': `var(--brand-${selectedAccountProvider})`, '--brand-glow': `var(--brand-${selectedAccountProvider})44` }">
          <input 
            v-model="store.newAccName" 
            placeholder="Nome da Nova Identidade (Trabalho, Pessoal...)" 
            class="nexus-input" 
            @keyup.enter="handleAddAccount(selectedAccountProvider)" 
          />
          <button @click="handleAddAccount(selectedAccountProvider)" class="btn-nexus-create">
            CRIAR PERFIL {{ accountProviders.find(p => p.id === selectedAccountProvider)?.icon }}
          </button>
        </div>

        <!-- GRADE DE IDENTIDADES -->
        <div class="accounts-grid-premium">
          <div v-if="filteredIdentities.length === 0" style="grid-column: 1/-1; text-align: center; padding: 4rem 2rem; border-radius: 24px; border: 1px dashed rgba(255,255,255,0.05); color: var(--p-text-dim);">
            <div style="font-size: 2rem; margin-bottom: 1rem; opacity: 0.3;">🎭</div>
            O Nexus de {{ selectedAccountProvider.toUpperCase() }} está vazio.
          </div>

          <div v-for="acc in filteredIdentities" :key="acc.name" 
               class="identity-profile-card" 
               :class="{ 'is-active': acc.active }"
               :style="{ 
                 '--brand-color': `var(--brand-${acc.provider})`, 
                 '--brand-glow': `var(--brand-${acc.provider})44`,
                 '--brand-alpha': `var(--brand-${acc.provider})11`
               }">
            
            <div v-if="acc.active" class="active-aura"></div>

            <div class="nexus-avatar">{{ acc.name[0].toUpperCase() }}</div>
            
            <div class="nexus-meta">
              <h4>{{ acc.name }}</h4>
              <div class="nexus-status" :style="{ color: acc.active ? `var(--brand-${acc.provider})` : 'var(--p-text-dim)' }">
                {{ acc.active ? 'Identidade Sincronizada' : 'Standby' }}
              </div>
            </div>

            <div style="margin-top: 1.5rem;">
              <!-- Campo de Chave de API (Para identidades não-Google) -->
              <div v-if="selectedAccountProvider !== 'google'" class="premium-form-group">
                 <label style="font-size: 0.6rem; opacity: 0.6; margin-bottom: 8px;">CHAVE DE ACESSO</label>
                 <input v-model="acc.api_key" type="password" class="maestro-input" style="font-size: 0.7rem; padding: 12px !important;" placeholder="sk-..." @change="save()" />
              </div>

              <div v-if="selectedAccountProvider === 'google'">
                 <div style="background: rgba(0,0,0,0.2); padding: 10px; border-radius: 10px; border: 1px solid rgba(255,255,255,0.03);">
                    <code style="font-size: 0.65rem; color: var(--brand-google); opacity: 0.8;">{{ acc.home_dir.split('\\').pop() }}</code>
                 </div>
              </div>
            </div>

            <div class="nexus-footer">
              <button v-if="selectedAccountProvider === 'google'" @click="handleLoginAccount(selectedAccountProvider, acc.name)" class="btn-nexus-action primary">
                LOGIN
              </button>
              
              <button v-if="!acc.active" @click="handleSwitchAccount(selectedAccountProvider, acc.name)" class="btn-nexus-action">
                ATIVAR
              </button>
              <button v-else class="btn-nexus-action" style="background: #fff; color: #000; cursor: default;">
                ONLINE
              </button>
              
              <button @click="handleRemoveAccount(selectedAccountProvider, acc.name)" class="btn-nexus-action danger" title="Remover Identidade">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- ABA SEGURANÇA (FIREWALL PREMIER) -->
      <section v-if="store.activeTab === 'seguranca'" class="glass-panel animate-slide-up" style="border-color: rgba(239, 68, 68, 0.15);">
         <div class="header-with-badge" style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 3rem;">
           <div>
              <h2 class="section-title" style="color: #ef4444; letter-spacing: 6px;">🛡️ Firewall da Sinfonia</h2>
              <p style="color: var(--p-text-dim); margin-top: 10px; font-size: 1rem;">Controle granular de acesso ao sistema de arquivos e execução local.</p>
           </div>
           <div class="security-level-badge" style="background: rgba(239, 68, 68, 0.1); border: 1px solid rgba(239, 68, 68, 0.3); color: #ef4444; padding: 5px 15px; border-radius: 20px; font-size: 0.6rem; font-weight: 900; letter-spacing: 2px;">MODO RESTRITO ATIVO</div>
         </div>

         <div class="security-grid-comprehensive">
             <div v-for="(label, key) in {
                allow_read: 'Permitir Leitura',
                allow_write: 'Permitir Escrita',
                allow_create: 'Criar Arquivos',
                allow_delete: 'Excluir Arquivos',
                allow_move: 'Mover/Renomear',
                allow_run_commands: 'Comandos Shell',
                full_machine_access: 'Acesso Global'
             }" :key="key" class="sec-card" :class="{ 'critical-sec': key === 'full_machine_access' || key === 'allow_run_commands' }">
                <div class="sec-info">
                   <h5 style="margin: 0; font-weight: 800; font-size: 1.1rem; color: #fff;">{{ label }}</h5>
                   <p style="margin: 8px 0 0; font-size: 0.8rem;" :style="{ color: key === 'full_machine_access' ? '#ef4444' : 'var(--p-text-dim)' }">
                     {{ key === 'full_machine_access' ? '⚠️ ALERTA: AUTORIZAÇÃO TOTAL' : 'Permissão de ' + label.toLowerCase() }}
                   </p>
                </div>
                
                <div class="sec-toggle-wrapper" @click="store.config.security[key] = !store.config.security[key]">
                   <div class="maestro-switch" :class="{ 
                     'on': store.config.security[key], 
                     'critical-on': store.config.security[key] && (key === 'full_machine_access' || key === 'allow_run_commands' || key === 'allow_delete')
                   }">
                     <div class="maestro-switch-thumb" :class="{
                       'critical-thumb': store.config.security[key] && (key === 'full_machine_access' || key === 'allow_run_commands' || key === 'allow_delete')
                     }"></div>
                   </div>
                   <span v-if="store.config.security[key]" class="sec-label-active" :style="(key === 'full_machine_access' || key === 'allow_run_commands' || key === 'allow_delete') ? 'color: #ef4444;' : 'color: #22c55e;'">
                     {{ (key === 'full_machine_access' || key === 'allow_run_commands') ? '⚠️ PERIGO' : 'ATIVO ✓' }}
                   </span>
                   <span v-else class="sec-label-blocked">🔒 BLOQUEADO</span>
                 </div>
             </div>
         </div>
         <button @click="save" class="btn-glow-red" style="margin-top: 3rem; width: 100%;">
           SALVAR E REVALIDAR PROTOCOLOS DE SEGURANÇA 🔐
         </button>
      </section>

      <!-- ABA MCP -->
      <section v-if="store.activeTab === 'mcp'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Model Context Protocol (MCP)</h2>
        <div class="mcp-restored-form">
           <div class="premium-form-group">
              <label>Identificador do Servidor</label>
              <input v-model="store.mcpName" placeholder="Ex: postgres, shopify, memory" class="maestro-input" />
           </div>
           <div class="premium-form-group">
              <label>Comando de Execução (Shell)</label>
              <input v-model="store.mcpCommand" placeholder="Ex: npx -y @modelcontextprotocol/server-postgres" class="maestro-input" />
           </div>
           <div class="mcp-actions-row" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1rem; margin-top: 2rem;">
              <button @click="addMCPServer" class="btn-glow-blue" style="width: 100%;">INSTALAR SERVIDOR ⚡</button>
              <button @click="listMCPServers" class="btn-outline" style="width: 100%;">LISTAR REGISTRADOS 📋</button>
           </div>
           <div v-if="store.showMcpList" class="mcp-output-container">
              <div class="output-header">SERVIDORES CONFIGURADOS</div>
              <pre class="mcp-output-box">{{ store.mcpServers }}</pre>
           </div>
        </div>
      </section>
    </div>

    <footer class="maestro-terminal-v2" v-show="store.installStatus !== '' || store.installLogs.length > 0">
      <div class="t-bar">
         <span class="t-title">SYSTEM_ORCHESTRATOR_OUTPUT</span>
         <div class="t-pulse"><span></span> ACTIVE</div>
      </div>
      <div class="t-contents" :ref="(el) => store.logContainer = el">
        <div v-for="(log, i) in store.installLogs" :key="i" class="t-entry">> {{ log }}</div>
        <div v-if="store.installStatus" class="t-status">>> {{ store.installStatus }}</div>
      </div>
    </footer>

    <!-- MODAL DE MIGRAÇÃO DE PROVEDOR (NATIVO/LMSTUDIO/GEMINI) -->
    <div v-if="showProviderModal" class="premium-modal-overlay">
       <div class="premium-modal-content warning-modal" style="border-color: #3b82f6; box-shadow: 0 0 30px rgba(59,130,246,0.2);">
          <div class="modal-icon" style="color: #60a5fa;">🔄</div>
          <h2 class="modal-title" style="color: #60a5fa;">Migração de Motor Neural</h2>
          <div class="modal-body">
             <p>Você selecionou o motor: <strong style="color: #fff; background: rgba(59,130,246,0.3); padding: 2px 6px; border-radius: 4px;">{{ pendingProvider.toUpperCase() }}</strong></p>
             <div style="background: rgba(0,0,0,0.3); padding: 15px; border-radius: 10px; margin: 15px 0;">
               <ul style="color: #cbd5e1; text-align: left; padding-left: 20px; font-size: 0.85rem; line-height: 1.6; margin: 0;">
                 <li style="margin-bottom: 8px;"><strong style="color: #ef4444;">Dimensão Vetorial:</strong> Isso altera o formato geométrico das sinapses (3072 vs 1024). Um Reset Atômico do banco será <b>obrigatório</b> a seguir.</li>
                 <li><strong style="color: #fbbf24;">Multimídia Limitada:</strong> Apenas textos e códigos-fonte. O processamento semântico de fotos e vídeos não embarca no motor local.</li>
               </ul>
             </div>
          </div>
          <div class="modal-actions">
             <button @click="cancelProviderChange" class="btn-cancel">VOLTAR</button>
             <button @click="applyProviderChange" style="background: #3b82f6; color: white; padding: 12px 24px; border-radius: 12px; border: none; cursor: pointer; font-weight: bold; box-shadow: 0 4px 15px rgba(59,130,246,0.4);">
                ENTENDI, CONTINUAR
             </button>
          </div>
       </div>
    </div>

    <!-- MODAL DE CONFIRMAÇÃO DE RESET -->
    <div v-if="store.showResetModal" class="premium-modal-overlay">
       <div class="premium-modal-content warning-modal">
          <div class="modal-icon">☢️</div>
          <h2 class="modal-title">Reset Atômico do Banco</h2>
          <div class="modal-body">
             <p>Você está prestes a excluir **todas as coleções do Qdrant** ({{ store.config.qdrant_url }}) e limpar o cache local.</p>
             <p class="warning-text">Esta ação é irreversível. O Maestro esquecerá todas as conexões neurais feitas até agora.</p>
          </div>
          <div class="modal-actions">
             <button @click="store.showResetModal = false" :disabled="store.isResetting" class="btn-cancel">ABORTAR</button>
             <button @click="handleResetDB" :disabled="store.isResetting" class="btn-confirm-delete">
                {{ store.isResetting ? 'LIMPANDO...' : 'SIM, APAGAR TUDO' }}
             </button>
          </div>
       </div>
    </div>
  </main>
</template>

<style scoped>
@import '../assets/css/Settings.css';

@keyframes pulse {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 10px rgba(16, 185, 129, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(16, 185, 129, 0); }
}

/* NEXUS TOAST SYSTEM */
.nexus-toast {
  position: fixed;
  top: 30px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10000;
  padding: 14px 24px;
  border-radius: 18px;
  background: rgba(13, 17, 23, 0.85);
  backdrop-filter: blur(25px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 15px 50px rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 320px;
  max-width: 450px;
  cursor: pointer;
  pointer-events: auto;
}

.toast-success { 
  border-color: rgba(59, 130, 246, 0.4); 
  box-shadow: 0 0 30px rgba(59, 130, 246, 0.15); 
}
.toast-error { 
  border-color: rgba(239, 68, 68, 0.4); 
  box-shadow: 0 0 30px rgba(239, 68, 68, 0.15); 
}

.toast-icon {
  font-size: 1.5rem;
  background: rgba(255,255,255,0.03);
  width: 45px;
  height: 45px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 12px;
  border: 1px solid rgba(255,255,255,0.05);
}

.toast-content {
  display: flex;
  flex-direction: column;
}

.toast-title {
  font-size: 0.65rem;
  font-weight: 900;
  text-transform: uppercase;
  letter-spacing: 2px;
  color: var(--p-text-dim);
  margin-bottom: 2px;
}

.toast-msg {
  font-size: 0.85rem;
  font-weight: 700;
  color: #fff;
}

.animate-toast-in { animation: toastIn 0.6s cubic-bezier(0.19, 1, 0.22, 1); }
@keyframes toastIn {
  from { transform: translate(-50%, -60px); opacity: 0; }
  to { transform: translate(-50%, 0); opacity: 1; }
}

.toast-leave-active {
  transition: all 0.5s ease;
}
.toast-leave-to {
  transform: translate(-50%, -60px);
  opacity: 0;
}
</style>
