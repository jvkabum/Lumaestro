<script setup>
import { storeToRefs } from 'pinia'
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useOrchestratorStore } from '../stores/orchestrator'
import ChatInput from './ChatInput.vue'
import ChatLog from './ChatLog.vue'
import PlanView from './PlanView.vue'
import ReviewBlock from './ReviewBlock.vue'
import SubagentPanel from './SubagentPanel.vue'
import TerminalView from './TerminalView.vue'

// Props e Emits
const props = defineProps({ isMinimized: { type: Boolean, default: false } })
const emit = defineEmits(['toggle-minimize'])

// --- Uso da Store (Pinia) ---
const orchestrator = useOrchestratorStore()
const settingsStore = useSettingsStore()
const { messages, isThinking, isNavigating, isTerminalMode, activeAgent, runningSessions, pendingReview, modelStats } = storeToRefs(orchestrator)

const getAgentStatusLabel = () => {
  const agent = activeAgent.value
  if (!agent) return 'PRONTO'
  
  if (isThinking.value) return 'PENSANDO...'

  // Se for LM Studio, checamos apenas se está habilitado
  if (agent === 'lmstudio') {
    return (settingsStore.config.lmstudio_enabled && settingsStore.config.lmstudio_url) ? 'PRONTO' : 'OFFLINE'
  }

  // Checa instalação e autenticação via status centralizado
  const toolStatus = settingsStore.status.tools[agent]
  const authStatus = settingsStore.status.tools[agent + '_auth']
  const useKey = settingsStore.config[`use_${agent}_api_key`]

  if (!toolStatus) return 'NÃO INSTALADO'
  if (!useKey && !authStatus) return 'ERRO AUTH'
  
  return 'PRONTO'
}

// --- Estados Locais de UI ---
const logContainer = ref(null)
const showRawTerminal = ref(false)
const showProjectDropdown = ref(false)

// O Terminal Bruto (Raw) só deve abrir via botão ou comando explícito (/cmd)
// para garantir que a experiência primária (Chat) não seja interrompida.


// Inicializa a escuta de eventos do Backend Go
onMounted(() => {
  orchestrator.initListeners()
})

// --- Ações de UI ---
const sendChatMessage = async (payload) => {
  const text = typeof payload === 'string' ? payload : payload.text
  if (!text.trim()) return

  // Roteamento de Comandos
  try {
    if (text.startsWith('/cmd ')) {
      const agentName = text.replace('/cmd ', '').trim()
      await orchestrator.startSession(agentName || 'gemini')
      showRawTerminal.value = true 
      return
    }

    if (text === '/exit' || text === '/quit') {
      await orchestrator.stopSession()
      return
    }

    if (text === '/scan') {
      await orchestrator.runScan()
      return
    }

    // Envio Padrão (Multimodal)
    const targetAgent = payload.agent || 'gemini'
    const isActMode = payload.mode === 'act'
    const images = payload.images || []

    if (isActMode) {
      console.log("[ChatPanel] Modo ACT detectado para:", targetAgent);
      // Garante que a sessão está ativa antes de enviar
      if (!runningSessions.value.includes(targetAgent)) {
        await orchestrator.startSession(targetAgent)
        await new Promise(r => setTimeout(r, 500))
      }
      await orchestrator.sendInput(targetAgent, text, images)
    } else {
      console.log("[ChatPanel] Modo CHAT detectado para:", targetAgent);
      await orchestrator.ask(targetAgent, text, images)
    }
  } catch (err) {
    console.error("[ChatPanel] Falha crítica no envio:", err);
    // Injeta erro visual para o usuário
    orchestrator.messages.push({
      role: 'assistant',
      text: `❌ Falha na Sinfonia: Não foi possível enviar a mensagem. (${err.message || 'Erro de Conexão'})`,
      mode: 'system'
    });
  }
}

const handleSwitchProject = async (proj) => {
  showProjectDropdown.value = false
  const { SetWorkspace, GetWorkspace } = await import('../../wailsjs/go/core/App')
  try {
    await SetWorkspace(proj.path)
    const updatedWs = await GetWorkspace()
    orchestrator.workspace = updatedWs
    settingsStore.notify(`🚀 Órbita alterada para: ${proj.core_node}`, "success")
  } catch (err) {
    settingsStore.notify(`❌ Falha na transição: ${err}`, "error")
  }
}

