<script setup>
import { computed } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()
const subagentsList = computed(() => Array.from(orchestrator.subagents.entries()))

const getAgentColor = (name) => {
  const n = name.toLowerCase()
  if (n.includes('investigator')) return '#3b82f6' // Blue
  if (n.includes('help')) return '#10b981' // Green
  if (n.includes('coder')) return '#c084fc' // Purple
  return '#94a3b8'
}
</script>

<template>
  <Transition name="panel-slide">
    <div v-if="subagentsList.length > 0" class="subagent-monitor glass">
      <div class="monitor-header">
        <div class="monitor-title">
          <span class="swarm-icon">🐝</span>
          <h3>ENXAME ATIVO</h3>
        </div>
        <span class="subagent-count">{{ subagentsList.length }}</span>
      </div>

      <div class="subagent-stack">
        <div 
          v-for="[id, data] in subagentsList" 
          :key="id" 
          class="subagent-card glass-light"
        >
          <div class="card-header">
            <div class="agent-avatar" :style="{ backgroundColor: getAgentColor(data.agentName) }">
              {{ data.agentName[0].toUpperCase() }}
            </div>
            <div class="agent-info">
              <h4>{{ data.agentName.toUpperCase() }}</h4>
              <p class="goal-text">{{ data.goal }}</p>
            </div>
          </div>
          
          <div class="card-status">
            <div class="pulse-container">
              <div class="status-pulse" :class="data.kind"></div>
            </div>
            <span class="status-text">{{ data.status }}</span>
          </div>

          <!-- Barra de Progresso Indeterminada (Atividade) -->
          <div class="activity-bar">
            <div class="bar-fill"></div>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.subagent-monitor {
  position: absolute;
  top: 80px;
  right: 20px;
  width: 280px;
  max-height: calc(100% - 160px);
  background: rgba(15, 23, 42, 0.7);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  z-index: 100;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
  overflow: hidden;
}

.monitor-header {
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.monitor-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.swarm-icon { font-size: 1.2rem; }

.monitor-title h3 {
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 1.5px;
  color: #94a3b8;
  margin: 0;
}

.subagent-count {
  font-size: 10px;
  font-weight: 800;
  background: #3b82f6;
  color: white;
  padding: 2px 8px;
  border-radius: 100px;
}

.subagent-stack {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  overflow-y: auto;
}

.subagent-card {
  padding: 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.subagent-card:hover {
  background: rgba(255, 255, 255, 0.05);
  transform: translateX(-4px);
}

.card-header {
  display: flex;
  gap: 12px;
  margin-bottom: 10px;
}

.agent-avatar {
  width: 32px;
  height: 32px;
  min-width: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 900;
  font-size: 14px;
  color: white;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.3);
}

.agent-info h4 {
  font-size: 10px;
  font-weight: 800;
  color: #f8fafc;
  margin: 0 0 2px 0;
}

.goal-text {
  font-size: 10px;
  color: #64748b;
  margin: 0;
  line-height: 1.2;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.card-status {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.2);
  padding: 6px 10px;
  border-radius: 6px;
  margin-bottom: 8px;
}

.pulse-container {
  width: 6px;
  height: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-pulse {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #3b82f6;
  box-shadow: 0 0 8px #3b82f6;
  animation: pulse 1.5s infinite;
}

.status-pulse.error { background: #ef4444; box-shadow: 0 0 8px #ef4444; }
.status-pulse.warning { background: #fbbf24; box-shadow: 0 0 8px #fbbf24; }
.status-pulse.tool { background: #c084fc; box-shadow: 0 0 8px #c084fc; }

.status-text {
  font-size: 10px;
  font-weight: 600;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.activity-bar {
  height: 2px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 100px;
  overflow: hidden;
}

.bar-fill {
  height: 100%;
  width: 30%;
  background: #3b82f6;
  border-radius: 100px;
  animation: slide-indeterminade 2s infinite ease-in-out;
}

@keyframes pulse {
  0% { transform: scale(0.95); opacity: 0.5; }
  50% { transform: scale(1.2); opacity: 1; }
  100% { transform: scale(0.95); opacity: 0.5; }
}

@keyframes slide-indeterminade {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(330%); }
}

/* Transições */
.panel-slide-enter-active, .panel-slide-leave-active {
  transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
}
.panel-slide-enter-from, .panel-slide-leave-to {
  opacity: 0;
  transform: translateX(50px) scale(0.95);
}
</style>
