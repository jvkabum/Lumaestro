<script setup>
import { ref, onMounted } from 'vue'
import { GetConfig, SaveConfig, GetToolsStatus, InstallTool, SetupTool } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

const config = ref({
  obsidian_vault_path: '',
  qdrant_url: '',
  gemini_api_key: '',
  use_gemini_api_key: false,
  claude_api_key: '',
  use_claude_api_key: false,
  active_agent: 'gemini'
})

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

const installLogs = ref([])
const installStatus = ref('')
const logContainer = ref(null)

onMounted(async () => {
  const savedConfig = await GetConfig()
  console.log("Configurações recebidas do Maestro:", savedConfig)
  if (savedConfig) {
    config.value = savedConfig
  }
  
  refreshStatus()

  // Ouvir logs do instalador em tempo real
  EventsOn('installer:log', (log) => {
    installLogs.value.push(log)
    if (logContainer.value) {
      setTimeout(() => {
        logContainer.value.scrollTop = logContainer.value.scrollHeight
      }, 10)
    }
  })
})

const refreshStatus = async () => {
  status.value.tools = await GetToolsStatus()
}

const fixEnv = async () => {
  installLogs.value = []
  installStatus.value = "Iniciando correção de ambiente..."
  // @ts-ignore
  const res = await window.go.main.App.FixEnvironment()
  installStatus.value = res
  refreshStatus()
}

const save = async () => {
  const res = await SaveConfig(config.value)
  alert(res)
  refreshStatus()
}

const install = async (name) => {
  installLogs.value = []
  installStatus.value = `Iniciando instalação de ${name}...`
  const res = await InstallTool(name)
  installStatus.value = res
  refreshStatus()
}

const setup = async (name) => {
  installStatus.value = `Abrindo terminal de configuração para ${name}...`
  const res = await SetupTool(name)
  installStatus.value = res
}

const getAuthLabel = (agent) => {
  if (config.value[`use_${agent}_api_key`]) {
    return 'CHAVE API ⚡'
  }
  return agent === 'claude' ? 'FAZER LOGIN (OAUTH)' : 'CONFIGURAR LOGIN'
}

const getAuthStyle = (agent) => {
  if (config.value[`use_${agent}_api_key`]) {
    return 'border-color: rgba(245, 158, 11, 0.4); color: #fde68a; background: rgba(245, 158, 11, 0.08);'
  }
  return 'border-color: var(--primary);'
}
</script>

