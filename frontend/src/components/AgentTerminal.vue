<script setup>
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const props = defineProps({
  isOpen: {
    type: Boolean,
    default: true
  }
})

const emit = defineEmits(['close'])

const orchestrator = useOrchestratorStore()
const { statusTimeline, statusFilter } = storeToRefs(orchestrator)

const filterOptions = [
  { label: 'TODOS', value: 'all' },
  { label: 'THINK', value: 'think' },
  { label: 'TOOL', value: 'tool' },
  { label: 'COMMAND', value: 'command' },
  { label: 'MEMORY', value: 'memory' },
  { label: 'ERROR', value: 'error' },
  { label: 'STATUS', value: 'status' },
]

const filteredTimeline = computed(() => {
  if (statusFilter.value === 'all') return statusTimeline.value || []
  return (statusTimeline.value || []).filter((item) => item.kind === statusFilter.value)
})
</script>

<template>
  <div v-show="isOpen" class="agent-terminal glass">
    <!-- Header/Tabs -->
    <div class="terminal-header">
      <div class="terminal-tabs-left">
        <div class="terminal-tab active">
          TERMINAL DE PROCESSAMENTO
        </div>
      </div>
      <div class="terminal-controls-right">
        <div class="activity-window-controls">
          <button
            v-for="opt in filterOptions"
            :key="opt.value"
            class="activity-filter-btn"
            :class="{ active: statusFilter === opt.value }"
            @click="statusFilter = opt.value"
          >
            {{ opt.label }}
          </button>
          <button class="activity-clear-btn" @click="orchestrator.clearStatusTimeline()">LIMPAR</button>
        </div>
        <button class="terminal-close-btn" @click="emit('close')" title="Fechar Terminal">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </button>
      </div>
    </div>

    <!-- Terminal Content (Like VSCode) -->
    <div class="terminal-body" ref="terminalBody">
      <div class="activity-window-list">
        <div v-if="filteredTimeline.length === 0" class="empty-state">
           Nenhuma atividade registrada no filtro atual.
        </div>
        <div v-for="item in filteredTimeline" :key="item.id" class="activity-window-item" :class="`kind-${item.kind || 'status'}`">
          <span class="activity-time">[{{ item.at }}]</span>
          <span class="activity-line">{{ item.text }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.agent-terminal {
  position: relative;
  display: flex;
  flex-direction: column;
  background: #090c10;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  height: 100%;
  width: 100%;
}

.terminal-header {
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 10px 0 0;
  background: rgba(13, 17, 23, 0.95);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.terminal-tabs-left {
  display: flex;
  height: 100%;
}

.terminal-tab {
  padding: 0 16px;
  display: flex;
  align-items: center;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.5px;
  color: #8b949e;
  border-bottom: 2px solid transparent;
  cursor: pointer;
}

.terminal-tab.active {
  color: #c9d1d9;
  border-bottom-color: #58a6ff;
}

.terminal-controls-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.activity-window-controls {
  display: flex;
  align-items: center;
  gap: 6px;
}

.activity-filter-btn,
.activity-clear-btn {
  border: 1px solid rgba(139, 148, 158, 0.2);
  background: rgba(33, 38, 45, 0.5);
  color: #8b949e;
  border-radius: 4px;
  padding: 2px 8px;
  font-size: 10px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.activity-filter-btn:hover {
  background: rgba(33, 38, 45, 0.8);
  color: #c9d1d9;
}

.activity-filter-btn.active {
  border-color: rgba(56, 189, 248, 0.45);
  color: #bae6fd;
  background: rgba(14, 116, 144, 0.35);
}

.activity-clear-btn {
  border-color: rgba(248, 81, 73, 0.3);
  color: #ff7b72;
}

.terminal-close-btn {
  background: transparent;
  border: none;
  color: #8b949e;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.terminal-close-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #c9d1d9;
}

.terminal-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, Consolas, Liberation Mono, monospace;
}

/* Custom Scrollbar for Terminal */
.terminal-body::-webkit-scrollbar {
  width: 10px;
}
.terminal-body::-webkit-scrollbar-track {
  background: transparent;
}
.terminal-body::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.1);
  border: 3px solid #090c10;
  border-radius: 6px;
}
.terminal-body::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.2);
}

.activity-window-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.empty-state {
  color: #484f58;
  font-size: 12px;
  padding: 10px;
  text-align: center;
}

.activity-window-item {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  font-size: 13px;
  line-height: 1.5;
}

.activity-time {
  color: #8b949e;
  flex-shrink: 0;
  user-select: none;
}

.activity-line {
  color: #c9d1d9;
  word-break: break-word;
}

.activity-window-item.kind-think .activity-line { color: #d2a8ff; }
.activity-window-item.kind-tool .activity-line { color: #79c0ff; }
.activity-window-item.kind-command .activity-line { color: #7ce38b; }
.activity-window-item.kind-memory .activity-line { color: #e3b341; }
.activity-window-item.kind-error .activity-line { color: #ffa198; }
</style>
