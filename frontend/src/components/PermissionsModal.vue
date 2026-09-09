<script setup>
import { ref, watch, computed } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()
const isLoading = ref(false)
const config = ref({
  allow_read: true,
  allow_write: true,
  allow_create: true,
  allow_delete: false,
  allow_move: true,
  allow_run_commands: true,
  full_machine_access: false,
  workspaces: []
})

// Estado de Permissões Granulares Antigravity CLI (action(target))
const antigravitySettings = ref(null)
const activePermTab = ref('allow') // 'allow', 'ask', 'deny'
const newRuleLevel = ref('allow')
const newRuleAction = ref('command')
const newRuleTarget = ref('')

const loadPermissions = async () => {
  isLoading.value = true
  try {
    const res = await orchestrator.getSecurityPermissions()
    if (res) config.value = res

    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.GetAntigravitySettings === 'function') {
      const agySettings = await bridge.GetAntigravitySettings()
      if (agySettings) antigravitySettings.value = agySettings
    }
  } catch (err) {
    console.error('[Permissions] Erro:', err)
  } finally {
    isLoading.value = false
  }
}

const addRule = async () => {
  if (!newRuleTarget.value.trim()) return
  try {
    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.SaveAntigravityPermissionRule === 'function') {
      await bridge.SaveAntigravityPermissionRule(newRuleLevel.value, newRuleAction.value, newRuleTarget.value.trim())
      newRuleTarget.value = ''
      await loadPermissions()
      orchestrator.pushStatus(`🛡️ Regra adicionada: ${newRuleAction.value}(...) em ${newRuleLevel.value}`, 'success')
    }
  } catch (err) {
    console.error('[Permissions] Erro ao salvar regra:', err)
  }
}

const deleteRule = async (ruleStr) => {
  try {
    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.RemoveAntigravityPermissionRule === 'function') {
      await bridge.RemoveAntigravityPermissionRule(ruleStr)
      await loadPermissions()
      orchestrator.pushStatus(`🗑️ Regra removida: ${ruleStr}`, 'status')
    }
  } catch (err) {
    console.error('[Permissions] Erro ao deletar regra:', err)
  }
}

const currentTabRules = computed(() => {
  if (!antigravitySettings.value?.permissions) return []
  const list = antigravitySettings.value.permissions[activePermTab.value]
  return Array.isArray(list) ? list : []
})

watch(() => orchestrator.isPermissionsModalOpen, (isOpen) => {
  if (isOpen) {
    loadPermissions()
  }
})

const close = () => {
  orchestrator.togglePermissionsModal(false)
}
</script>