<template>
  <main class="settings-view animate-fade-up">
    <header class="settings-header">
      <div class="brand-badge">SISTEMA</div>
      <h1 class="gradient-text">Configurações Maestro</h1>
      <p class="subtitle">Gerencie o cérebro e as ferramentas da sua IA.</p>
    </header>

    <div class="content-grid">
      <!-- Seção Geral -->
      <section class="glass premium-shadow panel-main">
        <h2 class="section-title">Configurações Base</h2>
        
        <div class="form-group">
          <label>Caminho do Obsidian Vault</label>
          <div class="input-wrapper">
            <input v-model="config.obsidian_vault_path" type="text" class="premium-input" placeholder="C:\Users\...\Obsidian" />
            <div class="input-glow"></div>
          </div>
        </div>

        <div class="form-group">
          <label>URL do Qdrant Cloud</label>
          <div class="input-wrapper">
            <input v-model="config.qdrant_url" type="text" class="premium-input" placeholder="https://..." />
            <div class="input-glow"></div>
          </div>
        </div>

        <div class="form-group">
          <label>Google Gemini API Key (Embeddings & CLI)</label>
          <div class="input-wrapper">
            <input v-model="config.gemini_api_key" type="password" class="premium-input" placeholder="••••••••••••••••" />
            <div class="input-glow"></div>
          </div>
        </div>

        <div class="form-group toggle-group" style="margin-bottom: 2.5rem;">
          <label class="toggle-label">
            <input type="checkbox" v-model="config.use_gemini_api_key" class="premium-toggle" />
            <span class="toggle-slider"></span>
            <div class="toggle-text">
              <span class="title">Modo Autônomo API (Gemini CLI)</span>
              <span class="desc">Usar chave em vez da sessão OAuth. (Embeddings usam a chave obrigatoriamente)</span>
            </div>
          </label>
        </div>

        <div class="form-group">
          <label>Anthropic Claude API Key</label>
          <div class="input-wrapper">
            <input v-model="config.claude_api_key" type="password" class="premium-input" placeholder="••••••••••••••••" :disabled="!config.use_claude_api_key" />
            <div class="input-glow"></div>
          </div>
        </div>

        <div class="form-group toggle-group">
          <label class="toggle-label">
            <input type="checkbox" v-model="config.use_claude_api_key" class="premium-toggle" />
            <span class="toggle-slider"></span>
            <div class="toggle-text">
              <span class="title">Modo Autônomo API</span>
              <span class="desc">Usar chave em vez da assinatura Pro (claude auth login)</span>
            </div>
          </label>
        </div>


        <button @click="save" class="btn-premium save-btn">
          <span>SALVAR ALTERAÇÕES</span>
          <div class="btn-shimmer"></div>
        </button>
      </section>

      <!-- Hub de Ferramentas -->
      <section class="tools-container">
        <h2 class="section-title">Hub de Agentes</h2>
        <div class="tools-grid">
          <!-- Card Gemini -->
          <div class="tool-card glass glow-on-hover" :class="{ 'active': status.tools.gemini }">
            <div class="card-header">
              <div class="status-indicator" :class="{ 'online': status.tools.gemini }"></div>
              <h3>Gemini CLI</h3>
            </div>
            <p>IA generativa rápida e eficiente.</p>
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
              <button @click="install('gemini')" class="tool-btn">
                {{ status.tools.gemini ? 'ATUALIZAR' : 'INSTALAR' }}
              </button>
              <button v-if="status.tools.gemini" @click="setup('gemini')" class="tool-btn" :style="getAuthStyle('gemini')">
                {{ getAuthLabel('gemini') }}
              </button>
            </div>
          </div>

          <!-- Card Claude -->
          <div class="tool-card glass glow-on-hover" :class="{ 'active': status.tools.claude }">
            <div class="card-header">
              <div class="status-indicator" :class="{ 'online': status.tools.claude }"></div>
              <h3>Claude Code</h3>
            </div>
            <p>Codificação autônoma de elite.</p>
            <div style="display: flex; gap: 10px; flex-wrap: wrap;">
              <button @click="install('claude')" class="tool-btn">
                {{ status.tools.claude ? 'ATUALIZAR' : 'INSTALAR' }}
              </button>
              <button v-if="status.tools.claude" @click="setup('claude')" class="tool-btn" :style="getAuthStyle('claude')">
                {{ getAuthLabel('claude') }}
              </button>
              <button v-if="!status.tools.claude" @click="fixEnv" class="tool-btn" style="border-color: var(--primary-glow); color: var(--primary);">
                CORRIGIR PATH
              </button>
            </div>
          </div>

          <!-- Card Obsidian -->
          <div class="tool-card glass glow-on-hover" :class="{ 'active': status.tools.obsidian }">
            <div class="card-header">
              <div class="status-indicator" :class="{ 'online': status.tools.obsidian }"></div>
              <h3>Obsidian CLI</h3>
            </div>
            <p>Sincronização de base de conhecimento.</p>
            <button @click="install('obsidian')" class="tool-btn">
              {{ status.tools.obsidian ? 'ATUALIZAR' : 'INSTALAR' }}
            </button>
          </div>
        </div>
      </section>
    </div>

    <!-- Console Section -->
    <footer class="console-section" v-if="installStatus || installLogs.length > 0">
      <div class="console-header glass">
        <div class="header-left">
          <div class="console-icon"></div>
          <span>OPERATIONAL TERMINAL</span>
        </div>
        <div class="pulse-indicator">
          <span>ACTIVE SESSION</span>
          <div class="dot"></div>
        </div>
      </div>
      <div class="console-body" ref="logContainer">
        <div class="scanlines"></div>
        <div v-for="(log, index) in installLogs" :key="index" class="log-entry">
          <span class="timestamp">[{{ new Date().toLocaleTimeString() }}]</span>
          <span class="prompt">LOG_INFO:</span> 
          <span class="message">{{ log }}</span>
        </div>
        <div v-if="installStatus" class="status-entry animate-pulse">
          >> SYSTEM: {{ installStatus }}
        </div>
      </div>
    </footer>
  </main>
</template>

<style scoped>
.settings-view {
  width: 100%;
  max-width: 100%;
  height: 100vh;
  padding: 2rem 3rem;
  overflow-y: auto;
  overflow-x: hidden;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 2rem;
}

.settings-header {
  margin-bottom: 1rem;
}

.brand-badge {
  display: inline-block;
  background: var(--primary-glow);
  color: var(--primary);
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 2px;
  margin-bottom: 1rem;
}

.gradient-text {
  font-size: 2.8rem;
  background: linear-gradient(135deg, #f8fafc 0%, #94a3b8 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0;
}

.subtitle {
  color: var(--text-dim);
  font-size: 1.1rem;
  margin-top: 0.5rem;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 2rem;
  align-items: start;
  width: 100%;
  box-sizing: border-box;
}

.panel-main {
  padding: 2.5rem;
}

.section-title {
  font-size: 0.9rem;
  text-transform: uppercase;
  letter-spacing: 3px;
  color: var(--primary);
  margin-bottom: 2rem;
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-title::after {
  content: '';
  flex: 1;
  height: 1px;
  background: linear-gradient(90deg, var(--primary-glow), transparent);
}

.form-group {
  margin-bottom: 2rem;
}

label {
  display: block;
  font-size: 0.85rem;
  font-weight: 600;
  color: #f8fafc;
  margin-bottom: 0.75rem;
  letter-spacing: 0.5px;
}

.input-wrapper {
  position: relative;
}

.premium-input {
  width: 100%;
  padding: 14px 18px;
  font-size: 1rem;
  border-radius: 12px;
  font-family: inherit;
}

.input-glow {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  border-radius: 12px;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.3s;
  box-shadow: 0 0 15px var(--primary-glow);
}

.premium-input:focus + .input-glow {
  opacity: 1;
}

.premium-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  background: rgba(0, 0, 0, 0.2);
}

/* Custom Premium Toggle */
.toggle-group {
  margin-top: -0.5rem;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 15px;
  cursor: pointer;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 12px;
  transition: all 0.3s;
}

.toggle-label:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(59, 130, 246, 0.3);
}