const handleSessionEnded = (agent) => {
    console.log('[ChatPanel] Sessão encerrada:', agent)
}
</script>

<template>
  <div class="chat-panel-parent">
    <!-- Grade de Fundo Sutil -->
    <div class="panel-grain"></div>

    <!-- Sistema de Revisão de Segurança (ACP Hands) -->
    <ReviewBlock v-if="pendingReview" :review="pendingReview" />

    <!-- 🐝 Monitor de Enxame de Subagentes -->
    <SubagentPanel />

    <!-- 📋 Overlay de Plano de Execução -->
    <PlanView />

    <header class="panel-header glass" :class="{ 'is-minimized': props.isMinimized }">
      <!-- 🚀 LADO ESQUERDO: Identidade e Status Compacto -->
      <div class="header-section section-left" v-show="!props.isMinimized">
        <div class="maestro-brand">
          <span class="orchestra-icon">🎻</span>
          <div class="brand-text">
            <h2>MAESTRO</h2>
            <div class="status-indicator">
              <span class="status-led" :class="getAgentStatusLabel().toLowerCase() === 'pronto' ? 'led-ready' : 'led-busy'"></span>
              <span class="status-label">{{ getAgentStatusLabel() }}</span>
            </div>
          </div>
        </div>

        <!-- 🚀 [Verde] Identidade de Perfil e Telemetria -->
        <div class="identity-badges">
          <span 
            v-if="orchestrator.activeProfile" 
            class="active-agent-badge" 
            :class="orchestrator.activeProfile.name.toLowerCase()"
          >
            {{ orchestrator.activeProfile.name.toUpperCase() }}
          </span>
          
          <div v-if="activeAgent && modelStats.agent === activeAgent && modelStats.info" class="quota-badge glass" title="Performance e Uso do Modelo">
             <span class="quota-icon">⚡</span>
             <span class="quota-value">{{ modelStats.info }}</span>
          </div>
        </div>
      </div>

      <!-- 🪐 CENTRO: Ilha Flutuante de Órbita (Vermelho - Mixer) -->
      <div class="header-section section-center" v-show="!props.isMinimized">
        <div class="workspace-island glass">
          <span class="ws-icon" @click="orchestrator.selectWorkspace()" title="Escolher Nova Pasta...">📂</span>
          <div class="ws-selector" @click="showProjectDropdown = !showProjectDropdown" title="Alternar entre Sistemas Solares">
            <span class="ws-name">{{ orchestrator.workspace?.path ? orchestrator.workspace.path.split(/[/\\]/).pop() : 'Nenhuma Órbita Ativa' }}</span>
            <span class="ws-arrow">▼</span>
            
            <Transition name="slide-up">
              <div v-if="showProjectDropdown" class="orbit-dropdown glass" @click.stop>
                <div class="dropdown-header">SISTEMAS EM ÓRBITA</div>
                <div 
                  v-for="proj in settingsStore.config.external_projects" 
                  :key="proj.path" 
                  class="orbit-item"
                  :class="{ 'is-active': proj.path === orchestrator.workspace.path }"
                  @click="handleSwitchProject(proj)"
                >
                  <span class="item-icon">🪐</span>
                  <div class="item-info">
                    <span class="item-name">{{ proj.core_node }}</span>
                    <span class="item-path">{{ proj.path }}</span>
                  </div>
                </div>
                <div class="dropdown-footer" @click="orchestrator.setView('repos'); showProjectDropdown = false">
                  + GERENCIAR PROJETOS
                </div>
              </div>
            </Transition>
          </div>
        </div>
      </div>

      <div class="header-section section-right" :class="{ 'actions-vertical': props.isMinimized }">
        <!-- Toggle Terminal View -->
        <button v-show="!props.isMinimized" @click="showRawTerminal = !showRawTerminal" class="action-btn" :class="{ 'btn-active': showRawTerminal }" title="Alternar Terminal Bruto">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2"></rect>
            <line x1="8" y1="21" x2="16" y2="21"></line>
            <line x1="12" y1="17" x2="12" y2="21"></line>
          </svg>
        </button>

        <!-- 📋 Botão de Visualização de Plano -->
        <button @click="orchestrator.showPlanOverlay = true" class="action-btn" :class="{ 'btn-active-plan': orchestrator.isPlanMode }" title="Visualizar Plano">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            <polyline points="14 2 14 8 20 8"></polyline>
            <line x1="16" y1="13" x2="8" y2="13"></line>
            <line x1="16" y1="17" x2="8" y2="17"></line>
            <polyline points="10 9 9 9 8 9"></polyline>
          </svg>
        </button>

        <!-- Botão Discreto de Histórico (Relógio/Lista) -->
        <button @click="orchestrator.toggleSidebar()" class="action-btn" :class="{ 'btn-active-history': orchestrator.isSidebarOpen }" title="Expandir Histórico de Sinfonias">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="12 8 12 12 14 14"></polyline>
            <path d="M3.05 11a9 9 0 1 1 .5 4m-.5 5v-5h5"></path>
          </svg>
        </button>
        
        <!-- 🔽 Botão Minimizar/Maximizar Chat -->
        <button @click="emit('toggle-minimize')" class="action-btn" :title="props.isMinimized ? 'Expandir Chat' : 'Minimizar Chat'">
          <svg v-if="!props.isMinimized" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="13 17 18 12 13 7"></polyline>
            <polyline points="6 17 11 12 6 7"></polyline>
          </svg>
          <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="11 17 6 12 11 7"></polyline>
            <polyline points="18 17 13 12 18 7"></polyline>
          </svg>
        </button>

        <button v-if="isTerminalMode" @click="orchestrator.stopSession()" class="exit-btn-circle" title="Encerrar Sessão">
           <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="3">
             <line x1="18" y1="6" x2="6" y2="18"></line>
             <line x1="6" y1="6" x2="18" y2="18"></line>
           </svg>
        </button>
      </div>
    </header>

    <!-- Área Principal do Chat (Premium) -->
    <div v-show="!showRawTerminal && !props.isMinimized" class="chat-main-area" ref="logContainer">
      <div class="chat-scroll-boundary">
        <!-- Tela de Harmonização Inicial (Loading Screen) -->
        <Transition name="fade">
          <div v-if="isThinking && messages.length === 0" class="loading-overlay glass">
            <div class="loader-content">
              <div class="pulsing-icon">🎻</div>
              <div class="loader-text">
                <h3>Afinando instrumentos...</h3>
                <p>O Maestro está sintonizando com o Gemini.</p>
              </div>
              <div class="loader-bars">
                <span></span><span></span><span></span><span></span><span></span>
              </div>
            </div>
          </div>
        </Transition>

        <ChatLog :messages="messages" :is-thinking="isThinking" />

        <!-- 📡 Pulso de Atividade: Mostra o que a IA está fazendo AGORA (Anti-Travamento) -->
        <Transition name="status-fade">
          <div v-if="orchestrator.currentStatus?.action" class="activity-status-bar glass">
            <div class="activity-pulse"></div>
            <div class="activity-info">
              <span v-if="orchestrator.currentStatus?.tool" class="activity-tool">{{ String(orchestrator.currentStatus.tool).replace('_', ' ').toUpperCase() }}</span>
              <span class="activity-text">{{ orchestrator.currentStatus.action }}</span>
            </div>
          </div>
        </Transition>

        <!-- Indicador de Navegação do Grafo (Context Flow) -->
        <Transition name="slide-up">
          <div v-if="isNavigating" class="navigation-status glass">
            <span class="nav-pulse"></span>
            <span class="nav-text">Explorando Base de Conhecimento...</span>
          </div>
        </Transition>
      </div>
      <div class="input-persistent-area">
        <ChatInput @send="sendChatMessage" :is-thinking="isThinking" />
      </div>
    </div>

    <!-- Terminal Real com TABS de Agentes -->
    <div v-show="showRawTerminal && !props.isMinimized" class="raw-terminal-view">
      <div class="terminal-overlay-header">
        <div class="terminal-tabs">
          <button
            v-for="agent in runningSessions"
            :key="agent"
            class="terminal-tab"
            :class="{ active: activeAgent === agent, gemini: agent === 'gemini', claude: agent === 'claude' }"
            @click="orchestrator.switchAgent(agent)"
          >
            <span class="tab-dot"></span>
            {{ agent }}
          </button>
          <span v-if="runningSessions.length === 0" class="no-sessions">Nenhuma sessão ativa</span>
        </div>
        <div class="terminal-actions">
          <button @click="showRawTerminal = false" class="back-btn">Chat View</button>
        </div>
      </div>
      <!-- Uma instância de TerminalView para cada agente ativo -->
       <div class="terminal-stack">
        <TerminalView
          v-for="agent in runningSessions"
          :key="agent"
          :agent="agent"
          :active="activeAgent === agent"
          @session-ended="handleSessionEnded"
        />
       </div>
    </div>
  </div>
