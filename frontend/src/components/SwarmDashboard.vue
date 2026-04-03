<script setup>
import { onMounted, ref } from 'vue'
import { GetExecutiveSummary, GetPendingApprovals, ApproveAction, RejectAction, GetLightningStats, GetLatestSpans, ExportTelemetry, GetPromptCandidates, ApprovePromptVariant, ExportRLHFDataset } from '../../wailsjs/go/main/App'

const summary = ref({
  total_spent_cents: 0,
  active_agents: 0,
  paused_agents: 0,
  open_issues: 0,
  done_issues: 0,
  pending_approvals: 0
})

const lightning = ref({
  total_rollouts: 0,
  avg_reward: 0,
  total_cost_usd: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  status: 'offline'
})

const pendingApprovals = ref([])
const pendingCandidates = ref([])
const liveSpans = ref([])
const isLoading = ref(true)

const loadDashboard = async () => {
  isLoading.value = true
  try {
    const s = await GetExecutiveSummary()
    if (s) summary.value = s
    
    const approvals = await GetPendingApprovals()
    pendingApprovals.value = approvals || []

    const lStats = await GetLightningStats()
    if (lStats) {
      lightning.value = {
        total_rollouts: lStats.total_rollouts || 0,
        avg_reward: lStats.avg_reward || 0,
        total_cost_usd: lStats.total_cost_usd || 0,
        prompt_tokens: lStats.prompt_tokens || 0,
        completion_tokens: lStats.completion_tokens || 0,
        status: lStats.status || 'offline'
      }
    }

    const spans = await GetLatestSpans()
    liveSpans.value = spans || []

    const cands = await GetPromptCandidates()
    pendingCandidates.value = cands || []
  } catch (e) {
    console.error("Erro ao carregar Dashboard:", e)
  } finally {
    isLoading.value = false
  }
}

const handleApprove = async (id) => {
  await ApproveAction(id, "Aprovado via Dashboard Executivo")
  await loadDashboard()
}

const handleReject = async (id) => {
  await RejectAction(id, "Rejeitado via Dashboard Executivo")
  await loadDashboard()
}

const handleExport = async () => {
  const result = await ExportTelemetry()
  alert(result)
}

const handleExportRLHF = async () => {
  const result = await ExportRLHFDataset()
  alert(result)
}

const handleApproveVariant = async (id) => {
  const result = await ApprovePromptVariant(id)
  alert(result)
  await loadDashboard()
}

onMounted(loadDashboard)

const formatCurrency = (cents) => {
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(cents / 100)
}

const getAccuracyClass = (acc) => {
  if (acc >= 90) return 'acc-high'
  if (acc >= 60) return 'acc-med'
  return 'acc-low'
}
</script>