<template>
  <Transition name="perm-fade">
    <div v-if="orchestrator.isPermissionsModalOpen" class="perm-modal-backdrop" @click.self="close">
      <div class="perm-modal glass-heavy">
        <header class="modal-header">
          <div class="header-left">
            <span class="header-icon">🛡️</span>
            <div>
              <h3>SEGURANÇA & PERMISSÕES (/permissions)</h3>
              <p>Políticas de execução segura, isolamento e permissões do sistema</p>
            </div>
          </div>
          <button class="close-btn" @click="close" title="Fechar (Esc)">✕</button>
        </header>

        <div class="modal-body">
          <div v-if="isLoading" class="loading-state">
            <div class="spinner"></div>
            <span>Carregando políticas de segurança...</span>
          </div>

          <div v-else class="perm-content">
            <!-- Sandbox & Machine Access Status -->
            <div class="status-cards">
              <div class="status-card">
                <span class="card-label">ISOLAMENTO DE MÁQUINA</span>
                <div class="card-val-row">
                  <span class="mode-pill" :class="config.full_machine_access ? 'warning' : 'secure'">
                    {{ config.full_machine_access ? 'ACESSO TOTAL' : 'SANDBOX / WORKSPACES' }}
                  </span>
                  <span class="mode-desc">
                    {{ config.full_machine_access ? 'Acesso a todo o sistema de arquivos' : 'Operações restritas às pastas autorizadas' }}
                  </span>
                </div>
              </div>

              <div class="status-card">
                <span class="card-label">EXECUÇÃO DE COMANDOS</span>
                <div class="card-val-row">
                  <span class="badge-status" :class="config.allow_run_commands ? 'active' : 'inactive'">
                    {{ config.allow_run_commands ? 'PERMITIDO' : 'BLOQUEADO' }}
                  </span>
                  <span class="mode-desc">Execução de binários e comandos shell/powershell</span>
                </div>
              </div>
            </div>

            <!-- Granular Permissions Grid -->
            <div class="policy-section">
              <div class="section-title">
                <span>PERMISSÕES GRANULARES DE ARQUIVOS</span>
              </div>
              <div class="perms-grid">
                <div class="perm-item" :class="{ enabled: config.allow_read }">
                  <span class="perm-icon">{{ config.allow_read ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Leitura de Arquivos</span>
                    <span class="perm-subtitle">Permitir que a IA inspecione arquivos</span>
                  </div>
                </div>

                <div class="perm-item" :class="{ enabled: config.allow_write }">
                  <span class="perm-icon">{{ config.allow_write ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Edição / Escrita</span>
                    <span class="perm-subtitle">Modificar código existente</span>
                  </div>
                </div>

                <div class="perm-item" :class="{ enabled: config.allow_create }">
                  <span class="perm-icon">{{ config.allow_create ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Criação de Arquivos</span>
                    <span class="perm-subtitle">Gerar novos arquivos e pastas</span>
                  </div>
                </div>

                <div class="perm-item" :class="{ enabled: config.allow_delete }">
                  <span class="perm-icon">{{ config.allow_delete ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Exclusão de Arquivos</span>
                    <span class="perm-subtitle">Remover arquivos do disco</span>
                  </div>
                </div>

                <div class="perm-item" :class="{ enabled: config.allow_move }">
                  <span class="perm-icon">{{ config.allow_move ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Mover / Renomear</span>
                    <span class="perm-subtitle">Reorganizar arquivos no projeto</span>
                  </div>
                </div>

                <div class="perm-item" :class="{ enabled: config.allow_run_commands }">
                  <span class="perm-icon">{{ config.allow_run_commands ? '✓' : '✕' }}</span>
                  <div class="perm-text">
                    <span class="perm-title">Comandos de Terminal</span>
                    <span class="perm-subtitle">Rodar testes, compilação e git</span>
                  </div>
                </div>
              </div>
            </div>

            <!-- Workspaces Whitelist -->
            <div class="policy-section">
              <div class="section-title">
                <span>PASTAS & WORKSPACES AUTORIZADOS ({{ config.workspaces?.length || 0 }})</span>
              </div>
              <div class="workspace-tags" v-if="config.workspaces && config.workspaces.length > 0">
                <span v-for="ws in config.workspaces" :key="ws" class="ws-tag">
                  📁 {{ ws }}
                </span>
              </div>
              <span v-else class="empty-hint">Nenhuma pasta adicional na whitelist (apenas workspace ativo)</span>
            </div>

            <!-- Antigravity Fine-Grained Permissions (action(target)) -->
            <div class="policy-section">
              <div class="section-title-row">
                <span class="section-title-text">REGRAS FINAS ANTIGRAVITY (action(target))</span>
                <span class="priority-hint">Hierarquia: <strong>Deny > Ask > Allow</strong></span>
              </div>

              <!-- Tabs Allow / Ask / Deny -->
              <div class="perm-rules-tabs">
                <button 
                  class="rule-tab-btn" 
                  :class="{ active: activePermTab === 'allow', allow: activePermTab === 'allow' }"
                  @click="activePermTab = 'allow'"
                >
                  🟢 Permitir ({{ antigravitySettings?.permissions?.allow?.length || 0 }})
                </button>
                <button 
                  class="rule-tab-btn" 
                  :class="{ active: activePermTab === 'ask', ask: activePermTab === 'ask' }"
                  @click="activePermTab = 'ask'"
                >
                  🟡 Perguntar ({{ antigravitySettings?.permissions?.ask?.length || 0 }})
                </button>
                <button 
                  class="rule-tab-btn" 
                  :class="{ active: activePermTab === 'deny', deny: activePermTab === 'deny' }"
                  @click="activePermTab = 'deny'"
                >
                  🔴 Bloquear ({{ antigravitySettings?.permissions?.deny?.length || 0 }})
                </button>
              </div>

              <!-- Lista de Regras da Aba Ativa -->
              <div class="rules-list-box" v-if="currentTabRules.length > 0">
                <div v-for="rule in currentTabRules" :key="rule" class="rule-row">
                  <span class="rule-text">{{ rule }}</span>
                  <button class="rule-del-btn" @click="deleteRule(rule)" title="Remover regra">×</button>
                </div>
              </div>
              <span v-else class="empty-hint">Nenhuma regra configurada para esta categoria.</span>

              <!-- Formulário para Adicionar Nova Regra -->
              <div class="add-rule-form">
                <select v-model="newRuleLevel" class="rule-select level-select">
                  <option value="allow">Allow (Permitir)</option>
                  <option value="ask">Ask (Perguntar)</option>
                  <option value="deny">Deny (Bloquear)</option>
                </select>
                <select v-model="newRuleAction" class="rule-select action-select">
                  <option value="command">command</option>
                  <option value="read">read</option>
                  <option value="write">write</option>
                  <option value="network">network</option>
                  <option value="tool">tool</option>
                </select>
                <input 
                  v-model="newRuleTarget" 
                  type="text" 
                  placeholder="Alvo (ex: git status, src/*, rm -rf)" 
                  class="rule-input"
                  @keydown.enter="addRule"
                />
                <button class="btn-add-rule" @click="addRule">+ Adicionar</button>
              </div>
            </div>
          </div>
        </div>

        <footer class="modal-footer">
          <p class="footer-hint">💡 O Lumaestro protege seu sistema interceptando chamadas destrutivas antes da execução.</p>
          <button class="btn-done" @click="close">Concluído</button>
        </footer>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.perm-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(10px);
  padding: 30px;
}

.perm-modal {
  width: 100%;
  max-width: 720px;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.8);
  overflow: hidden;
}

.modal-header {
  padding: 18px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(255, 255, 255, 0.02);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.header-icon { font-size: 1.6rem; }

.header-left h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 1.5px;
  color: #f1f5f9;
}

.header-left p {
  margin: 2px 0 0 0;
  font-size: 11px;
  color: #94a3b8;
}

.close-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 16px;
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 8px;
  transition: all 0.2s;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: white;
}

.modal-body {
  padding: 24px;
  overflow-y: auto;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
  gap: 12px;
  color: #94a3b8;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.status-cards {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
  margin-bottom: 20px;
}

.status-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 12px;
  padding: 14px;
}

.card-label {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #64748b;
  display: block;
  margin-bottom: 8px;
}

.card-val-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.mode-pill {
  font-size: 12px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  width: fit-content;
}

.mode-pill.secure {
  color: #60a5fa;
  background: rgba(59, 130, 246, 0.15);
}

.mode-pill.warning {
  color: #facc15;
  background: rgba(234, 179, 8, 0.15);
}

.badge-status {
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 6px;
  width: fit-content;
}

.badge-status.active {
  color: #4ade80;
  background: rgba(34, 197, 94, 0.15);
}

.badge-status.inactive {
  color: #f87171;
  background: rgba(239, 68, 68, 0.15);
}

.mode-desc {
  font-size: 11px;
  color: #94a3b8;
}

.policy-section {
  margin-bottom: 20px;
}

.section-title {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #94a3b8;
  margin-bottom: 10px;
}

.perms-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.perm-item {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 10px;
  padding: 10px 14px;
  display: flex;
  align-items: center;
  gap: 12px;
}

.perm-item.enabled {
  border-color: rgba(34, 197, 94, 0.2);
  background: rgba(34, 197, 94, 0.04);
}

.perm-icon {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 13px;
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
}

.perm-item.enabled .perm-icon {
  background: rgba(34, 197, 94, 0.2);
  color: #4ade80;
}

.perm-text {
  display: flex;
  flex-direction: column;
}

.perm-title {
  font-size: 12px;
  font-weight: 700;
  color: #f1f5f9;
}

.perm-subtitle {
  font-size: 10px;
  color: #64748b;
}

.workspace-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.ws-tag {
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.25);
  color: #93c5fd;
  font-size: 11px;
  padding: 4px 10px;
  border-radius: 6px;
  font-family: monospace;
}

.empty-hint {
  font-size: 11px;
  color: #64748b;
  font-style: italic;
}

.modal-footer {
  padding: 14px 24px;
  background: rgba(0, 0, 0, 0.2);
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.footer-hint {
  margin: 0;
  font-size: 11px;
  color: #64748b;
}

.btn-done {
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 8px;
  padding: 6px 16px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.btn-done:hover {
  background: #1d4ed8;
}

/* Antigravity Fine-Grained Rules */
.section-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.section-title-text {
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #94a3b8;
}

.priority-hint {
  font-size: 10px;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.1);
  border: 1px solid rgba(56, 189, 248, 0.2);
  padding: 2px 8px;
  border-radius: 4px;
}

.perm-rules-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.rule-tab-btn {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s;
}

.rule-tab-btn.active.allow {
  background: rgba(34, 197, 94, 0.2);
  border-color: rgba(34, 197, 94, 0.5);
  color: #4ade80;
}

.rule-tab-btn.active.ask {
  background: rgba(234, 179, 8, 0.2);
  border-color: rgba(234, 179, 8, 0.5);
  color: #facc15;
}

.rule-tab-btn.active.deny {
  background: rgba(239, 68, 68, 0.2);
  border-color: rgba(239, 68, 68, 0.5);
  color: #f87171;
}

.rules-list-box {
  max-height: 140px;
  overflow-y: auto;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 12px;
}

.rule-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.02);
  border-radius: 4px;
  font-family: monospace;
  font-size: 11px;
  color: #e2e8f0;
}

.rule-text {
  word-break: break-all;
}

.rule-del-btn {
  background: transparent;
  border: none;
  color: #ef4444;
  font-size: 14px;
  cursor: pointer;
  padding: 0 4px;
}

.add-rule-form {
  display: flex;
  gap: 6px;
  align-items: center;
}

.rule-select {
  background: #1e293b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #f8fafc;
  font-size: 11px;
  padding: 6px 8px;
  border-radius: 6px;
  outline: none;
}

.rule-input {
  flex: 1;
  background: #1e293b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #f8fafc;
  font-size: 11px;
  padding: 6px 10px;
  border-radius: 6px;
  outline: none;
}

.btn-add-rule {
  background: #3b82f6;
  border: none;
  color: white;
  font-size: 11px;
  font-weight: 700;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  white-space: nowrap;
}

.perm-fade-enter-active, .perm-fade-leave-active {
  transition: opacity 0.25s ease;
}
.perm-fade-enter-from, .perm-fade-leave-to {
  opacity: 0;
}
</style>