.premium-toggle {
  appearance: none;
  width: 44px;
  height: 24px;
  background: rgba(255, 255, 255, 0.1);
  border-radius: 20px;
  position: relative;
  outline: none;
  cursor: pointer;
  transition: 0.3s;
  flex-shrink: 0;
}

.premium-toggle::before {
  content: '';
  position: absolute;
  top: 3px;
  left: 3px;
  width: 18px;
  height: 18px;
  background: #f8fafc;
  border-radius: 50%;
  transition: 0.3s cubic-bezier(0.4, 0.0, 0.2, 1);
  box-shadow: 0 2px 4px rgba(0,0,0,0.3);
}

.premium-toggle:checked {
  background: var(--primary);
  box-shadow: 0 0 12px rgba(59, 130, 246, 0.4);
}

.premium-toggle:checked::before {
  left: 23px;
}

.toggle-text {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toggle-text .title {
  color: #f8fafc;
  font-weight: 700;
  font-size: 0.85rem;
  line-height: 1.2;
}

.toggle-text .desc {
  color: #94a3b8;
  font-size: 0.70rem;
  line-height: 1.3;
}

.save-btn {
  width: 100%;
  position: relative;
  overflow: hidden;
  margin-top: 1rem;
}

/* Tools Grid */
.tools-grid {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.tool-card {
  padding: 1.5rem;
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.tool-card:hover {
  transform: scale(1.02) translateX(5px);
  background: rgba(255, 255, 255, 0.04);
}

.tool-card.active {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.05);
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 0.5rem;
}

.card-header h3 {
  font-size: 1.1rem;
  margin: 0;
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #334155;
}

.status-indicator.online {
  background: var(--success);
  box-shadow: 0 0 12px var(--success);
}

.tool-card p {
  color: var(--text-dim);
  font-size: 0.85rem;
  margin: 0.5rem 0 1.25rem 0;
}

.tool-btn {
  background: transparent;
  border: 1px solid var(--border-color);
  color: #f8fafc;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.3s;
  white-space: nowrap;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.tool-btn:hover {
  border-color: var(--primary);
  background: var(--primary-glow);
}

/* Console Styling */
.console-section {
  margin-top: 2rem;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.console-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 20px;
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 1px;
  border-radius: 12px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--primary);
}

.console-icon {
  width: 8px;
  height: 8px;
  background: var(--primary);
  clip-path: polygon(0% 0%, 100% 50%, 0% 100%);
}

.pulse-indicator {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--success);
}

.pulse-indicator .dot {
  width: 6px;
  height: 6px;
  background: var(--success);
  border-radius: 50%;
  animation: pulse 1s infinite;
}

.console-body {
  background: #020617;
  border: 1px solid var(--border-color);
  border-radius: 16px;
  height: 250px;
  overflow-y: auto;
  padding: 1.5rem;
  position: relative;
  font-family: 'Fira Code', monospace;
  font-size: 0.85rem;
  line-height: 1.6;
}

.scanlines {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: linear-gradient(rgba(18, 16, 16, 0) 50%, rgba(0, 0, 0, 0.25) 50%), linear-gradient(90deg, rgba(255, 0, 0, 0.06), rgba(0, 255, 0, 0.02), rgba(0, 0, 255, 0.06));
  background-size: 100% 2px, 3px 100%;
  pointer-events: none;
  z-index: 10;
}

.log-entry {
  display: flex;
  gap: 12px;
  margin-bottom: 0.5rem;
}

.timestamp { color: #475569; }
.prompt { color: var(--primary); font-weight: bold; }
.message { color: #e2e8f0; }

.status-entry {
  color: var(--primary);
  font-weight: bold;
  margin-top: 1rem;
  border-top: 1px solid var(--border-color);
  padding-top: 0.5rem;
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.5); opacity: 0.6; }
  100% { transform: scale(1); opacity: 1; }
}

@keyframes pulse-anim {
  0% { opacity: 0.5; }
  50% { opacity: 1; }
  100% { opacity: 0.5; }
}

.animate-pulse {
  animation: pulse-anim 2s infinite ease-in-out;
}
</style>