<template>
  <div class="swarm-dashboard glass">
    <header class="dashboard-header">
      <div class="title-group">
        <h1>Dashboard Executivo</h1>
        <p>Visão em tempo real da Autonomia do Enxame</p>
      </div>
      <div class="header-actions">
        <button @click="handleExportRLHF" class="rlhf-btn" title="Gerar Dataset para Fine-tuning (JSONL)">
          📦 Gerar Dataset RLHF
        </button>
        <button @click="handleExport" class="export-btn" title="Exportar Telemetria Elite (JSON)">
          📥 Exportar Traces
        </button>
        <button @click="loadDashboard" class="refresh-btn" :class="{ rotating: isLoading }">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
          </svg>
        </button>
      </div>
    </header>

    <div class="stats-grid">
      <div class="stat-card glass-card blue">
        <div class="card-icon">💰</div>
        <div class="card-data">
          <span class="label">Investimento (Tokens)</span>
          <h3 class="value">{{ formatCurrency(summary.total_spent_cents) }}</h3>
        </div>
      </div>
      <div class="stat-card glass-card green">
        <div class="card-icon">🤖</div>
        <div class="card-data">
          <span class="label">Agentes Ativos</span>
          <h3 class="value">{{ summary.active_agents }}</h3>
        </div>
      </div>
      <div class="stat-card glass-card yellow">
        <div class="card-icon">⏳</div>
        <div class="card-data">
          <span class="label">Tarefas Pendentes</span>
          <h3 class="value">{{ summary.open_issues }}</h3>
        </div>
      </div>
      <div class="stat-card glass-card red">
        <div class="card-icon">⚖️</div>
        <div class="card-data">
          <span class="label">Aprovações Críticas</span>
          <h3 class="value">{{ summary.pending_approvals }}</h3>
        </div>
      </div>
    </div>

    <!-- ⚡ SEÇÃO LIGHTNING (INTELIGÊNCIA) -->
    <div class="lightning-grid">
      <div class="intel-card glass-card purple">
        <div class="intel-header">
          <div class="card-icon">⚡</div>
          <span class="label">Dopamina Digital (Média)</span>
        </div>
        <div class="reward-viz">
          <div class="reward-progress" :style="{ width: (lightning.avg_reward * 100) + '%' }"></div>
          <span class="reward-value">{{ (lightning.avg_reward * 100).toFixed(1) }}%</span>
        </div>
        <p class="intel-desc">Nível de satisfação do Comandante com o enxame.</p>
      </div>

      <div class="intel-card glass-card cyan">
        <div class="intel-header">
          <div class="card-icon">📈</div>
          <span class="label">Sessões de Treino (Rollouts)</span>
        </div>
        <h3 class="valueLarge">{{ lightning.total_rollouts }}</h3>
        <p class="intel-desc">Trajetórias processadas pelo DuckDB.</p>
      </div>

      <div class="intel-card glass-card orange">
        <div class="intel-header">
          <div class="card-icon">🧠</div>
          <span class="label">Status do Aprendizado</span>
        </div>
        <div class="status-badge" :class="lightning.status">
          {{ lightning.status.toUpperCase() }}
        </div>
        <p class="intel-desc">Motor APO analisando falhas no cache.</p>
      </div>

      <div class="intel-card glass-card yellow">
        <div class="intel-header">
          <div class="card-icon">💵</div>
          <span class="label">Economia de Escala (USD)</span>
        </div>
        <h3 class="valueLarge">${{ (lightning.total_cost_usd).toFixed(4) }}</h3>
        <p class="intel-desc">{{ (lightning.prompt_tokens + lightning.completion_tokens).toLocaleString() }} tokens consumidos.</p>
      </div>
    </div>

    <!-- 🤖 CÓRTEX DE DECISÃO (BEAM SEARCH) -->
    <div v-if="pendingCandidates.length > 0" class="decision-cortex glass-panel mb-4">
      <div class="section-header">
        <span class="pulse-purple"></span>
        <h2>Córtex de Decisão Evolutiva (Variantes APO)</h2>
      </div>
      <div class="variants-grid">
        <div v-for="cand in pendingCandidates" :key="cand.id" class="variant-card glass-card purple-glow">
          <div class="variant-header">
            <span class="variant-personality">{{ cand.name }}</span>
            <div class="variant-meta">
              <span class="variant-accuracy" :class="getAccuracyClass(cand.accuracy)">
                📦 {{ cand.accuracy.toFixed(0) }}% GOLD
              </span>
              <span class="variant-agent">{{ cand.agent }}</span>
            </div>
          </div>
          <p class="variant-critique">{{ cand.critique }}</p>
          <div class="variant-actions">
            <button @click="handleApproveVariant(cand.id)" class="activate-btn">Ativar Inteligência</button>
          </div>
        </div>
      </div>
    </div>

    <!-- 🌊 FLUXO DE CONSCIÊNCIA (LIVE ROLLOUTS) -->
    <div class="trace-section glass-panel mb-4">
      <div class="section-header">
        <span class="pulse-cyan"></span>
        <h2>Fluxo de Consciência (Live Rollouts)</h2>
      </div>
      <div v-if="liveSpans.length === 0" class="empty-state">
        <p>Aguardando atividade do enxame...</p>
      </div>
      <div v-else class="trace-list">
        <div v-for="span in liveSpans" :key="span.id" class="trace-item">
          <span class="trace-time">{{ span.time }}</span>
          <span class="trace-method">[{{ span.op.toUpperCase() }}]</span>
          <div class="trace-id">{{ span.agent.toUpperCase() }} ({{ span.id.substring(0,8) }})</div>
          <div class="trace-media" v-if="span.media" :title="'Mídia capturada: ' + span.media">👁️</div>
          <div class="trace-tokens">{{ span.usage }} tokens</div>
        </div>
      </div>
    </div>

    <section class="approvals-section glass-panel">
      <div class="section-header">
        <span class="pulse-red"></span>
        <h2>Fila de Portões de Aprovação</h2>
      </div>

      <div v-if="pendingApprovals.length === 0" class="empty-state">
        <p>Nenhuma ação pendente. O enxame está operando em harmonia.</p>
      </div>

      <div v-else class="approval-list">
        <div v-for="app in pendingApprovals" :key="app.id" class="approval-item glass-item">
          <div class="app-info">
            <span class="app-type">{{ app.type.replace('_', ' ').toUpperCase() }}</span>
            <div class="app-payload">{{ app.payload }}</div>
            <span class="app-date">{{ new Date(app.created_at).toLocaleString() }}</span>
          </div>
          <div class="app-actions">
            <button @click="handleReject(app.id)" class="reject-btn">Rejeitar</button>
            <button @click="handleApprove(app.id)" class="approve-btn">Aprovar</button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.swarm-dashboard {
  flex: 1;
  padding: 30px;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(20px);
  color: white;
  overflow-y: auto;
  font-family: 'Inter', sans-serif;
}

.dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 40px;
}

.title-group h1 { font-size: 24px; font-weight: 800; letter-spacing: -1px; margin: 0; }
.title-group p { font-size: 14px; color: #94a3b8; margin-top: 4px; }

.header-actions { display: flex; gap: 15px; align-items: center; }

.rlhf-btn {
  background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.2);
  color: #f59e0b; padding: 10px 20px; border-radius: 10px; font-weight: 700;
  font-size: 13px; cursor: pointer; transition: 0.2s;
}
.rlhf-btn:hover { background: rgba(245, 158, 11, 0.2); transform: translateY(-2px); }

.export-btn {
  background: rgba(6, 182, 212, 0.1); border: 1px solid rgba(6, 182, 212, 0.2);
  color: #22d3ee; padding: 10px 20px; border-radius: 10px; font-weight: 700;
  font-size: 13px; cursor: pointer; transition: 0.2s;
}
.export-btn:hover { background: rgba(6, 182, 212, 0.2); transform: translateY(-2px); }

.refresh-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  padding: 10px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s;
}
.refresh-btn:hover { background: rgba(59, 130, 246, 0.2); color: #60a5fa; }
.rotating { animation: spin 1s infinite linear; }

@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 20px;
  margin-bottom: 40px;
}

.glass-card {
  padding: 24px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  align-items: center;
  gap: 20px;
  transition: transform 0.2s;
}
.glass-card:hover { transform: translateY(-5px); background: rgba(255, 255, 255, 0.05); }

