<script setup>
import { ref, watch, nextTick, onMounted, onUnmounted } from 'vue'
import { AskAgent, ScanVault, StartAgentSession, SendAgentInput, StopAgentSession, SendTerminalData } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import TerminalView from './TerminalView.vue'
import ChatLog from './ChatLog.vue'
import ChatInput from './ChatInput.vue'

const props = defineProps({
  logs: { type: Array, default: () => [] }
})

const input = ref('')
const logContainer = ref(null)
const isTerminalMode = ref(false)
const isRealPTY = ref(false)
const showRawTerminal = ref(false)
const selectedAgent = ref('gemini')
const activeAgent = ref(null)
const messages = ref([])
const isThinking = ref(false)

// Buffer para acumular output do terminal e evitar spam de bolhas
let outputBuffer = ""
let lastAssistantMsg = null

// Escuta o evento terminal:started para saber se é ConPTY real
EventsOn('terminal:started', (info) => {
  if (info && info.isRealPTY !== undefined) {
    isRealPTY.value = info.isRealPTY
  }
})

// Escuta o output do terminal para transformar em Bolhas de Chat
const handleTerminalOutput = (dataBase64) => {
  if (!isTerminalMode.value) return
  
  const text = window.atob(dataBase64)
  console.log("📥 PTY Output Raw:", text) // Debug: Ver no F12 do Wails
  
  // Limpeza de ANSI (Regex melhorado)
  const cleanText = text.replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '')
  console.log("✨ PTY Output Clean:", cleanText)
  
  if (!cleanText.trim()) return

  isThinking.value = false

  // Escuta logs do sistema e fim de sessão
  const systemLogUnsub = EventsOn('execution:log', (log) => {
    if (log.source === 'SYSTEM') {
      console.warn("[Maestro] Log de Sistema:", log.content);
      // Se a sessão foi encerrada e ainda estávamos no "Iniciando...", mostramos o erro
      if ((log.content.includes("Sessão") && log.content.includes("encerrada")) || log.content.includes("❌")) {
        if (messages.value.length > 0 && messages.value[messages.value.length - 1].isStreaming) {
            messages.value[messages.value.length - 1].content = `⚠️ Erro na inicialização: ${log.content}. Verifique os logs no terminal ou tente configurar o login novamente.`
            messages.value[messages.value.length - 1].isStreaming = false
        }
      }
    }
  });

  if (!lastAssistantMsg || messages.value[messages.value.length - 1] !== lastAssistantMsg) {
    lastAssistantMsg = {
      role: 'assistant',
      text: cleanText,
      agent: activeAgent.value,
      mode: 'act'
    }
    messages.value.push(lastAssistantMsg)
  } else {
    lastAssistantMsg.text += cleanText
  }
}

onMounted(() => {
  EventsOn('terminal:output', handleTerminalOutput)
})

onUnmounted(() => {
  EventsOff('terminal:output')
})

// Auto-scroll para a última mensagem
watch(() => props.logs.length, async () => {
  await nextTick()
  if (logContainer.value) {
    logContainer.value.scrollTo({
      top: logContainer.value.scrollHeight,
      behavior: 'smooth'
    })
  }
})

const sendChatMessage = async (payload) => {
  const { text, agent, mode } = payload
  
  // Adiciona msg do usuário ao log
  messages.value.push({
    role: 'user',
    text: text
  })

  if (!isTerminalMode.value || activeAgent.value !== agent) {
    // Se o agente mudou ou não iniciou, inicia nova sessão
    messages.value.push({ role: 'assistant', text: `Iniciando sessão com ${agent.toUpperCase()}...`, mode: 'system' })
    await startCLISession(agent)
  }

  isThinking.value = true
  // Envia para o PTY
  SendTerminalData(window.btoa(text + '\r'))
}

const copyTerminalOutput = () => {
  // Isso seria implementado pegando a seleção do xterm ou emitindo um evento
  // Por enquanto, mostra um aviso
  window.runtime.EventsEmit('ui:notify', { message: 'Output copiado!', type: 'success' })
}

const runScan = async () => {
  await ScanVault()
}

const getSourceClass = (source) => {
  const s = source.toLowerCase()
  if (s.includes('maestro')) return 'maestro-msg'
  if (s.includes('crawler')) return 'crawler-msg'
  if (s.includes('claude')) return 'agent-msg-claude'
  if (s.includes('gemini')) return 'agent-msg-gemini'
  if (s.includes('você')) return 'user-msg'
  if (s.includes('system')) return 'system-msg'
  return 'default-msg'
}

const getSourceIcon = (source) => {
  const s = source.toLowerCase()
  if (s.includes('maestro')) return '🧠'
  if (s.includes('crawler')) return '🕷️'
  if (s.includes('claude')) return '🕊️'
  if (s.includes('gemini')) return '♊'
  if (s.includes('system')) return '⚙️'
  return '🤖'
}
const startCLISession = async (agent) => {
  activeAgent.value = agent
  isTerminalMode.value = true
  
  // Chama o backend Go
  const result = await StartAgentSession(agent)
  console.log("[Maestro] Sessão iniciada:", result)
}

