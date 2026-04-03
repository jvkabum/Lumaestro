<script setup>
import { onMounted, ref } from 'vue'
import { GetExecutiveSummary, GetPendingApprovals, ApproveAction, RejectAction } from '../../wailsjs/go/main/App'

const summary = ref({
  total_spent_cents: 0,
  active_agents: 0,
  paused_agents: 0,
  open_issues: 0,
  done_issues: 0,
  pending_approvals: 0
})

const pendingApprovals = ref([])
const isLoading = ref(true)

const loadDashboard = async () => {
  isLoading.value = true
  try {
    const s = await GetExecutiveSummary()
    if (s) summary.value = s
    
    // Supondo que GetPendingApprovals retorne a lista de Approval com Status pending
    // Para simplificar, usamos uma query direta aqui se o binding permitir
    // ou chamamos um método específico.
    const approvals = await GetPendingApprovals()
    pendingApprovals.value = approvals || []
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

onMounted(loadDashboard)

const formatCurrency = (cents) => {
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(cents / 100)
}
</script>

<template>
  <div class="swarm-dashboard glass">
    <header class="dashboard-header">
      <div class="title-group">
        <h1>Dashboard Executivo</h1>
        <p>Visão em tempo real da Autonomia do Enxame</p>
      </div>
      <button @click="loadDashboard" class="refresh-btn" :class="{ rotating: isLoading }">
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path>
        </svg>
      </button>
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
</style>
