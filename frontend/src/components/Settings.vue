<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { GetConfig, SaveConfig, GetToolsStatus, InstallTool, SetupTool, AddGeminiAccount, SwitchGeminiAccount, LoginGeminiAccount, AddMCPServer, ListMCPServers } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime'

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

// Helpers para auto-start toggles
const isAutoStart = (agent) => {
  return (config.value.auto_start_agents || []).includes(agent)
}

const toggleAutoStart = async (agent) => {
  if (!config.value.auto_start_agents) {
    config.value.auto_start_agents = []
  }
  const idx = config.value.auto_start_agents.indexOf(agent)
  if (idx >= 0) {
    config.value.auto_start_agents.splice(idx, 1)
  } else {
    config.value.auto_start_agents.push(agent)
  }
  await SaveConfig(config.value)
}

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

const scrollToConsole = async () => {
  await nextTick()
  setTimeout(() => {
    const view = document.querySelector('.settings-view')
    if (view) {
      view.scrollTo({
        top: view.scrollHeight,
        behavior: 'smooth'
      })
    }
  }, 100)
}

onMounted(async () => {
  try {
    const savedConfig = await GetConfig()
    // alert("DEBUG WAILS: " + JSON.stringify(savedConfig))
    if (savedConfig && Object.keys(savedConfig).length > 0) {
      // Fazemos o merge dos valores recebidos com os valores defaults locais (segurança!)
      config.value = Object.assign({}, config.value, savedConfig)
    } else {
      console.warn("Nenhuma config carregada do backend. Usando defaults.")
    }
  } catch(e) {
    alert("ERRO RARO DE COMUNICAÇÃO: " + e)
  }
  
  refreshStatus()

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

const save = async () => {
  try {
    const res = await SaveConfig(config.value)
    alert(res)
    refreshStatus()
  } catch (err) {
    alert("Erro na comunicação Wails ao salvar: " + err)
  }
}

const install = async (name) => {
  try {
    installLogs.value = []
    installStatus.value = `Iniciando operação para ${name}...`
    scrollToConsole()
    const res = await InstallTool(name)
    installStatus.value = res ? res : "Operação finalizada."
  } catch (err) {
    installStatus.value = `ERRO Crítico: ${err}`
  }
  refreshStatus()
}

const setup = async (name) => {
  installStatus.value = `Abrindo terminal de configuração para ${name}...`
  scrollToConsole()
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
  return 'border-color: #3b82f6;'
}

const mcpName = ref('')
const mcpCommand = ref('')
const mcpServers = ref('')
const showMcpList = ref(false)

// Estados para Diagnóstico Vetorial
const isDiagnosing = ref(false)
const diagnosticResult = ref(null)

const runDiagnostic = async () => {
  isDiagnosing.value = true
  diagnosticResult.value = null
  try {
    const res = await window.go.main.App.RunVectorDiagnostic()
    diagnosticResult.value = res
  } catch (e) {
    diagnosticResult.value = { success: false, error: String(e) }
  } finally {
    isDiagnosing.value = false
  }
}

const addMCPServer = async () => {
  if (!mcpName.value || !mcpCommand.value) {
    alert("Preencha o Nome e o Comando para o MCP")
    return
  }
  installLogs.value = []
  installStatus.value = `Instalando servidor MCP: ${mcpName.value}...`
  scrollToConsole()
  const res = await AddMCPServer(mcpName.value, mcpCommand.value)
  installStatus.value = "Instalação do MCP Finalizada."
  mcpName.value = ''
  mcpCommand.value = ''
  alert("Retorno do Terminal:\n" + res)
}

const listMCPServers = async () => {
  const res = await ListMCPServers()
  mcpServers.value = res
  showMcpList.value = true
}

// Funções de Multi-Conta
const handleAddAccount = async () => {
  if (!newAccName.value) return
  await AddGeminiAccount(newAccName.value)
  newAccName.value = ''
  const cfg = await GetConfig()
  if (cfg) config.value = cfg
}

const handleLoginAccount = async (name) => {
  await LoginGeminiAccount(name)
}

const handleSwitchAccount = async (name) => {
  await SwitchGeminiAccount(name)
  const cfg = await GetConfig()
  if (cfg) config.value = cfg
}

const activeTab = ref('geral')
const newAccName = ref('')
</script>

<template>
  <main class="settings-view animate-fade-up">
    <div class="settings-header">
      <div class="brand-badge pulse-aura">LUMAESTRO PREMIER</div>
      <h1 class="gradient-text">Orquestração de IAs</h1>
      <p class="subtitle">Configurações globais e gerenciamento de identidades.</p>
    </div>

    <div class="tabs-nav-glass">
      <button v-for="tab in ['geral', 'chaves', 'motores', 'contas', 'seguranca', 'mcp']" 
              :key="tab"
              @click="activeTab = tab" 
              :class="{ 'active': activeTab === tab }" 
              class="tab-btn-premium">
        {{ tab === 'contas' ? 'CONTAS GEMINI 💎' : tab.toUpperCase() }}
      </button>
    </div>

    <div class="content-viewport">
      <!-- ABA GERAL -->
      <section v-if="activeTab === 'geral'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Base da Sinfonia</h2>
        
        <div class="form-grid">
          <div class="premium-form-group">
            <label>Idioma Nativo do Agente</label>
            <select v-model="config.agent_language" class="maestro-input">
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
            <input v-model="config.obsidian_vault_path" type="text" class="maestro-input" placeholder="C:\Users\...\Obsidian" />
          </div>
        </div>

        <div class="premium-form-group">
          <label>Alcance da Teia (Vizinhos): <span class="highlight-val">{{ config.graph_neighbor_limit }}</span></label>
          <input v-model.number="config.graph_neighbor_limit" type="range" min="1" max="25" step="1" class="maestro-range" />
        </div>

        <div class="premium-form-group">
          <label>URL do Qdrant Cloud</label>
          <input v-model="config.qdrant_url" type="text" class="maestro-input" placeholder="https://..." />
        </div>

        <div class="premium-form-group">
          <label>Qdrant API Key (Coolify)</label>
          <input v-model="config.qdrant_api_key" type="password" class="maestro-input" placeholder="••••••••" />
        </div>

        <button @click="save" class="btn-glow-blue">SALVAR ALTERAÇÕES GERAIS</button>
      </section>

      <!-- ABA CHAVES (INJEÇÃO DE CHAVES DIRETAS) -->
      <section v-if="activeTab === 'chaves'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Chaves de API (Conexão Legada)</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Gerencie injeções diretas de tokens de acesso para execução em modo bypass em vez do sistema nativo OAuth.
        </p>
        
        <div class="premium-form-group">
          <label>Gemini API Key</label>
          <input v-model="config.gemini_api_key" type="password" class="maestro-input" placeholder="••••••••" />
        </div>

        <div class="premium-form-group">
          <label>Qdrant API Key (Climb/Coolify)</label>
          <input v-model="config.qdrant_api_key" type="password" class="maestro-input" placeholder="••••••••" />
        </div>

        <!-- PAINEL DE DIAGNÓSTICO VETORIAL -->
        <div class="diagnostic-panel-premium glass-panel" style="margin-top: 2rem; border: 1px solid rgba(59, 130, 246, 0.2);">
          <div class="diag-header" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
            <div>
              <h3 style="margin: 0; color: #fff; font-size: 1.1rem;">Integridade Vetorial</h3>
              <p style="margin: 0; font-size: 0.8rem; color: var(--p-text-dim);">Valide o pipeline Gemini + Qdrant Cloud</p>
            </div>
            <button @click="runDiagnostic" :disabled="isDiagnosing" class="btn-diag" style="padding: 0.6rem 1.2rem; border-radius: 12px; background: rgba(59, 130, 246, 0.1); border: 1px solid var(--primary); color: #fff; cursor: pointer;">
              <span v-if="!isDiagnosing">⚡ EXECUTAR STRESS TEST</span>
              <span v-else>⏳ PROCESSANDO...</span>
            </button>
          </div>

          <div v-if="diagnosticResult" class="diag-results animate-fade-in" style="background: rgba(0,0,0,0.3); padding: 1.5rem; border-radius: 15px;">
            <div v-if="diagnosticResult.success" class="res-success">
               <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 1rem; margin-bottom: 1rem;">
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">GEMINI EMBED</span>
                    <b style="color: #4ade80;">{{ diagnosticResult.embed_ms }}ms</b>
                  </div>
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">QDRANT UPSERT</span>
                    <b style="color: #4ade80;">{{ diagnosticResult.qdrant_ms }}ms</b>
                  </div>
                  <div class="stat-box" style="text-align: center;">
                    <span style="font-size: 0.7rem; display: block; color: var(--p-text-dim);">TOTAL CICLO</span>
                    <b style="color: var(--primary);">{{ diagnosticResult.total_ms }}ms</b>
                  </div>
               </div>
               <div class="vector-preview">
                  <label style="font-size: 0.7rem; color: var(--p-text-dim);">VETOR GERADO (DUMP 5-DIM):</label>
                  <code style="display: block; background: #000; padding: 0.8rem; border-radius: 10px; font-size: 0.8rem; color: #3b82f6; margin-top: 0.5rem; border: 1px solid rgba(59, 130, 246, 0.3);">
                    {{ diagnosticResult.vector_preview }}...
                  </code>
               </div>
            </div>
            <div v-else class="res-error" style="color: #ef4444; font-size: 0.9rem;">
              ❌ ERRO NO DIAGNÓSTICO: {{ diagnosticResult.error }}
            </div>
          </div>
        </div>

        <div class="sec-card" style="margin-top: 2rem; margin-bottom: 2.5rem; padding: 1.5rem 2.5rem;">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #fff;">Modo Autônomo API</h5>
              <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Usar chave legada em vez de Sessões OAuth.</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="config.use_gemini_api_key" />
              <span class="m-slider-sec"></span>
           </label>
        </div>

        <div class="premium-form-group">
          <label>Claude API Key</label>
          <input v-model="config.claude_api_key" type="password" class="maestro-input" placeholder="••••••••" :disabled="!config.use_claude_api_key" />
        </div>

        <div class="sec-card" style="margin-bottom: 2.5rem; padding: 1.5rem 2.5rem;">
           <div class="sec-info">
              <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #fff;">Claude API Mode</h5>
              <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Habilitar injeção direta de chave Anthropic.</p>
           </div>
           <label class="hybrid-toggle-maestro">
              <input type="checkbox" v-model="config.use_claude_api_key" />
              <span class="m-slider-sec"></span>
           </label>
        </div>

        <button @click="save" class="btn-glow-blue" style="margin-top: 1.5rem; width: 100%;">SALVAR CHAVES</button>
      </section>

      <!-- ABA MOTORES (O CÉREBRO) -->
      <section v-if="activeTab === 'motores'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Hub de Motores e Orquestração</h2>
        <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
          Estação de controle dos núcleos de inteligência. Acompanhe a disponibilidade binária e inicie os daemons em background.
        </p>

        <div class="engine-cards-stack">
           <div v-for="tool in ['gemini', 'claude']" :key="tool" class="profile-card engine-showcase-card" :class="tool">
              <div class="engine-glow-backdrop"></div>
              
              <div style="position: relative; z-index: 2; height: 100%; display: flex; flex-direction: column;">
                <div style="display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 1.5rem;">
                   <div style="display: flex; align-items: center; gap: 1rem;">
                      <div class="avatar-glow maestro-engine-icon" :style="tool === 'gemini' ? 'background: linear-gradient(135deg, #3b82f6, #8b5cf6)' : 'background: linear-gradient(135deg, #f97316, #ea580c)'">
                         {{ tool === 'gemini' ? '⚡' : '🦾' }}
                      </div>
                      <div>
                        <h4 style="margin: 0; font-weight: 900; color: #fff; font-size: 1.3rem; letter-spacing: 2px;">{{ tool.toUpperCase() }}</h4>
                        <div class="engine-status-badge">
                          <span class="status-dot"></span> OPERACIONAL
                        </div>
                      </div>
                   </div>
                   
                   <!-- Auto-Start integrado como Toggle de topo em coluna única com nowrap -->
                   <div style="display: flex; flex-direction: column; align-items: center; gap: 6px; padding-top: 4px;">
                     <label class="hybrid-toggle-maestro" title="Ativar Auto-Start no Boot">
                        <input type="checkbox" :checked="isAutoStart(tool)" @change="toggleAutoStart(tool)" />
                        <span class="m-slider-sec" style="width: 44px; height: 22px;"></span>
                     </label>
                     <span style="font-size: 0.55rem; color: var(--p-text-dim); font-weight: 900; letter-spacing: 1px; white-space: nowrap;">AUTO-BOOT</span>
                   </div>
                </div>
                
                <p style="color: #cbd5e1; font-size: 0.85rem; margin-bottom: 2.5rem; line-height: 1.6; font-weight: 300; flex-grow: 1;">
                   {{ tool === 'gemini' ? 'Motor de Inteligência Central. Responsável pela execução de rotinas autônomas e retenção de contexto contínuo (ACP) em background.' : 'Motor Analítico Avançado. Infraestrutura secundária focada em modelagem pesada, testes lógicos e geração de códigos complexos.' }}
                </p>

                <div style="display: flex; gap: 12px; margin-top: auto;">
                   <button @click="install(tool)" class="unit-btn-solid" style="flex: 1.5;">
                     SINCRONIZAR
                   </button>
                   <button v-if="status.tools[tool]" @click="setup(tool)" class="unit-btn-glow" :style="getAuthStyle(tool)" style="flex: 1;">
                      {{ getAuthLabel(tool) }}
                   </button>
                </div>
              </div>
           </div>
        </div>
      </section>

      <!-- ABA CONTAS GEMINI (OAUTH) -->
      <section v-if="activeTab === 'contas'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Identidades Gemini OAuth</h2>
        <p class="subtitle-maestro" style="color: var(--p-text-dim); margin-bottom: 3rem; font-size: 1rem;">
          Gerencie múltiplas sessões isoladas do Google para alternar quotas de API e perfis em tempo real.
        </p>

        <div class="premium-form-group" style="display: flex; gap: 1.5rem; align-items: flex-end; margin-bottom: 4rem;">
          <div style="flex: 1;">
            <label>Nome da Nova Identidade</label>
            <input v-model="newAccName" placeholder="Ex: Trabalho, Pessoal, Pesquisa..." class="maestro-input" @keyup.enter="handleAddAccount" />
          </div>
          <button @click="handleAddAccount" class="btn-glow-blue" style="height: 60px; padding: 0 30px; font-size: 0.8rem;">
            CRIAR IDENTIDADE 💎
          </button>
        </div>

        <div class="accounts-grid-premium">
          <div v-for="acc in config.gemini_accounts" :key="acc.name" class="profile-card" :class="{ 'active-profile': acc.active }">
            <div class="profile-header" style="display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2rem;">
              <div class="avatar-glow">{{ acc.name[0].toUpperCase() }}</div>
              <div class="profile-meta">
                <h4 style="margin: 0; font-weight: 900; color: #fff; font-size: 1.2rem;">{{ acc.name }}</h4>
                <div class="status-chip" :style="{ color: acc.active ? 'var(--p-accent)' : '#475569' }">
                  {{ acc.active ? 'SESSÃO ATIVA' : 'MODO STANDBY' }}
                </div>
              </div>
            </div>
            
            <div class="profile-actions" style="display: flex; gap: 10px;">
              <button @click="handleLoginAccount(acc.name)" class="btn-util" style="border-color: var(--p-accent); color: var(--p-accent);">LOGAR 🔑</button>
              <button v-if="!acc.active" @click="handleSwitchAccount(acc.name)" class="btn-util" style="background: rgba(255,255,255,0.05);">ATIVAR</button>
              <!-- Botão de Excluir (Opcional, mas melhora UX) -->
              <button class="btn-util" style="flex: 0.4; border-color: rgba(239, 68, 68, 0.2); color: #ef4444;">×</button>
            </div>
          </div>
        </div>
      </section>

      <!-- ABA SEGURANÇA (FIREWALL PREMIER) -->
      <section v-if="activeTab === 'seguranca'" class="glass-panel animate-slide-up" style="border-color: rgba(239, 68, 68, 0.15);">
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
                
                <label class="hybrid-toggle-maestro">
                  <input type="checkbox" v-model="config.security[key]" />
                  <span class="m-slider-sec"></span>
                </label>
             </div>
         </div>
         <button @click="save" class="btn-glow-red" style="margin-top: 3rem; width: 100%;">
           SALVAR E REVALIDAR PROTOCOLOS DE SEGURANÇA 🔐
         </button>
      </section>

      <!-- ABA MCP -->
      <section v-if="activeTab === 'mcp'" class="glass-panel animate-slide-up">
        <h2 class="section-title">Model Context Protocol (MCP)</h2>
        <div class="mcp-restored-form">
           <div class="premium-form-group">
              <label>Identificador do Servidor</label>
              <input v-model="mcpName" placeholder="Ex: postgres, shopify, memory" class="maestro-input" />
           </div>
           <div class="premium-form-group">
              <label>Comando de Execução (Shell)</label>
              <input v-model="mcpCommand" placeholder="Ex: npx -y @modelcontextprotocol/server-postgres" class="maestro-input" />
           </div>
           <div class="mcp-actions-row" style="display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1rem; margin-top: 2rem;">
              <button @click="addMCPServer" class="btn-glow-blue" style="width: 100%;">INSTALAR SERVIDOR ⚡</button>
              <button @click="listMCPServers" class="btn-outline" style="width: 100%;">LISTAR REGISTRADOS 📋</button>
           </div>
           <div v-if="showMcpList" class="mcp-output-container">
              <div class="output-header">SERVIDORES CONFIGURADOS</div>
              <pre class="mcp-output-box">{{ mcpServers }}</pre>
           </div>
        </div>
      </section>
    </div>

    <!-- Terminal de Logs (Restored Logic) -->
    <footer class="maestro-terminal-v2" v-show="installStatus !== '' || installLogs.length > 0">
      <div class="t-bar">
         <span class="t-title">SYSTEM_ORCHESTRATOR_OUTPUT</span>
         <div class="t-pulse"><span></span> ACTIVE</div>
      </div>
      <div class="t-contents" ref="logContainer">
        <div v-for="(log, i) in installLogs" :key="i" class="t-entry">> {{ log }}</div>
        <div v-if="installStatus" class="t-status">>> {{ installStatus }}</div>
      </div>
    </footer>
  </main>
</template>

<style scoped>
/* --- SISTEMA DE DESIGN PREMIER --- */
:root {
  --p-bg: #030712;
  --p-accent: #3b82f6;
  --p-accent-glow: rgba(59, 130, 246, 0.4);
  --p-glass: rgba(255, 255, 255, 0.02);
  --p-border: rgba(255, 255, 255, 0.06);
  --p-text: #f8fafc;
  --p-text-dim: #94a3b8;
  --p-error: #ef4444;
}

.settings-view { 
  padding: 4rem 6rem; 
  color: var(--p-text); 
  background: var(--p-bg); 
  background-image: 
    radial-gradient(at 0% 0%, rgba(59, 130, 246, 0.05) 0px, transparent 50%),
    radial-gradient(at 100% 100%, rgba(139, 92, 246, 0.05) 0px, transparent 50%);
  min-height: 100vh;
  width: 100%;
  box-sizing: border-box;
  font-family: 'Outfit', 'Inter', sans-serif; 
  overflow-y: visible; /* Mudança crucial para permitir scroll */
}

.brand-badge { 
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.2), rgba(139, 92, 246, 0.2));
  color: #fff;
  padding: 6px 14px; 
  border-radius: 20px; 
  font-size: 0.65rem; 
  font-weight: 900; 
  letter-spacing: 2px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: inline-block; 
  margin-bottom: 1.5rem;
  text-transform: uppercase;
}