</template>

<style scoped>
.chat-panel-parent {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #0f172a; /* Slate 900 mais profundo */
  position: relative;
  overflow: hidden;
  border-radius: 12px 12px 0 0;
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4);
}

.panel-grain {
  position: absolute;
  inset: 0;
  background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noiseFilter'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.65' numOctaves='3' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noiseFilter)'/%3E%3C/svg%3E");
  opacity: 0.02;
  pointer-events: none;
  z-index: 1;
}

.panel-header {
  height: 70px;
  display: grid;
  grid-template-columns: 1fr 2fr 1fr;
  align-items: center;
  padding: 0 20px;
  background: rgba(15, 23, 42, 0.5);
  backdrop-filter: blur(20px);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  z-index: 100;
  position: relative;
}

/* Quando o painel está minimizado: header vira coluna vertical */
.chat-panel-parent:has(.chat-main-area[style*="display: none"]) .panel-header,
.chat-panel-parent .panel-header.is-minimized {
  flex-direction: column;
  height: 100%;
  padding: 16px 0;
  justify-content: flex-start;
  gap: 16px;
}


.header-section {
  display: flex;
  align-items: center;
}

.section-center {
  justify-content: center;
}

.section-right {
  justify-content: flex-end;
  gap: 12px;
}

/* 🎻 LADO ESQUERDO: Estilo Compacto */
.maestro-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.orchestra-icon { font-size: 1.2rem; filter: drop-shadow(0 0 8px rgba(59, 130, 246, 0.5)); }

