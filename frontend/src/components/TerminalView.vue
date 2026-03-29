<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { SendTerminalData, ResizeTerminal } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  agent: { type: String, required: true },
  active: { type: Boolean, default: false }
})

const emit = defineEmits(['session-ended'])

const terminalContainer = ref(null)
let terminal = null
let fitAddon = null
let resizeObserver = null

// Tema Dark Premium alinhado com Lumaestro
const lumaestroTheme = {
  background: '#0a0e1a',
  foreground: '#e2e8f0',
  cursor: '#3b82f6',
  cursorAccent: '#0a0e1a',
  selectionBackground: 'rgba(59, 130, 246, 0.3)',
  selectionForeground: '#ffffff',
  black: '#1e293b',
  red: '#ef4444',
  green: '#22c55e',
  yellow: '#f59e0b',
  blue: '#3b82f6',
  magenta: '#a855f7',
  cyan: '#06b6d4',
  white: '#f1f5f9',
  brightBlack: '#475569',
  brightRed: '#f87171',
  brightGreen: '#4ade80',
  brightYellow: '#fbbf24',
  brightBlue: '#60a5fa',
  brightMagenta: '#c084fc',
  brightCyan: '#22d3ee',
  brightWhite: '#ffffff'
}

const initTerminal = () => {
  if (!terminalContainer.value || terminal) return

  terminal = new Terminal({
    theme: lumaestroTheme,
    fontFamily: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace",
    fontSize: 14,
    lineHeight: 1.4,
    cursorBlink: true,
    cursorStyle: 'bar',
    cursorWidth: 2,
    scrollback: 10000,
    allowProposedApi: true,
    convertEol: true,
    windowsMode: true
  })

  fitAddon = new FitAddon()
  terminal.loadAddon(fitAddon)
  terminal.open(terminalContainer.value)

  nextTick(() => {
    try {
      fitAddon.fit()
      ResizeTerminal(props.agent, terminal.cols, terminal.rows)
    } catch (e) { /* Ignora */ }
  })

  // Envia teclas para o agente CORRETO
  terminal.onData((data) => {
    const bytes = new TextEncoder().encode(data)
    const binary = Array.from(bytes, b => String.fromCharCode(b)).join('')
    SendTerminalData(props.agent, btoa(binary))
  })

  // Filtra output — só escreve bytes que pertençam a ESTE agente
  EventsOn('terminal:output', (payload) => {
    if (terminal && payload?.agent === props.agent) {
      const binary = atob(payload.data)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i++) {
        bytes[i] = binary.charCodeAt(i)
      }
      terminal.write(bytes)
    }
  })

  // Escuta encerramento apenas DESTE agente
  EventsOn('terminal:closed', (closedAgent) => {
    if (closedAgent === props.agent) {
      if (terminal) {
        terminal.write('\r\n\x1b[1;33m── Sessão encerrada ──\x1b[0m\r\n')
      }
      emit('session-ended', props.agent)
    }
  })

  // Auto-fit
  resizeObserver = new ResizeObserver(() => {
    try {
      if (fitAddon && terminal) {
        fitAddon.fit()
        ResizeTerminal(props.agent, terminal.cols, terminal.rows)
      }
    } catch (e) { /* Ignora */ }
  })
  resizeObserver.observe(terminalContainer.value)

  terminal.focus()
}

onMounted(() => {
  if (props.active) nextTick(() => initTerminal())
})

watch(() => props.active, (isActive) => {
  if (isActive && !terminal) nextTick(() => initTerminal())
  if (isActive && terminal) terminal.focus()
})

onUnmounted(() => {
  EventsOff('terminal:output')
  EventsOff('terminal:closed')
  if (resizeObserver) { resizeObserver.disconnect(); resizeObserver = null }
  if (terminal) { terminal.dispose(); terminal = null }
})
</script>

<template>
  <div class="terminal-view-container" v-show="active">
    <div ref="terminalContainer" class="terminal-viewport"></div>
  </div>
</template>

<style scoped>
.terminal-view-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  background: #0a0e1a;
  min-height: 0;
  animation: terminalAppear 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes terminalAppear {
  from { opacity: 0; transform: scale(0.99); }
  to { opacity: 1; transform: scale(1); }
}

.terminal-viewport {
  flex: 1;
  padding: 8px;
  min-height: 0;
}

.terminal-viewport :deep(.xterm) { padding: 4px; }
.terminal-viewport :deep(.xterm-viewport::-webkit-scrollbar) { width: 8px; }
.terminal-viewport :deep(.xterm-viewport::-webkit-scrollbar-track) { background: transparent; }
.terminal-viewport :deep(.xterm-viewport::-webkit-scrollbar-thumb) {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 10px;
  border: 2px solid #0a0e1a;
}
.terminal-viewport :deep(.xterm-viewport::-webkit-scrollbar-thumb:hover) {
  background: rgba(255, 255, 255, 0.1);
}
</style>