.card-icon { font-size: 32px; }
.label { font-size: 12px; color: #94a3b8; font-weight: 600; text-transform: uppercase; }
.value { font-size: 20px; font-weight: 900; margin: 4px 0 0 0; }

.blue { border-left: 4px solid #3b82f6; }
.green { border-left: 4px solid #10b981; }
.yellow { border-left: 4px solid #f59e0b; }
.red { border-left: 4px solid #ef4444; }

.glass-panel {
  background: rgba(15, 23, 42, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 24px;
  padding: 24px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}
.section-header h2 { font-size: 16px; font-weight: 700; margin: 0; }

.pulse-red {
  width: 8px;
  height: 8px;
  background: #ef4444;
  border-radius: 50%;
  animation: pulse 1.5s infinite;
}

@keyframes pulse { 0% { box-shadow: 0 0 0 0 rgba(239, 68, 68, 0.7); } 70% { box-shadow: 0 0 0 10px rgba(239, 68, 68, 0); } 100% { box-shadow: 0 0 0 0 rgba(239, 68, 68, 0); } }

.empty-state { text-align: center; padding: 40px; color: #64748b; font-style: italic; }

.approval-list { display: flex; flex-direction: column; gap: 12px; }

.glass-item {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  padding: 20px;
  border-radius: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}

.app-type { font-size: 10px; font-weight: 800; color: #3b82f6; letter-spacing: 1px; }
.app-payload { font-size: 14px; margin: 8px 0; line-height: 1.6; color: #e2e8f0; white-space: pre-wrap; }
.app-date { font-size: 11px; color: #64748b; }

.app-actions { display: flex; gap: 10px; }

.approve-btn {
  background: #10b981; color: white; border: none; padding: 8px 20px;
  border-radius: 8px; font-weight: 700; cursor: pointer; transition: 0.2s;
}
.approve-btn:hover { background: #059669; transform: scale(1.05); }

.reject-btn {
  background: rgba(239, 68, 68, 0.1); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.2);
  padding: 8px 20px; border-radius: 8px; font-weight: 700; cursor: pointer; transition: 0.2s;
}
.reject-btn:hover { background: rgba(239, 68, 68, 0.2); }

/* ⚡ ESTILOS LIGHTNING */
.lightning-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 20px;
  margin-bottom: 40px;
}

.intel-card {
  flex-direction: column;
  align-items: flex-start;
  padding: 24px;
}

.intel-header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }

.reward-viz {
  width: 100%;
  height: 30px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 15px;
  position: relative;
  overflow: hidden;
  margin: 10px 0;
}

.reward-progress {
  height: 100%;
  background: linear-gradient(90deg, #8b5cf6, #3b82f6);
  box-shadow: 0 0 20px rgba(139, 92, 246, 0.5);
  transition: width 1s cubic-bezier(0.4, 0, 0.2, 1);
}

.reward-value {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  font-size: 12px;
  font-weight: 900;
  color: white;
  text-shadow: 0 2px 4px rgba(0,0,0,0.5);
}

.valueLarge { font-size: 32px; font-weight: 900; margin: 10px 0; color: #22d3ee; }

.intel-desc { font-size: 11px; color: #64748b; margin: 0; }

.status-badge {
  padding: 6px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 800;
  margin: 10px 0;
}
.status-badge.online { background: rgba(16, 185, 129, 0.2); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); }
.status-badge.offline { background: rgba(100, 116, 139, 0.2); color: #64748b; }

.purple { border-top: 4px solid #8b5cf6; }
.cyan { border-top: 4px solid #06b6d4; }
.orange { border-top: 4px solid #f97316; }

/* 🌊 TRACE STYLES */
.trace-section { margin-bottom: 20px; }
.pulse-cyan {
  width: 8px; height: 8px; background: #06b6d4; border-radius: 50%;
  animation: pulse-cyan 1.5s infinite;
}
@keyframes pulse-cyan { 0% { box-shadow: 0 0 0 0 rgba(6, 182, 212, 0.7); } 70% { box-shadow: 0 0 0 10px rgba(6, 182, 212, 0); } 100% { box-shadow: 0 0 0 0 rgba(6, 182, 212, 0); } }

.trace-list { display: flex; flex-direction: column; gap: 8px; }
.trace-item {
  display: flex; align-items: center; gap: 15px; padding: 12px 20px;
  background: rgba(0, 0, 0, 0.2); border-left: 3px solid #06b6d4; border-radius: 8px;
  font-family: 'Fira Code', monospace; font-size: 13px;
}
.trace-time { color: #06b6d4; font-weight: 700; width: 70px; }
.trace-method { color: #e2e8f0; font-weight: 600; width: 120px; }
.trace-id { color: #64748b; flex: 1; }
.trace-media { 
  font-size: 16px; cursor: help; filter: drop-shadow(0 0 5px #06b6d4);
  animation: pulse-eye 2s infinite ease-in-out;
}
@keyframes pulse-eye { 0%, 100% { opacity: 0.5; } 50% { opacity: 1; } }

.trace-tokens { background: rgba(59, 130, 246, 0.1); color: #60a5fa; padding: 2px 8px; border-radius: 4px; font-size: 11px; }

/* 🧠 BEAM SEARCH STYLES */
.decision-cortex { background: rgba(88, 28, 135, 0.15); border: 1px solid rgba(139, 92, 246, 0.3); }
.pulse-purple {
  width: 8px; height: 8px; background: #a78bfa; border-radius: 50%;
  animation: pulse-purple 1.5s infinite;
}
@keyframes pulse-purple { 0% { box-shadow: 0 0 0 0 rgba(167, 139, 250, 0.7); } 70% { box-shadow: 0 0 0 10px rgba(167, 139, 250, 0); } 100% { box-shadow: 0 0 0 0 rgba(167, 139, 250, 0); } }

.variants-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}
.variant-card {
  flex-direction: column; align-items: stretch; padding: 20px;
  background: rgba(0, 0, 0, 0.4); border: 1px solid rgba(139, 92, 246, 0.2);
}
.purple-glow:hover { box-shadow: 0 0 20px rgba(139, 92, 246, 0.3); }

.variant-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 15px; }
.variant-meta { display: flex; flex-direction: column; align-items: flex-end; gap: 4px; }
.variant-personality { font-size: 14px; font-weight: 900; color: #a78bfa; text-transform: uppercase; letter-spacing: 1px; }
.variant-agent { font-size: 10px; color: #64748b; background: rgba(255,255,255,0.05); padding: 2px 8px; border-radius: 4px; }

.variant-accuracy {
  font-size: 10px; font-weight: 800; padding: 2px 6px; border-radius: 4px;
}
.acc-high { background: rgba(16, 185, 129, 0.2); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); }
.acc-med { background: rgba(245, 158, 11, 0.2); color: #f59e0b; border: 1px solid rgba(245, 158, 11, 0.3); }
.acc-low { background: rgba(239, 68, 68, 0.2); color: #ef4444; border: 1px solid rgba(239, 68, 68, 0.3); }

.variant-critique { font-size: 12px; color: #e2e8f0; line-height: 1.5; margin-bottom: 20px; font-style: italic; }

.activate-btn {
  background: linear-gradient(90deg, #8b5cf6, #6366f1); color: white; border: none;
  padding: 10px; border-radius: 8px; font-weight: 800; font-size: 11px; cursor: pointer;
  transition: 0.3s; text-transform: uppercase;
}
.activate-btn:hover { filter: brightness(1.2); transform: scale(1.02); }

</style>