.brand-text h2 {
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 2px;
  margin: 0;
  color: #fff;
}

.status-indicator {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 2px;
}

.status-led {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  box-shadow: 0 0 5px rgba(255,255,255,0.2);
}

.led-ready { background: #10b981; box-shadow: 0 0 8px #10b981; animation: led-pulse 2s infinite; }
.led-busy { background: #f59e0b; box-shadow: 0 0 8px #f59e0b; }

@keyframes led-pulse {
  0% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(0.8); }
  100% { opacity: 1; transform: scale(1); }
}

.status-label {
  font-size: 8px;
  font-weight: 800;
  color: #94a3b8;
  letter-spacing: 0.5px;
}

.active-agent-badge {
  margin-left: 12px;
  font-size: 8px;
  font-weight: 900;
  padding: 1px 8px;
  border-radius: 4px;
  background: rgba(255,255,255,0.05);
  color: #64748b;
  border: 1px solid rgba(255,255,255,0.05);
  height: 16px;
  display: flex;
  align-items: center;
}

.active-agent-badge.gemini { background: rgba(59, 130, 246, 0.1); color: #60a5fa; }
.active-agent-badge.claude { background: rgba(16, 185, 129, 0.1); color: #34d399; }
.active-agent-badge.standby { background: rgba(148, 163, 184, 0.1); color: #94a3b8; }


/* 🪐 Mixer: Ilha de Workspace (Floating Island) */
.header-section.section-center {
  flex: 1;
  display: flex;
  justify-content: center;
  pointer-events: none; /* Deixa cliques passarem para o grafo se necessário, mas os filhos reativam */
}

.workspace-island {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 14px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-radius: 100px;
  border: 1px solid rgba(139, 92, 246, 0.15); /* Purple hint */
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3), inset 0 1px 1px rgba(255, 255, 255, 0.05);
  pointer-events: auto;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.workspace-island:hover {
  background: rgba(139, 92, 246, 0.1);
  border-color: rgba(168, 85, 247, 0.4);
  box-shadow: 0 6px 24px rgba(168, 85, 247, 0.15), inset 0 1px 1px rgba(255, 255, 255, 0.1);
  transform: translateY(-1px);
}

.ws-icon {
  font-size: 13px;
  cursor: pointer;
  opacity: 0.8;
  transition: all 0.3s;
  padding-right: 12px;
  border-right: 1px solid rgba(255, 255, 255, 0.1);
}
.workspace-island:hover .ws-icon { opacity: 1; filter: drop-shadow(0 0 6px rgba(168,85,247,0.5)); }

.ws-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  position: relative;
  padding: 2px 4px;
}

.ws-name {
  font-size: 11px;
  font-weight: 800;
  color: #f8fafc;
  letter-spacing: 0.5px;
  max-width: 180px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ws-arrow {
  font-size: 9px;
  color: #a78bfa;
  transition: transform 0.3s;
}
.workspace-island:hover .ws-arrow { color: #d8b4fe; }

/* Dropdown de Órbita */
.orbit-dropdown {
  position: absolute;
  top: calc(100% + 15px);
  left: 50%;
  transform: translateX(-50%);
  width: 340px;
  background: rgba(15, 23, 42, 0.95);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  border: 1px solid rgba(139, 92, 246, 0.15); /* Soft purple border */
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255,255,255,0.05) inset;
  padding: 8px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.dropdown-header {
  font-size: 10px;
  font-weight: 800;
  color: #c084fc;
  letter-spacing: 2px;
  margin-bottom: 8px;
  padding: 8px 8px 4px 8px;
  text-transform: uppercase;
}

.orbit-item {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 12px;
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  border: 1px solid transparent;
}

.orbit-item:hover { 
  background: rgba(168, 85, 247, 0.08); 
}

.orbit-item.is-active { 
  background: rgba(168, 85, 247, 0.12); 
  border: 1px solid rgba(168, 85, 247, 0.3);
  box-shadow: 0 4px 15px rgba(168, 85, 247, 0.15);
  position: relative;
  overflow: hidden;
}

.orbit-item.is-active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  height: 100%;
  width: 4px;
  background: #c084fc;
  box-shadow: 0 0 10px #c084fc;
}

.item-name { display: block; font-size: 13px; font-weight: 800; color: #f8fafc; }
.item-path { display: block; font-size: 10px; color: #94a3b8; margin-top: 4px; font-family: 'Fira Code', monospace; }
.item-icon { font-size: 20px; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.3)); }

.dropdown-footer {
  margin-top: 4px;
  padding: 12px;
  text-align: center;
  font-size: 11px;
  font-weight: 800;
  color: #c084fc;
  background: rgba(168, 85, 247, 0.05);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  justify-content: center;
  align-items: center;
}

.dropdown-footer:hover { 
  background: rgba(168, 85, 247, 0.15);
  color: #e9d5ff;
}

/* 🎻 Maestro Brand Styling */
.maestro-brand { display: flex; align-items: center; gap: 12px; }
.brand-text h2 {
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 2.5px;
  color: #f1f5f9;
  margin: 0;
}

.status-indicator { display: flex; align-items: center; gap: 6px; margin-top: 2px; }
.status-led { width: 5px; height: 5px; border-radius: 50%; background: #64748b; }
.status-led.led-ready { background: #10b981; box-shadow: 0 0 8px #10b981; }
.status-led.led-busy { background: #3b82f6; box-shadow: 0 0 8px #3b82f6; }
.status-label { font-size: 8px; font-weight: 800; color: #64748b; text-transform: uppercase; }

.identity-badges { display: flex; align-items: center; gap: 6px; margin-left: 10px; padding-left: 10px; border-left: 1px solid rgba(255, 255, 255, 0.05); }

.header-section.section-right { display: flex; align-items: center; gap: 10px; }

/* 📊 Monitor de Cotas Badge */
.quota-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.03);
  padding: 2px 10px;
  border-radius: 100px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  margin-left: 8px;
  transition: all 0.3s;
}

.quota-badge:hover {
  background: rgba(59, 130, 246, 0.1);
  border-color: rgba(59, 130, 246, 0.2);
}

.quota-icon { font-size: 10px; color: #fbbf24; }
.quota-value { font-size: 10px; font-weight: 800; color: #94a3b8; letter-spacing: 0.5px; }

.header-actions { display: flex; align-items: center; gap: 10px; }

.actions-vertical {
  flex-direction: column;
  justify-content: flex-start;
  gap: 8px;
  padding-top: 4px;
  width: 100%;
  align-items: center;
}


.action-btn {
  background: transparent; border: none; color: #64748b; cursor: pointer;
  padding: 8px; border-radius: 8px; transition: all 0.2s;
}
.action-btn:hover { background: rgba(255, 255, 255, 0.05); color: #fff; }
.action-btn.btn-active-history { color: #38bdf8; background: rgba(56, 189, 248, 0.1); border: 1px solid rgba(56, 189, 248, 0.2); }
.action-btn.btn-active { color: #3b82f6; background: rgba(59, 130, 246, 0.1); }
.action-btn.btn-active-plan { color: #c084fc; background: rgba(168, 85, 247, 0.1); border: 1px solid rgba(168, 85, 247, 0.2); }

.exit-btn-circle {
  background: #ef4444; border: none; color: white; width: 28px; height: 28px;
  border-radius: 50%; cursor: pointer; display: flex; align-items: center; justify-content: center;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.4); transition: transform 0.2s;
}
.exit-btn-circle:hover { transform: scale(1.1) rotate(90deg); }

.chat-main-area { flex: 1; display: flex; flex-direction: column; min-height: 0; z-index: 5; }
.chat-scroll-boundary { flex: 1; min-height: 0; display: flex; flex-direction: column; }
.input-persistent-area { 
  padding: 10px 16px 16px 16px; /* 🗜️ Mais compacto para evitar cortes em janelas menores */
  background: linear-gradient(to top, #0f172a 85%, transparent); 
  z-index: 20; /* Garante que menus flutuantes fiquem visíveis */
}

.raw-terminal-view { flex: 1; display: flex; flex-direction: column; background: #000; min-height: 0; }
.terminal-overlay-header {
  padding: 6px 16px; background: #111; border-bottom: 1px solid #222;
  display: flex; justify-content: space-between; align-items: center;
}
.terminal-tabs { display: flex; gap: 4px; align-items: center; }
.terminal-tab {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 14px; border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.06);
  color: #64748b; font-size: 11px; font-weight: 800;
  text-transform: uppercase; cursor: pointer;
  transition: all 0.2s;
}
.terminal-tab.active { color: #f8fafc; border-color: rgba(255, 255, 255, 0.15); }
.terminal-tab.active.gemini { background: rgba(59, 130, 246, 0.15); border-color: rgba(59, 130, 246, 0.3); color: #60a5fa; }
.terminal-tab.active.claude { background: rgba(16, 185, 129, 0.15); border-color: rgba(16, 185, 129, 0.3); color: #34d399; }
.tab-dot { width: 5px; height: 5px; border-radius: 50%; background: currentColor; }
.terminal-stack { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.back-btn {
  background: #222; border: 1px solid #333; color: #888; padding: 4px 12px;
  border-radius: 4px; cursor: pointer; font-size: 11px;
}

/* --- Loading Overlay & Splah Screen --- */
.loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(15, 23, 42, 0.85);
  backdrop-filter: blur(12px);
  z-index: 100;
}

.loader-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  text-align: center;
}

.pulsing-icon {
  font-size: 3rem;
  animation: heart-pulse 2s infinite ease-in-out;
}

.loader-text h3 {
  font-size: 1.2rem;
  font-weight: 700;
  color: #fff;
  margin-bottom: 4px;
  letter-spacing: 0.5px;
}

.loader-text p {
  font-size: 0.9rem;
  color: #94a3b8;
}

.loader-bars {
  display: flex;
  gap: 4px;
  height: 20px;
  align-items: flex-end;
}

.loader-bars span {
  width: 3px;
  height: 100%;
  background: #3b82f6;
  border-radius: 2px;
  animation: bar-rise 1s infinite ease-in-out;
}

.loader-bars span:nth-child(2) { animation-delay: 0.1s; height: 70%; }
.loader-bars span:nth-child(3) { animation-delay: 0.2s; height: 90%; }
.loader-bars span:nth-child(4) { animation-delay: 0.3s; height: 60%; }
.loader-bars span:nth-child(5) { animation-delay: 0.4s; height: 80%; }

@keyframes heart-pulse {
  0%, 100% { transform: scale(1); filter: drop-shadow(0 0 10px rgba(59, 130, 246, 0.3)); }
  50% { transform: scale(1.1); filter: drop-shadow(0 0 20px rgba(59, 130, 246, 0.6)); }
}

@keyframes bar-rise {
  0%, 100% { height: 40%; }
  50% { height: 100%; }
}

/* Transições */
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.5s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
/* 📡 Activity Status Bar (Anti-Travamento) */
.activity-status-bar {
  position: absolute;
  bottom: 120px;
  left: 20px;
  right: 20px;
  padding: 10px 16px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  gap: 12px;
  z-index: 50;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
  pointer-events: none;
}

.activity-pulse {
  width: 6px;
  height: 6px;
  background: #3b82f6;
  border-radius: 50%;
  animation: activity-glow 1.2s infinite ease-in-out;
  box-shadow: 0 0 8px #3b82f6;
}

@keyframes activity-glow {
  0%, 100% { transform: scale(1); opacity: 0.5; }
  50% { transform: scale(1.5); opacity: 1; }
}

.activity-text {
  font-size: 11px;
  font-weight: 500;
  color: #94a3b8;
  letter-spacing: 0.5px;
}

.activity-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.activity-tool {
  font-size: 9px;
  font-weight: 900;
  color: #3b82f6;
  letter-spacing: 1px;
}

/* Perfis de Identidade Visual */
.active-agent-badge.doc-master { background: rgba(168, 85, 247, 0.15); color: #c084fc; border: 1px solid rgba(168, 85, 247, 0.2); }
.active-agent-badge.coder { background: rgba(16, 185, 129, 0.15); color: #34d399; border: 1px solid rgba(16, 185, 129, 0.2); }
.active-agent-badge.planner { background: rgba(59, 130, 246, 0.15); color: #60a5fa; border: 1px solid rgba(59, 130, 246, 0.2); }

.status-fade-enter-active, .status-fade-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.status-fade-enter-from, .status-fade-leave-to {
  opacity: 0;
  transform: translateY(10px);
}

/* 🪐 CENTRO: Ilha Flutuante de Órbita */
.workspace-island {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 6px 16px;
  border-radius: 100px;
  background: rgba(30, 41, 59, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.3);
  transition: all 0.3s;
}

.workspace-island:hover {
  background: rgba(30, 41, 59, 0.8);
  border-color: rgba(168, 85, 247, 0.4);
}

.ws-icon { cursor: pointer; font-size: 14px; transition: transform 0.2s; }
.ws-icon:hover { transform: scale(1.2); }

.ws-selector {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  position: relative;
}

.ws-name {
  font-size: 11px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.5px;
}

.ws-arrow { font-size: 8px; opacity: 0.5; }

.ws-tools {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 12px;
  border-left: 1px solid rgba(255,255,255,0.1);
}

.tool-btn {
  background: none; border: none; cursor: pointer; color: #fff; opacity: 0.6;
  font-size: 12px; transition: all 0.2s;
}

.tool-btn:hover { opacity: 1; transform: scale(1.2); }
.btn-clear { color: #ef4444; }

.workspace-selector {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  position: relative;
  padding: 2px 6px;
  border-radius: 6px;
  transition: background 0.2s;
}

.workspace-selector:hover {
  background: rgba(255, 255, 255, 0.05);
}

.dropdown-arrow {
  font-size: 8px;
  opacity: 0.5;
  transition: transform 0.3s;
}

.workspace-selector:hover .dropdown-arrow {
  opacity: 1;
  transform: translateY(1px);
}

/* 🛰️ Orbit Dropdown Styles */
.orbit-dropdown {
  position: absolute;
  top: calc(100% + 12px);
  left: 50%;
  transform: translateX(-50%);
  width: 280px;
  background: rgba(15, 23, 42, 0.95) !important;
  border: 1px solid rgba(168, 85, 247, 0.3) !important;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
  z-index: 1000;
  overflow: hidden;
  padding: 8px 0;
}

.dropdown-header {
  font-size: 9px;
  font-weight: 900;
  color: #a855f7;
  padding: 8px 16px;
  letter-spacing: 2px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  margin-bottom: 4px;
}

.orbit-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  cursor: pointer;
  transition: all 0.2s;
}

.orbit-item:hover {
  background: rgba(168, 85, 247, 0.1);
}

.orbit-item.is-active {
  background: rgba(168, 85, 247, 0.15);
  border-left: 3px solid #a855f7;
}

.item-icon {
  font-size: 16px;
}

.item-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.item-name {
  font-size: 12px;
  font-weight: 700;
  color: #fff;
}

.item-path {
  font-size: 9px;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dropdown-empty {
  padding: 20px;
  text-align: center;
  font-size: 11px;
  color: #64748b;
}

.dropdown-footer {
  margin-top: 4px;
  padding: 10px;
  text-align: center;
  font-size: 10px;
  font-weight: 800;
  color: #a855f7;
  cursor: pointer;
  background: rgba(168, 85, 247, 0.05);
  transition: background 0.2s;
}

.dropdown-footer:hover {
  background: rgba(168, 85, 247, 0.15);
}

/* Animação Slide Up */
.slide-up-enter-active, .slide-up-leave-active {
  transition: all 0.3s ease;
}
.slide-up-enter-from, .slide-up-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(10px);
}
</style>
