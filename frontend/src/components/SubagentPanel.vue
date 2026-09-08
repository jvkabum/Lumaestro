<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()
const activeTab = ref('active') // 'active' | 'custom'

const subagentsList = computed(() => Array.from(orchestrator.subagents.entries()))
const customAgentsList = computed(() => orchestrator.customAgents || [])

const isVisible = computed(() => {
  return subagentsList.value.length > 0 || orchestrator.isAgentsPanelOpen
})

// Quando abrir o painel explicitamente, atualiza a lista de agentes customizados
watch(() => orchestrator.isAgentsPanelOpen, (open) => {
  if (open) {
    orchestrator.fetchCustomAgents()
  }
})

onMounted(() => {
  if (orchestrator.isAgentsPanelOpen) {
    orchestrator.fetchCustomAgents()
  }
})

const getAgentColor = (name) => {
  const n = (name || '').toLowerCase()
  if (n.includes('investigator')) return '#3b82f6' // Blue
  if (n.includes('help')) return '#10b981' // Green
  if (n.includes('coder') || n.includes('dev')) return '#c084fc' // Purple
  if (n.includes('reviewer') || n.includes('audit')) return '#f59e0b' // Amber
  return '#94a3b8'
}

const killAgent = async (sessionId) => {
  try {
    await orchestrator.killSubagent(sessionId)
  } catch (err) {
    console.error('[SubagentPanel] Erro ao encerrar subagente:', err)
  }
}

const closePanel = () => {
  orchestrator.toggleAgentsPanel(false)
}

const selectCustomAgent = (agent) => {
  orchestrator.pushStatus(`Agente customizado selecionado: ${agent.name}`, 'status')
  // Se houver instruções, insere no chat ou envia
  window.dispatchEvent(new CustomEvent('insert:prompt', { 
    detail: `Use o agente @${agent.name} para a tarefa:\n` 
  }))
  closePanel()
}
</script>

<template>
  <Transition name="panel-slide">
    <div v-if="isVisible" class="subagent-monitor glass-heavy">
      <!-- Header com Navegação e Fechar -->
      <div class="monitor-header">
        <div class="header-top">
          <div class="monitor-title">
            <span class="swarm-icon">🐝</span>
            <h3>AGENT MANAGER (/agents)</h3>
          </div>
          <button class="close-btn" @click="closePanel" title="Fechar painel">✕</button>
        </div>

        <!-- Abas de Navegação -->
        <div class="panel-tabs">
          <button 
            class="tab-btn" 
            :class="{ active: activeTab === 'active' }"
            @click="activeTab = 'active'"
          >
            Subagentes
            <span class="tab-badge" v-if="subagentsList.length">{{ subagentsList.length }}</span>
          </button>
          <button 
            class="tab-btn" 
            :class="{ active: activeTab === 'custom' }"
            @click="activeTab = 'custom'"
          >
            Customizados
            <span class="tab-badge secondary" v-if="customAgentsList.length">{{ customAgentsList.length }}</span>
          </button>
        </div>
      </div>

      <!-- Conteúdo da Aba 1: Subagentes Ativos -->
      <div v-if="activeTab === 'active'" class="tab-content">
        <div v-if="subagentsList.length === 0" class="empty-subagents">
          <span>💤</span>
          <p>Nenhum subagente em execução no momento.</p>
          <small>Use <code>/boost</code> ou <code>/teamwork-preview</code> para orquestrar enxames.</small>
        </div>

        <div v-else class="subagent-stack">
          <div 
            v-for="[id, data] in subagentsList" 
            :key="id" 
            class="subagent-card glass-light"
          >
            <div class="card-header">
              <div class="agent-avatar" :style="{ backgroundColor: getAgentColor(data.agentName) }">
                {{ (data.agentName || 'A')[0].toUpperCase() }}
              </div>
              <div class="agent-info">
                <h4>{{ (data.agentName || 'SUBAGENTE').toUpperCase() }}</h4>
                <p class="goal-text">{{ data.goal || 'Executando sub-tarefa delegada...' }}</p>
              </div>
              <button 
                class="kill-btn" 
                @click="killAgent(id)" 
                title="Encerrar subagente imediatamente (K)"
              >
                🛑 Parar
              </button>
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

      <!-- Conteúdo da Aba 2: Agentes Customizados -->
      <div v-if="activeTab === 'custom'" class="tab-content">
        <div class="custom-agents-header">
          <span>Agentes Registrados (.agents/)</span>
          <button class="btn-refresh" @click="orchestrator.fetchCustomAgents()" title="Recarregar agentes">
            🔄
          </button>
        </div>

        <div v-if="customAgentsList.length === 0" class="empty-subagents">
          <span>📁</span>
          <p>Nenhum agente customizado encontrado.</p>
          <small>Crie <code>.agents/agents/&lt;nome&gt;/agent.md</code> no workspace ou em <code>~/.gemini/config/agents/</code>.</small>
        </div>

        <div v-else class="custom-agents-list">
          <div v-for="agent in customAgentsList" :key="agent.name" class="custom-agent-card">
            <div class="custom-card-header">
              <div class="custom-agent-avatar" :style="{ backgroundColor: getAgentColor(agent.name) }">
                {{ agent.name[0].toUpperCase() }}
              </div>
              <div class="custom-agent-meta">
                <div class="meta-row">
                  <span class="custom-name">{{ agent.name }}</span>
                  <span class="source-badge" :class="agent.source">{{ agent.source }}</span>
                </div>
                <p class="custom-desc">{{ agent.description || 'Sem descrição declarada.' }}</p>
              </div>
            </div>
            <div class="custom-card-actions">
              <button class="btn-use-agent" @click="selectCustomAgent(agent)">
                Usar Agente
              </button>
            </div>
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
  width: 340px;
  max-height: calc(100% - 160px);
  background: rgba(15, 23, 42, 0.9);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  z-index: 100;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}