const exitTerminal = async () => {
  await StopAgentSession()
  isTerminalMode.value = false
  isRealPTY.value = false
  isThinking.value = false
  activeAgent.value = null
  
  messages.value.push({
    role: 'assistant',
    text: 'Sessão encerrada pelo usuário.',
    mode: 'system'
  })
}

const onSessionEnded = () => {
  isTerminalMode.value = false
  isRealPTY.value = false
  isThinking.value = false
  activeAgent.value = null
}
</script>

<template>
  <main :class="['chat-container animate-fade-up', { 'terminal-active': isTerminalMode }]">
    <!-- Header Premium -->
    <header class="chat-header glass">
      <div class="header-top-row">
        <div class="header-info">
          <div :class="['pulse-indicator', { 'pulse-terminal': isTerminalMode }]"></div>
          <div class="header-titles">
            <h2>{{ isTerminalMode ? 'Orquestra Interativa' : 'Maestro Console' }}</h2>
            <span v-if="isTerminalMode" class="terminal-badge">
              {{ isRealPTY ? '🟢 CONPTY REAL' : '🟡 ONE-SHOT' }}: {{ activeAgent?.toUpperCase() }}
            </span>
          </div>
        </div>

        <div class="header-right-actions">
           <!-- Botão de Debug sempre visível durante o desenvolvimento -->
           <button @click="showRawTerminal = !showRawTerminal" class="btn-debug-top">
            {{ showRawTerminal ? '🔙 VOLTAR PRO CHAT' : '⚙️ VER TERMINAL (DEBUG)' }}
           </button>

           <button v-if="!isTerminalMode" @click="runScan" class="btn-scan">
             <span class="icon">📦</span>
             <span>SCAN VAULT</span>
           </button>
           <button v-if="isTerminalMode" @click="exitTerminal" class="btn-exit">
             <span class="icon">⏹️</span> SAIR
           </button>
        </div>
      </div>
      
    </header>

    <!-- Área Principal do Chat (Premium) -->
    <div v-if="!showRawTerminal" class="chat-main-area">
      <ChatLog :messages="messages" :is-thinking="isThinking" />
      <ChatInput @send="sendChatMessage" />
    </div>

    <!-- Terminal Real (Visível apenas em modo Debug ou quando solicitado) -->
    <div v-show="showRawTerminal" class="terminal-wrapper-debug">
      <div class="debug-overlay-header">
        <span>MODO TERMINAL (DEBUG)</span>
        <button @click="showRawTerminal = false">VOLTAR PARA CHAT</button>
      </div>
      <TerminalView
        :agent="activeAgent"
        :active="isTerminalMode"
        @session-ended="onSessionEnded"
      />
    </div>
  </main>
</template>

<style scoped>
.chat-container {
  height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 1rem 1.5rem;
  background: radial-gradient(circle at bottom right, rgba(59, 130, 246, 0.05) 0%, transparent 60%);
  gap: 1.25rem;
}

/* Header Styling */
.chat-header {
  display: flex;
  flex-direction: column;
  padding: 1rem 1.5rem;
  border-radius: 12px;
  backdrop-filter: blur(20px);
  background: rgba(15, 23, 42, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.08);
  gap: 16px;
  width: 100%;
  flex-shrink: 0;
}

.header-top-row {
  display: flex;
  justify-content: center;
  gap: 16px;
  align-items: center;
  width: 100%;
  flex-wrap: wrap;
}