.gradient-text { 
  font-size: 4rem; 
  font-weight: 900; 
  letter-spacing: -3px;
  background: linear-gradient(135deg, #fff 20%, #64748b 100%); 
  -webkit-background-clip: text; 
  color: transparent; 
  margin-bottom: 0.5rem; 
}

.subtitle { color: var(--p-text-dim); font-size: 1.1rem; margin-bottom: 3rem; font-weight: 400; }

.tabs-nav-glass { 
  display: flex; 
  gap: 8px; 
  margin-bottom: 4rem; 
  background: rgba(255,255,255,0.01); 
  padding: 8px; 
  border-radius: 16px; 
  width: fit-content;
  border: 1px solid var(--p-border);
}

.tab-btn-premium { 
  background: none; 
  border: none; 
  padding: 12px 28px; 
  color: var(--p-text-dim); 
  font-weight: 700; 
  font-size: 0.85rem;
  cursor: pointer; 
  border-radius: 12px; 
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.tab-btn-premium.active { 
  color: #fff; 
  background: var(--p-accent);
  box-shadow: 0 0 25px var(--p-accent-glow);
}

.glass-panel { 
  background: var(--p-glass); 
  border: 1px solid var(--p-border); 
  border-radius: 32px; 
  padding: 4rem; 
  backdrop-filter: blur(40px);
  box-shadow: 0 40px 100px rgba(0,0,0,0.5);
}

.section-title { 
  font-size: 0.8rem; 
  text-transform: uppercase; 
  color: var(--p-accent); 
  font-weight: 900;
  letter-spacing: 4px; 
  margin-bottom: 3rem; 
  display: flex;
  align-items: center;
  gap: 15px;
}

.section-title::after { content: ''; flex: 1; height: 1px; background: linear-gradient(to right, var(--p-border), transparent); }

/* Inputs & Form */
.premium-form-group { margin-bottom: 2.5rem; }
.premium-form-group label { 
  display: block; 
  font-weight: 800; 
  font-size: 0.85rem; 
  color: var(--p-text-dim); 
  margin-bottom: 1rem; 
  text-transform: uppercase;
  letter-spacing: 1px;
}

.maestro-input { 
  background: rgba(0, 0, 0, 0.6) !important; 
  border: 1px solid var(--p-border) !important; 
  color: #fff !important; 
  padding: 18px 24px !important; 
  border-radius: 14px !important; 
  width: 100%; 
  font-size: 0.95rem !important;
  transition: all 0.3s ease;
  font-family: 'Inter', sans-serif;
}

.maestro-input:focus { 
  border-color: var(--p-accent) !important; 
  box-shadow: 0 0 40px rgba(59, 130, 246, 0.15) !important; 
  background: #000 !important;
  outline: none;
}

.btn-glow-blue { 
  background: linear-gradient(135deg, #3b82f6, #2563eb); 
  border: none; 
  color: #fff; 
  font-weight: 800; 
  padding: 20px 40px; 
  border-radius: 18px; 
  cursor: pointer; 
  transition: 0.4s;
  letter-spacing: 1px;
  font-size: 0.9rem;
}

.btn-glow-blue:hover { 
  transform: translateY(-3px) scale(1.02); 
  box-shadow: 0 15px 40px var(--p-accent-glow);
}

.maestro-range { 
  width: 100%; 
  height: 8px; 
  appearance: none; /* Lint Fix */
  -webkit-appearance: none; 
  background: rgba(255,255,255,0.05); 
  border-radius: 20px; 
  margin: 20px 0;
  outline: none;
}

.maestro-range::-webkit-slider-thumb { 
  -webkit-appearance: none; 
  width: 24px; 
  height: 24px; 
  background: var(--p-accent); 
  border-radius: 50%; 
  cursor: pointer;
  box-shadow: 0 0 15px var(--p-accent-glow);
  border: 3px solid #000;
  transition: 0.3s;
}

.maestro-range::-webkit-slider-thumb:hover { transform: scale(1.2); filter: brightness(1.2); }

.highlight-val { color: var(--p-accent); font-weight: 900; margin-left: 8px; font-size: 1.1rem; }

/* Security Matrix */
.security-grid-comprehensive { 
  display: grid; 
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); 
  gap: 1.5rem; 
}

.sec-card { 
  padding: 2.5rem; 
  display: flex; 
  justify-content: space-between; 
  align-items: center; 
  background: rgba(255,255,255,0.02); 
  border-radius: 24px; 
  border: 1px solid var(--p-border);
  transition: all 0.3s;
}

.sec-card:hover { border-color: var(--p-accent); background: rgba(59, 130, 246, 0.03); }

.sec-card.critical-sec {
  border-color: rgba(239, 68, 68, 0.2);
  background: rgba(239, 68, 68, 0.02);
}

.sec-card.critical-sec:hover {
  border-color: #ef4444;
  box-shadow: 0 0 30px rgba(239, 68, 68, 0.15);
}

/* Toggle Segurança Elite */
.hybrid-toggle-maestro {
  cursor: pointer;
  user-select: none;
}

.m-slider-sec {
  position: relative;
  display: inline-block;
  width: 48px;
  height: 24px;
  background: #1e293b;
  border-radius: 20px;
  transition: 0.3s;
}

.m-slider-sec::before {
  content: '';
  position: absolute;
  width: 16px;
  height: 16px;
  left: 4px;
  bottom: 4px;
  background: #475569;
  border-radius: 50%;
  transition: 0.3s;
}

.hybrid-toggle-maestro input:checked + .m-slider-sec { background: var(--p-accent); }
.critical-sec .hybrid-toggle-maestro input:checked + .m-slider-sec { background: #ef4444; }

.hybrid-toggle-maestro input:checked + .m-slider-sec::before { 
  transform: translateX(24px); 
  background: #fff; 
  box-shadow: 0 0 10px #fff;
}

.hybrid-toggle-maestro input { display: none; }

.btn-glow-red {
  background: linear-gradient(135deg, #ef4444, #991b1b);
  border: none;
  color: #fff;
  font-weight: 900;
  padding: 22px;
  border-radius: 18px;
  cursor: pointer;
  transition: 0.4s;
  letter-spacing: 2px;
  box-shadow: 0 10px 30px rgba(239, 68, 68, 0.2);
}

.btn-glow-red:hover {
  transform: translateY(-3px);
  box-shadow: 0 15px 50px rgba(239, 68, 68, 0.4);
  filter: brightness(1.2);
}

/* --- ACCOUNTS & ENGINES HUB --- */
.accounts-grid-premium, .engine-cards-stack { 
  display: grid; 
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); 
  gap: 1.5rem; 
  margin-top: 1.5rem;
}

.profile-card, .engine-unit { 
  background: rgba(255, 255, 255, 0.02); 
  border: 1px solid var(--p-border); 
  border-radius: 24px; 
  padding: 2rem; 
  position: relative;
  transition: 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
  backdrop-filter: blur(10px);
}

.profile-card:hover, .engine-unit:hover {
  border-color: var(--p-accent);
  background: rgba(59, 130, 246, 0.04);
  transform: translateY(-5px);
  box-shadow: 0 20px 40px rgba(0,0,0,0.3);
}

.profile-card.active-profile { border-color: var(--p-accent); background: rgba(59, 130, 246, 0.06); }

.avatar-glow, .unit-icon { 
  width: 50px; height: 50px; 
  background: linear-gradient(135deg, #3b82f6, #8b5cf6); 
  border-radius: 14px; 
  display: flex; align-items: center; justify-content: center; 
  font-weight: 900; font-size: 1.2rem;
  box-shadow: 0 0 20px rgba(59, 130, 246, 0.4);
  margin-bottom: 1.5rem;
}

.unit-head h4 { font-size: 1.1rem; font-weight: 900; letter-spacing: 2px; color: #fff; margin: 0 0 1rem 0; text-transform: uppercase; }

.unit-actions, .profile-actions { display: flex; gap: 10px; margin-top: 1rem; }

.btn-util, .unit-btn { 
  background: var(--p-glass); 
  border: 1px solid var(--p-border); 
  color: #fff; 
  padding: 12px 16px; 
  border-radius: 12px; 
  font-weight: 800; 
  font-size: 0.75rem; 
  cursor: pointer; 
  transition: 0.3s;
  flex: 1;
  text-transform: uppercase;
}

.btn-util:hover, .unit-btn:hover { background: #fff; color: #000; box-shadow: 0 0 20px rgba(255,255,255,0.2); }
.unit-btn.auth { border-color: var(--p-accent); color: var(--p-accent); }
.unit-btn.auth:hover { background: var(--p-accent); color: #fff; }

.unit-footer { margin-top: 1.5rem; padding-top: 1.5rem; border-top: 1px solid var(--p-border); }
.mini-toggle { display: flex; align-items: center; gap: 12px; font-size: 0.7rem; font-weight: 900; color: var(--p-text-dim); cursor: pointer; }
.m-slider { position: relative; width: 40px; height: 20px; background: #1e293b; border-radius: 20px; transition: 0.3s; }
.m-slider::before { content: ''; position: absolute; width: 12px; height: 12px; left: 4px; bottom: 4px; background: #64748b; border-radius: 50%; transition: 0.3s; }
input:checked + .m-slider { background: var(--p-accent); }
input:checked + .m-slider::before { transform: translateX(20px); background: #fff; box-shadow: 0 0 10px #fff; }
.mini-toggle input { display: none; }

/* --- TERMINAL OUTPUT --- */
.maestro-terminal-v2 { 
  position: fixed; 
  bottom: 2.5rem; 
  right: 2.5rem; 
  width: 500px; 
  background: #000; 
  border: 1px solid var(--p-border); 
  border-radius: 20px; 
  box-shadow: 0 30px 60px rgba(0,0,0,0.8);
  z-index: 1000;
  backdrop-filter: blur(20px);
}

.t-bar { 
  padding: 15px 20px; 
  background: rgba(255,255,255,0.02); 
  display: flex; 
  justify-content: space-between; 
  font-size: 0.65rem; 
  color: #475569; 
  font-weight: 900; 
  border-bottom: 1px solid var(--p-border);
}

.t-contents { padding: 2rem; font-family: 'Fira Code', monospace; font-size: 0.8rem; max-height: 250px; overflow-y: auto; color: #94a3b8; line-height: 1.6; }
.t-entry { margin-bottom: 6px; }
.mcp-restored-form { display: flex; flex-direction: column; gap: 1rem; }
.mcp-actions-row { display: flex; gap: 1rem; margin-top: 1rem; }

.btn-outline {
  background: transparent;
  border: 1px solid var(--p-border);
  color: var(--p-text-dim);
  padding: 16px 32px;
  border-radius: 12px;
  font-weight: 800;
  font-size: 0.8rem;
  cursor: pointer;
  transition: 0.3s;
  text-transform: uppercase;
}

.btn-outline:hover { border-color: #fff; color: #fff; background: rgba(255,255,255,0.05); }

.mcp-output-container { margin-top: 2rem; }
.output-header { font-size: 0.65rem; font-weight: 900; color: var(--p-accent); letter-spacing: 2px; margin-bottom: 10px; }

.mcp-output-box { 
  background: #000; 
  padding: 2rem; 
  border-radius: 16px; 
  color: #4ade80; 
  font-size: 0.85rem; 
  border: 1px solid var(--p-border); 
  font-family: 'Fira Code', monospace;
  overflow: auto;
  max-height: 300px;
}

/* Custom Scrollbar */
::-webkit-scrollbar { width: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: var(--p-border); border-radius: 10px; }
::-webkit-scrollbar-thumb:hover { background: var(--p-accent); }

/* --- ENGINE SHOWCASE LUXURY DESIGN --- */
.engine-showcase-card {
  position: relative;
  border-top: 1px solid rgba(255,255,255,0.1);
  background: linear-gradient(180deg, rgba(255,255,255,0.03) 0%, rgba(0,0,0,0.4) 100%);
}

.engine-glow-backdrop {
  position: absolute;
  top: -50px; left: -50px; right: -50px; height: 150px;
  background: radial-gradient(ellipse at top left, rgba(59, 130, 246, 0.15) 0%, transparent 70%);
  z-index: 1;
  pointer-events: none;
}

.engine-showcase-card.claude .engine-glow-backdrop {
  background: radial-gradient(ellipse at top left, rgba(249, 115, 22, 0.12) 0%, transparent 70%);
}

.engine-status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 0.55rem;
  letter-spacing: 1.5px;
  font-weight: 900;
  padding: 4px 10px;
  border-radius: 20px;
  background: rgba(74, 222, 128, 0.08);
  color: #4ade80;
  border: 1px solid rgba(74, 222, 128, 0.2);
  margin-top: 8px;
}

.status-dot { 
  width: 6px; height: 6px; 
  background: #4ade80; 
  border-radius: 50%; 
  box-shadow: 0 0 8px #4ade80; 
}

.unit-btn-solid {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #fff;
  padding: 14px 20px;
  border-radius: 14px;
  font-weight: 800;
  font-size: 0.75rem;
  cursor: pointer;
  transition: 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.unit-btn-solid:hover {
  background: var(--p-accent);
  border-color: var(--p-accent);
  box-shadow: 0 10px 25px rgba(59, 130, 246, 0.3);
  transform: translateY(-2px);
}

.unit-btn-glow {
  background: transparent;
  border: 1px solid var(--p-border);
  color: var(--p-text-dim);
  padding: 14px 20px;
  border-radius: 14px;
  font-weight: 800;
  font-size: 0.75rem;
  cursor: pointer;
  transition: 0.3s;
  text-transform: uppercase;
}
.unit-btn-glow:hover {
  background: rgba(255,255,255,0.1);
  color: #fff;
  border-color: rgba(255,255,255,0.2);
}
</style>