.monitor-header {
  padding: 12px 16px 8px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.monitor-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.swarm-icon { font-size: 1.2rem; }

.monitor-title h3 {
  font-size: 11px;
  font-weight: 900;
  letter-spacing: 1.5px;
  color: #f1f5f9;
  margin: 0;
}

.close-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 14px;
  cursor: pointer;
  padding: 4px 6px;
  border-radius: 6px;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: white;
}

.panel-tabs {
  display: flex;
  gap: 6px;
  background: rgba(0, 0, 0, 0.3);
  padding: 3px;
  border-radius: 8px;
}

.tab-btn {
  flex: 1;
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 700;
  padding: 6px;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition: all 0.2s;
}

.tab-btn.active {
  background: rgba(255, 255, 255, 0.1);
  color: #f8fafc;
}

.tab-badge {
  font-size: 9px;
  background: #3b82f6;
  color: white;
  padding: 1px 6px;
  border-radius: 10px;
}

.tab-badge.secondary {
  background: rgba(255, 255, 255, 0.15);
  color: #cbd5e1;
}

.tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 12px;
  display: flex;
  flex-direction: column;
}

.empty-subagents {
  padding: 40px 16px;
  text-align: center;
  color: #64748b;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.empty-subagents span { font-size: 2rem; }
.empty-subagents p { margin: 0; font-size: 12px; color: #94a3b8; }
.empty-subagents small { font-size: 10px; color: #475569; }

.subagent-stack, .custom-agents-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.subagent-card {
  padding: 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  transition: all 0.2s;
}

.subagent-card:hover {
  background: rgba(255, 255, 255, 0.04);
}

.card-header {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  margin-bottom: 10px;
}

.agent-avatar, .custom-agent-avatar {
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

.agent-info {
  flex: 1;
  overflow: hidden;
}

.agent-info h4 {
  font-size: 11px;
  font-weight: 800;
  color: #f8fafc;
  margin: 0 0 2px 0;
}

.goal-text {
  font-size: 10px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.kill-btn {
  background: rgba(239, 68, 68, 0.2);
  border: 1px solid rgba(239, 68, 68, 0.4);
  color: #fca5a5;
  font-size: 10px;
  font-weight: 700;
  padding: 4px 8px;
  border-radius: 6px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.2s;
}

.kill-btn:hover {
  background: rgba(239, 68, 68, 0.4);
  color: white;
}

.card-status {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.3);
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

/* Custom Agents styles */
.custom-agents-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #64748b;
  margin-bottom: 8px;
}

.btn-refresh {
  background: transparent;
  border: none;
  cursor: pointer;
  font-size: 12px;
  opacity: 0.7;
}

.btn-refresh:hover { opacity: 1; }

.custom-agent-card {
  padding: 12px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.custom-card-header {
  display: flex;
  gap: 10px;
}

.custom-agent-meta {
  flex: 1;
}

.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 2px;
}

.custom-name {
  font-size: 12px;
  font-weight: 700;
  color: #f1f5f9;
}

.source-badge {
  font-size: 9px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
  text-transform: uppercase;
}

.source-badge.workspace {
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
}

.source-badge.global {
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
}

.custom-desc {
  font-size: 10px;
  color: #94a3b8;
  margin: 0;
  line-height: 1.3;
}

.custom-card-actions {
  display: flex;
  justify-content: flex-end;
}

.btn-use-agent {
  background: rgba(59, 130, 246, 0.2);
  border: 1px solid rgba(59, 130, 246, 0.3);
  color: #93c5fd;
  font-size: 10px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-use-agent:hover {
  background: #2563eb;
  color: white;
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

.panel-slide-enter-active, .panel-slide-leave-active {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.panel-slide-enter-from, .panel-slide-leave-to {
  opacity: 0;
  transform: translateX(50px) scale(0.95);
}
</style>
