<script setup>
import { onMounted, ref, watch } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useSettingsConfig } from '../composables/useSettingsConfig'
import { useSettingsTools } from '../composables/useSettingsTools'
import { useSettingsMCP } from '../composables/useSettingsMCP'
import { useSettingsAccounts } from '../composables/useSettingsAccounts'
import { useSettingsProjects } from '../composables/useSettingsProjects'

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
const { handleAddAccount, handleLoginAccount, handleSwitchAccount } = useSettingsAccounts()
const { handleAddProject, handleSelectDirectory } = useSettingsProjects()

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
      alert(`O modelo "${model}" nao respondeu no endpoint de embeddings do LM Studio. Use um modelo de embedding (ex: nomic-embed-text).`)
    }
  } catch (e) {
    alert('Falha ao detectar dimensao do embedding: ' + e)
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
    alert('Erro ao conectar ao LM Studio: ' + e)
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
  <main class="settings-view animate-fade-up">
    <div class="settings-header">
      <div class="brand-badge pulse-aura">LUMAESTRO PREMIER</div>
      <h1 class="gradient-text">Orquestração de IAs</h1>
      <p class="subtitle">Configurações globais e gerenciamento de identidades.</p>
    </div>

    <div class="tabs-nav-glass">
      <button v-for="tab in ['geral', 'qdrant', 'chaves', 'motores', 'modelos', 'contas', 'seguranca', 'mcp', 'repositórios']" 
              :key="tab"
              @click="store.activeTab = tab" 
              :class="{ 'active': store.activeTab === tab }" 
              class="tab-btn-premium">
        {{ tab === 'contas' ? 'CONTAS GEMINI 💎' : tab.toUpperCase() }}
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
        <h2 class="section-title">Chaves de API (Conexão Legada)</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 1.5rem; font-size: 0.9rem;">
          Gerencie injeções diretas de tokens de acesso para execução em modo bypass em vez do sistema nativo OAuth.
        </p>
        
        <div class="premium-form-group">
          <label style="display: flex; align-items: center; justify-content: space-between;">
            <span>Gemini API Keys (Pool de Failover)</span>
            <span v-if="store.geminiKeyCount > 0" style="background: rgba(59, 130, 246, 0.15); color: #3b82f6; padding: 3px 10px; border-radius: 8px; font-size: 0.65rem; font-weight: 900; letter-spacing: 1px;">
              {{ store.geminiKeyCount }} CHAVE{{ store.geminiKeyCount > 1 ? 'S' : '' }} 🔑
            </span>
          </label>
          <textarea 
            v-model="store.config.gemini_api_key" 
            class="maestro-input" 
            placeholder="AIzaSy...chave1, AIzaSy...chave2, AIzaSy...chave3"
            rows="3"
            style="resize: vertical; font-family: monospace; font-size: 0.85rem; line-height: 1.6;"
          ></textarea>

        </div>



        <div class="sec-card" style="margin-top: 1.5rem; margin-bottom: 1.5rem; padding: 1.2rem 1.6rem;">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 0.95rem; color: #fff;">Modo Autônomo API</h5>
              <p style="margin: 8px 0 0; font-size: 0.78rem; color: var(--p-text-dim);">Usar chave legada em vez de Sessões OAuth.</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="store.config.use_gemini_api_key" />
              <span class="m-slider-sec"></span>
           </label>
        </div>

        <div class="premium-form-group">
          <label>Claude API Key</label>
          <input v-model="store.config.claude_api_key" type="password" class="maestro-input" placeholder="••••••••" :disabled="!store.config.use_claude_api_key" />
        </div>

        <div class="sec-card" style="margin-bottom: 2.5rem; padding: 1.5rem 2.5rem;">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #fff;">Claude API Mode</h5>
              <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Habilitar injeção direta de chave Anthropic.</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="store.config.use_claude_api_key" />
              <span class="m-slider-sec"></span>
           </label>
        </div>

        <button @click="save" class="btn-glow-blue" style="margin-top: 1.5rem; width: 100%;">SALVAR CHAVES</button>
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
                      <div class="avatar-glow maestro-engine-icon" :style="tool === 'gemini' ? 'background: linear-gradient(135deg, #3b82f6, #8b5cf6)' : tool === 'claude' ? 'background: linear-gradient(135deg, #f97316, #ea580c)' : 'background: linear-gradient(135deg, #10b981, #059669)'">
                         {{ tool === 'gemini' ? '⚡' : tool === 'claude' ? '🦾' : '🤖' }}
                      </div>
                      <div>
                        <h4 style="margin: 0; font-weight: 900; color: #fff; font-size: 1.3rem; letter-spacing: 2px;">{{ tool === 'lmstudio' ? 'LM STUDIO' : tool.toUpperCase() }}</h4>
                        <div v-if="tool !== 'lmstudio'" class="engine-status-badge" :style="store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? '' : 'border-color: rgba(245, 158, 11, 0.3); background: rgba(245, 158, 11, 0.05); color: #f59e0b;') : 'border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.05);'">
                          <span class="status-dot" :style="store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? '' : 'background: #f59e0b; box-shadow: none;') : 'background: #ef4444; box-shadow: none;'"></span> 
                          {{ store.status.tools[tool] ? (store.config[`use_${tool}_api_key`] || store.status.tools[tool + '_auth'] ? 'SISTEMA PRONTO' : 'NÃO AUTENTICADO') : 'NÃO INSTALADO' }}
                        </div>
                        <div v-else class="engine-status-badge" :style="(store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05);' : 'border-color: rgba(239, 68, 68, 0.3); background: rgba(239, 68, 68, 0.05);'">
                          <span class="status-dot" :style="(store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'background: #10b981;' : 'background: #ef4444; box-shadow: none;'"></span> 
                          {{ (store.config.lmstudio_enabled || isAutoStart('lmstudio')) && store.config.lmstudio_url ? 'CONFIGURADO ✓' : 'DESABILITADO' }}
                        </div>
                      </div>
                   </div>
                   
                   <!-- Auto-Start Switch -->
                   <div class="auto-boot-container" @click="toggleAutoStart(tool)" title="Inicia o motor automaticamente assim que você abre o Lumaestro" style="flex-shrink: 0;">
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
                   <button @click="install(tool)" class="unit-btn-solid" style="flex: 1.5;">
                     {{ tool === 'lmstudio' ? 'SALVAR CONFIG' : 'SINCRONIZAR' }}
                   </button>
                   <button v-if="tool !== 'lmstudio' && store.status.tools[tool]" @click="setup(tool)" class="unit-btn-glow" :style="getAuthStyle(tool)" style="flex: 1;">
                      {{ getAuthLabel(tool) }}
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

      <!-- ABA MODELOS (POOL ATIVO) -->
      <section v-if="store.activeTab === 'modelos'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Pool Ativo de Modelos</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Configure quais provedores e modelos o sistema usa para chat, embeddings e RAG semântico.
        </p>

        <!-- BLEND DE PROVEDORES PARA CHAT -->
        <h3 style="font-size: 0.85rem; font-weight: 800; letter-spacing: 1px; color: #94a3b8; margin-bottom: 1rem; text-transform: uppercase;">Chat / Orquestrador</h3>

        <div class="sec-card" style="margin-bottom: 1.5rem; padding: 1.2rem 1.6rem;">
          <div class="sec-info">
            <h5 style="margin: 0; font-weight: 800; font-size: 0.95rem; color: #fff;">Blend de provedores</h5>
            <p style="margin: 6px 0 0; font-size: 0.78rem; color: var(--p-text-dim);">Quando ativo, o orquestrador respeita o pool e roteia pelo provedor mais adequado disponível.</p>
          </div>
          <label class="hybrid-toggle-maestro">
            <input type="checkbox" v-model="store.config.blend_active_models" />
            <span class="m-slider-sec"></span>
          </label>
        </div>

        <div class="premium-form-group" style="margin-bottom: 1.5rem;">
          <label style="margin-bottom: 10px; display: block;">Provedores ativos para chat</label>
          <div style="display: flex; flex-wrap: wrap; gap: 0.75rem;">
            <button
              v-for="p in ['gemini', 'claude', 'lmstudio']"
              :key="p"
              type="button"
              @click="toggleProvider(p)"
              :style="isProviderActive(p)
                ? 'padding: 10px 14px; border-radius: 10px; border: 1px solid rgba(16,185,129,0.4); background: rgba(16,185,129,0.12); color: #d1fae5; cursor: pointer; font-weight: 700;'
                : 'padding: 10px 14px; border-radius: 10px; border: 1px solid rgba(148,163,184,0.25); background: rgba(15,23,42,0.35); color: #cbd5e1; cursor: pointer; font-weight: 700;'"
            >
              {{ providerLabel(p) }} {{ isProviderActive(p) ? '✓' : '' }}
            </button>
          </div>
          <small style="display: block; margin-top: 0.6rem; color: var(--p-text-dim); font-size: 0.72rem;">
            Pelo menos um provedor precisa ficar ativo.
          </small>
        </div>

        <div class="premium-form-group" style="margin-bottom: 2rem;">
          <label>Provedor primário (chat padrão)</label>
          <select v-model="store.config.primary_provider" class="maestro-input" @change="setPrimaryProvider(store.config.primary_provider)">
            <option value="gemini">Gemini</option>
            <option value="claude">Claude</option>
            <option value="lmstudio">LM Studio</option>
          </select>
        </div>

        <div style="height: 1px; background: rgba(148,163,184,0.12); margin: 2rem 0;"></div>

        <!-- MOTOR DE EMBEDDINGS -->
        <h3 style="font-size: 0.85rem; font-weight: 800; letter-spacing: 1px; color: #94a3b8; margin-bottom: 1rem; text-transform: uppercase;">🔬 Motor de Embeddings (Qdrant / Sync 3D)</h3>
        <p style="color: var(--p-text-dim); margin-bottom: 1.5rem; font-size: 0.82rem; line-height: 1.5;">
          Define qual motor gera os vetores para busca semântica. Necessário para o Sync Obsidian 3D funcionar.<br/>
          <strong style="color: #fbbf24;">⚠️ Alterar o provedor muda a dimensão dos vetores — será necessário resetar o banco Qdrant.</strong>
        </p>

        <div class="premium-form-group" style="margin-bottom: 1.2rem;">
          <label>Provedor de embeddings</label>
          <select id="embedding-provider-select" :value="store.config.embeddings_provider" class="maestro-input" @change="confirmProviderChange">
            <option value="gemini">Nuvem: Gemini (gemini-embedding-2-preview · 3072 dim)</option>
            <option value="native">Local: Lumaestro Nativo (Qwen3 0.6B Interno · 1024 dim)</option>
            <option value="lmstudio">Servidor Externo: LM Studio</option>
          </select>
        </div>

        <div v-if="store.config.embeddings_provider === 'lmstudio'" class="premium-form-group" style="margin-bottom: 1.2rem;">
          <label>Modelo de embeddings no LM Studio</label>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <input
              v-model="store.config.embeddings_model"
              placeholder="Ex: nomic-embed-text, text-embedding-nomic-embed-text-v1.5"
              class="maestro-input"
              @change="detectEmbeddingDimension"
              style="flex: 1;"
            />
            <select v-if="store.lmModels.length > 0" v-model="store.config.embeddings_model" class="maestro-input" style="max-width: 220px;" @change="detectEmbeddingDimension">
              <option value="">-- selecionar do LM Studio --</option>
              <option v-for="m in store.lmModels" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>
          <small style="color: var(--p-text-dim); font-size: 0.72rem; display: block; margin-top: 0.4rem;">
            Carregue um modelo de embeddings no LM Studio (ex: nomic-embed-text). Clique em "🔄 MODELOS" na aba Motores para atualizar a lista.
          </small>
        </div>

        <div class="premium-form-group" style="margin-bottom: 2rem;">
          <label>Dimensão dos vetores</label>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <input
              v-model.number="store.config.embedding_dimension"
              type="number"
              class="maestro-input"
              style="max-width: 140px;"
              placeholder="3072"
            />
            <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
              <button type="button" @click="store.config.embedding_dimension = 3072" style="padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(59,130,246,0.4); background: rgba(59,130,246,0.08); color: #93c5fd; cursor: pointer; font-size: 0.75rem; font-weight: 700;">3072 Gemini</button>
              <button type="button" @click="store.config.embedding_dimension = 768" style="padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(168,85,247,0.4); background: rgba(168,85,247,0.08); color: #d8b4fe; cursor: pointer; font-size: 0.75rem; font-weight: 700;">768 nomic</button>
              <button type="button" @click="store.config.embedding_dimension = 1536" style="padding: 8px 12px; border-radius: 8px; border: 1px solid rgba(234,179,8,0.4); background: rgba(234,179,8,0.08); color: #fde68a; cursor: pointer; font-size: 0.75rem; font-weight: 700;">1536 ada-002</button>
            </div>
          </div>
          <small style="color: var(--p-text-dim); font-size: 0.72rem; display: block; margin-top: 0.4rem;">
            Precisa coincidir exatamente com o modelo escolhido.
          </small>
        </div>

        <div style="height: 1px; background: rgba(148,163,184,0.12); margin: 2rem 0;"></div>

        <!-- MOTOR DE RAG / ONTOLOGIA -->
        <h3 style="font-size: 0.85rem; font-weight: 800; letter-spacing: 1px; color: #94a3b8; margin-bottom: 1rem; text-transform: uppercase;">🧠 Motor de RAG / Ontologia (extração de triplas)</h3>
        <p style="color: var(--p-text-dim); margin-bottom: 1.5rem; font-size: 0.82rem; line-height: 1.5;">
          Define qual motor processa prompts de análise semântica (extração de fatos, grafo de conhecimento, conflitos).<br/>
          Pode ser diferente do motor de embeddings.
        </p>

        <div class="premium-form-group" style="margin-bottom: 1.2rem;">
          <label>Provedor de RAG/Ontologia</label>
          <select v-model="store.config.rag_provider" class="maestro-input" @change="onRAGProviderChange">
            <option value="gemini">Gemini (cascata resiliente de modelos)</option>
            <option value="lmstudio">LM Studio (modelo local)</option>
            <option value="claude">Claude (melhor para análise de código)</option>
          </select>
        </div>

        <div v-if="store.config.rag_provider === 'lmstudio'" class="premium-form-group" style="margin-bottom: 2rem;">
          <label>Modelo de chat para RAG no LM Studio</label>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <input
              v-model="store.config.rag_model"
              placeholder="Ex: google/gemma-4-26b-a4b, llama-3.2-3b-instruct"
              class="maestro-input"
              style="flex: 1;"
            />
            <select v-if="store.lmModels.length > 0" v-model="store.config.rag_model" class="maestro-input" style="max-width: 220px;">
              <option value="">-- selecionar do LM Studio --</option>
              <option v-for="m in store.lmModels" :key="m" :value="m">{{ m }}</option>
            </select>
          </div>
          <small style="color: var(--p-text-dim); font-size: 0.72rem; display: block; margin-top: 0.4rem;">
            Deixe vazio para usar o modelo padrão configurado na aba Motores.
          </small>
        </div>

        <div v-if="store.config.rag_provider === 'claude'" class="premium-form-group" style="margin-bottom: 2rem;">
          <label>Modelo Claude para RAG</label>
          <select v-model="store.config.rag_model" class="maestro-input">
            <option value="">claude-3-5-sonnet (padrão)</option>
            <option value="claude-3-5-sonnet-latest">claude-3-5-sonnet-latest</option>
            <option value="claude-3-5-haiku-latest">claude-3-5-haiku-latest (mais rápido)</option>
            <option value="claude-opus-4-5">claude-opus-4-5</option>
          </select>
        </div>

        <button @click="save" class="btn-glow-blue" style="width: 100%;">SALVAR CONFIGURAÇÃO DE MODELOS</button>
      </section>

      <!-- ABA CONTAS GEMINI (OAUTH) -->
      <section v-if="store.activeTab === 'contas'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Identidades Gemini OAuth</h2>
        <p class="subtitle-maestro" style="color: var(--p-text-dim); margin-bottom: 1.5rem; font-size: 0.95rem;">
          Gerencie múltiplas sessões isoladas do Google para alternar quotas de API e perfis em tempo real.
        </p>

        <div class="premium-form-group" style="display: flex; gap: 1rem; align-items: flex-end; margin-bottom: 2.5rem;">
          <div style="flex: 1;">
            <label>Nome da Nova Identidade</label>
            <input v-model="store.newAccName" placeholder="Ex: Trabalho, Pessoal, Pesquisa..." class="maestro-input" @keyup.enter="handleAddAccount" />
          </div>
          <button @click="handleAddAccount" class="btn-glow-blue" style="height: 52px; padding: 0 24px; font-size: 0.8rem;">
            CRIAR IDENTIDADE 💎
          </button>
        </div>

        <div class="accounts-grid-premium">
          <div v-for="acc in store.config.gemini_accounts" :key="acc.name" class="profile-card" :class="{ 'active-profile': acc.active }" style="display: flex; flex-direction: column;">
            <div class="profile-header" style="display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2.5rem;">
              <div class="avatar-glow" style="flex-shrink: 0;">{{ acc.name[0].toUpperCase() }}</div>
              <div class="profile-meta" style="min-width: 0; flex: 1;">
                <h4 style="margin: 0; font-weight: 900; color: #fff; font-size: 1.1rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;" :title="acc.name">{{ acc.name }}</h4>
                <div class="status-chip" :style="{ color: acc.active ? 'var(--p-accent)' : '#64748b', background: acc.active ? 'rgba(59, 130, 246, 0.1)' : 'transparent', border: acc.active ? '1px solid rgba(59, 130, 246, 0.2)' : 'none', padding: acc.active ? '4px 8px' : '0', borderRadius: '12px', display: 'inline-block', marginTop: '6px', fontSize: '0.65rem', fontWeight: '900', letterSpacing: '1px' }">
                  {{ acc.active ? 'SESSÃO ATIVA' : 'MODO STANDBY' }}
                </div>
              </div>
            </div>
            
            <div class="profile-actions" style="display: flex; gap: 12px; margin-top: auto;">
              <button @click="handleLoginAccount(acc.name)" class="btn-util" style="border-color: rgba(59, 130, 246, 0.4); color: #3b82f6; background: rgba(59, 130, 246, 0.05);">
                LOGAR 🔑
              </button>
              <button v-if="!acc.active" @click="handleSwitchAccount(acc.name)" class="btn-util" style="background: rgba(255,255,255,0.05);">
                ATIVAR ⚡
              </button>
              <!-- Botão de Excluir Premium -->
              <button class="btn-util btn-danger" style="flex: 0 0 50px; padding: 0;" title="Remover Identidade">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg>
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
              <button @click="liststore.mcpServers" class="btn-outline" style="width: 100%;">LISTAR REGISTRADOS 📋</button>
           </div>
           <div v-if="store.showMcpList" class="mcp-output-container">
              <div class="output-header">SERVIDORES CONFIGURADOS</div>
              <pre class="mcp-output-box">{{ store.mcpServers }}</pre>
           </div>
        </div>
      </section>

      <!-- ABA REPOSITÓRIOS (Code RAG & Aglomerados Radiais) -->
      <section v-if="store.activeTab === 'repositórios'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Aglomerados Estelares (Repositórios Radiais)</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Injete pastas de projetos locais no Grafo do Lumaestro. Estes projetos formarão órbitas concêntricas independentes (RAG Radial) protegidas de poluição vetorial, orbitando seu respectivo <b>Nó Núcleo</b>.
        </p>

        <div class="mcp-restored-form" style="border: 1px solid rgba(139, 92, 246, 0.3); background: rgba(139, 92, 246, 0.03); padding: 2.5rem; border-radius: 20px;">
           <div class="form-grid">
             <div class="premium-form-group">
                <label>Caminho Absoluto do Repositório</label>
                <div style="display: flex; gap: 10px;">
                  <input v-model="store.repoPathInput" placeholder="Ex: C:\git\Lumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4); flex: 1;" />
                  <button @click="handleSelectDirectory" class="btn-glow-blue" style="flex: 0 0 auto; padding: 0 24px; font-size: 1.2rem; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5); border-radius: 14px;" title="Navegar e Escolher Pasta">
                    📁
                  </button>
                </div>
             </div>
             <div class="premium-form-group">
                <label>Nome do Núcleo Satélite (Core Node)</label>
                <input v-model="store.coreNodeInput" placeholder="Ex: ProjetoLumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4);" />
             </div>
           </div>

           <!-- Code RAG Toggle Switch Premium -->
           <div class="sec-card" style="margin-top: 1rem; margin-bottom: 2.5rem; border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05); padding: 1.5rem 2.5rem; display: flex; align-items: center; justify-content: space-between;">
              <div class="sec-info" style="flex: 1;">
                 <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #10b981;">Devorar Código Fonte (Code RAG)</h5>
                 <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Ativando isto, além de .MD e Imagens, a IA irá ler, processar e gerar semânticas de todos os códigos .js, .go, .py e .ts.</p>
              </div>
              <label class="hybrid-toggle-maestro">
                 <input type="checkbox" v-model="store.includeCodeToggle" />
                 <span class="m-slider-sec" style="background: rgba(16, 185, 129, 0.1);"></span>
              </label>
           </div>

           <button @click="handleAddProject" :disabled="store.repoStatusMsg !== ''" class="btn-glow-blue" style="width: 100%; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5);">
              <span v-if="store.repoStatusMsg === ''">VINCULAR REPOSITÓRIO À SINFORNIA 🪐</span>
              <span v-else>{{ store.repoStatusMsg }}</span>
           </button>
        </div>

        <div style="margin-top: 4rem;">
          <h3 style="font-size: 1rem; color: #fff; letter-spacing: 2px; margin-bottom: 1.5rem;">SISTEMAS SOLARES (Orquestrados)</h3>
          
          <div v-if="!store.config.external_projects || store.config.external_projects.length === 0" style="color: var(--p-text-dim); text-align: center; padding: 2rem; border-radius: 12px; border: 1px dashed rgba(255,255,255,0.1);">
             O Universo ainda não possui outros projetos em órbita.
          </div>
          
          <div v-else class="satellites-grid">
             <div v-for="proj in store.config.external_projects" :key="proj.path" class="satellite-card">
               <div class="sat-core">
                 <div class="sat-ring-icon">🪐</div>
                 <h4 class="sat-node-name">{{ proj.core_node }}</h4>
                 <div class="sat-badge" :class="proj.include_code ? 'neo-active' : 'neo-docs'">
                   {{ proj.include_code ? '⚡ CODE RAG' : '📄 APENAS DOCS' }}
                 </div>
               </div>
               
               <div class="sat-path-box">
                 <span style="opacity: 0.6; margin-right: 8px;">📂</span>
                 <span>{{ proj.path }}</span>
               </div>
             </div>
          </div>
        </div>
      </section>
    </div>

    <footer class="maestro-terminal-v2" v-show="store.installStatus !== '' || store.installLogs.length > 0">
      <div class="t-bar">
         <span class="t-title">SYSTEM_ORCHESTRATOR_OUTPUT</span>
         <div class="t-pulse"><span></span> ACTIVE</div>
      </div>
      <div class="t-contents" ref="store.logContainer">
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
</style>