.header-info {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.header-info h2 {
  font-size: 0.8rem;
  text-transform: uppercase;
  letter-spacing: 1.5px;
  font-weight: 800;
  color: var(--primary);
  margin: 0;
  white-space: nowrap;
}

.pulse-indicator {
  width: 8px;
  height: 8px;
  background: var(--primary);
  border-radius: 50%;
  box-shadow: 0 0 10px var(--primary);
  animation: pulse 2s infinite;
}

.pulse-terminal {
  background: #f59e0b;
  box-shadow: 0 0 10px #f59e0b;
}

.header-titles {
  display: flex;
  flex-direction: column;
}

.terminal-badge {
  font-size: 0.6rem;
  font-weight: 900;
  color: #f59e0b;
  letter-spacing: 1px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  justify-content: center;
}

.btn-tool {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  color: #94a3b8;
  padding: 5px 12px;
  border-radius: 8px;
  font-size: 0.65rem;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.3s;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.btn-tool .icon { font-size: 0.8rem; }

.btn-claude:hover { color: #d97706; border-color: #d97706; background: rgba(217, 119, 6, 0.05); }
.btn-gemini:hover { color: #3b82f6; border-color: #3b82f6; background: rgba(59, 130, 246, 0.05); }

/* Terminal Layout & Toolbar */
.terminal-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: rgba(13, 17, 23, 0.95);
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
}

.terminal-toolbar {
  height: 40px;
  background: rgba(30, 41, 59, 0.6);
  backdrop-filter: blur(10px);
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.active-agent-badge {
  font-size: 0.65rem;
  font-weight: 800;
  color: #94a3b8;
  display: flex;
  align-items: center;
  gap: 6px;
  letter-spacing: 0.5px;
}

.active-agent-badge .icon {
  font-size: 1rem;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-action {
  background: transparent;
  border: 1px solid transparent;
  color: #cbd5e1;
  padding: 4px 8px;
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 4px;
  transition: all 0.2s;
}

.btn-action:hover {
  background: rgba(255, 255, 255, 0.05);
  color: white;
  border-color: rgba(255, 255, 255, 0.1);
}

.btn-action .icon {
  font-size: 0.8rem;
}

.divider {
  width: 1px;
  height: 16px;
  background: rgba(255, 255, 255, 0.1);
  margin: 0 4px;
}

.btn-exit {
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  color: #ef4444;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 800;
  cursor: pointer;
}

.btn-exit:hover { background: #ef4444; color: white; }

.btn-scan {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.1), rgba(37, 99, 235, 0.1));
  border: 1px solid rgba(59, 130, 246, 0.2);
  color: #f8fafc;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 0.55rem;
  font-weight: 800;
  letter-spacing: 0.5px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  transition: all 0.3s;
  flex-shrink: 0;
  white-space: nowrap;
  width: max-content;
}

.btn-scan:hover {
  background: rgba(59, 130, 246, 0.1);
  border-color: var(--primary);
  transform: translateY(-1px);
}

.header-right-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.btn-debug-top {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #f59e0b;
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 10px;
  font-weight: 800;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-debug-top:hover {
  background: #f59e0b;
  color: #000;
}

.debug-overlay-header {
  padding: 10px 20px;
  background: #000;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid #222;
}

.debug-overlay-header span { color: #f59e0b; font-size: 12px; font-weight: 900; }
.debug-overlay-header button { 
  background: #f59e0b; border: none; color: #000; padding: 4px 12px; border-radius: 4px; font-size: 10px; font-weight: bold; cursor: pointer;
}

.terminal-wrapper-debug {
  flex: 1;
  background: #000;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid #333;
  display: flex;
  flex-direction: column;
}

/* Messages Area */
.messages-area {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  padding-right: 1.5rem;
  scroll-behavior: smooth;
  min-height: 0;
}

.message-card {
  padding: 1.5rem;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.02);
  border-left: 4px solid var(--border-color);
  backdrop-filter: blur(4px);
  transition: transform 0.3s;
  animation: slideIn 0.4s ease-out;
}

.message-card:hover {
  transform: translateX(4px);
  background: rgba(255, 255, 255, 0.03);
}

.maestro-msg { border-left-color: var(--primary); }
.crawler-msg { border-left-color: var(--success); }
.agent-msg-claude { border-left-color: #d97706; background: rgba(217, 119, 6, 0.02) !important; }
.agent-msg-gemini { border-left-color: #3b82f6; background: rgba(59, 130, 246, 0.02) !important; }
.system-msg { border-left-color: #64748b; font-style: italic; opacity: 0.8; }
.user-msg { 
  border-left-color: #f8fafc; 
  background: rgba(255, 255, 255, 0.05) !important;
  margin-left: 2rem;
}

.message-meta {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.source-badge {
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: var(--text-dim);
  display: flex;
  align-items: center;
  gap: 6px;
}

.time {
  font-size: 0.7rem;
  color: #475569;
  font-family: 'Fira Code', monospace;
}

.message-content {
  color: #e2e8f0;
  line-height: 1.6;
  font-size: 1rem;
  white-space: pre-wrap;
  font-family: 'Inter', sans-serif;
}

.empty-state {
  margin: auto;
  text-align: center;
  opacity: 0.5;
}

.empty-icon { font-size: 3rem; margin-bottom: 1rem; }

/* Input Section */
.input-section {
  padding-bottom: 1rem;
  flex-shrink: 0;
}

.input-glass-wrapper {
  position: relative;
  background: rgba(2, 6, 23, 0.8);
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  padding: 6px;
  display: flex;
  align-items: center;
}

.ghost-input {
  width: 100%;
  background: transparent;
  border: none;
  padding: 16px 20px;
  color: white;
  font-size: 1rem;
  outline: none;
  font-family: inherit;
}

.glow-border {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  border-radius: 20px;
  pointer-events: none;
  border: 2px solid transparent;
  transition: all 0.4s;
}

.ghost-input:focus + .input-actions + .glow-border,
.input-glass-wrapper:focus-within .glow-border {
  border-color: var(--primary-glow);
  box-shadow: 0 0 25px rgba(59, 130, 246, 0.15);
}

.shortcut-hint {
  padding: 4px 10px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  font-size: 0.65rem;
  font-weight: 800;
  color: var(--text-dim);
  margin-right: 15px;
  white-space: nowrap;
  flex-shrink: 0;
  display: inline-block;
}

/* Animations */
@keyframes slideIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.4); opacity: 0.4; }
  100% { transform: scale(1); opacity: 1; }
}

/* Scrollbar */
.messages-area::-webkit-scrollbar { width: 5px; }
.messages-area::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 10px;
}
</style>
