<script setup>
import { ref, computed, watch } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()

const isLoading = ref(false)
const diffData = ref({ statusText: '', modified: [], untracked: [], diffText: '' })
const copied = ref(false)

const loadDiff = async () => {
  isLoading.value = true
  try {
    const res = await orchestrator.getWorkspaceDiff()
    diffData.value = res || { statusText: '', modified: [], untracked: [], diffText: '' }
  } catch (err) {
    console.error('[DiffViewer] Erro ao carregar diff:', err)
  } finally {
    isLoading.value = false
  }
}

watch(() => orchestrator.isDiffViewerOpen, (isOpen) => {
  if (isOpen) {
    loadDiff()
  }
})

const close = () => {
  orchestrator.toggleDiffViewer(false)
}

const copyDiff = () => {
  const text = diffData.value.diffText || diffData.value.statusText
  if (!text) return
  navigator.clipboard?.writeText(text)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}

const allFiles = computed(() => {
  const list = []
  if (Array.isArray(diffData.value.modified)) {
    for (const f of diffData.value.modified) {
      list.push({ path: f, status: 'M' })
    }
  }
  if (Array.isArray(diffData.value.untracked)) {
    for (const f of diffData.value.untracked) {
      list.push({ path: f, status: '??' })
    }
  }
  return list
})

const parsedLines = computed(() => {
  const diffStr = diffData.value.diffText || ''
  if (!diffStr) return []
  return diffStr.split('\n').map((line, idx) => {
    let type = 'normal'
    if (line.startsWith('+++') || line.startsWith('---')) {
      type = 'file-header'
    } else if (line.startsWith('+')) {
      type = 'add'
    } else if (line.startsWith('-')) {
      type = 'del'
    } else if (line.startsWith('@@')) {
      type = 'chunk'
    } else if (line.startsWith('diff --git') || line.startsWith('index ')) {
      type = 'meta'
    }
    return { id: idx, line, type }
  })
})

const hasChanges = computed(() => {
  return allFiles.value.length > 0 || (diffData.value.diffText && diffData.value.diffText.trim().length > 0)
})
</script>

<template>
  <Transition name="diff-fade">
    <div v-if="orchestrator.isDiffViewerOpen" class="diff-modal-backdrop" @click.self="close">
      <div class="diff-modal glass-heavy">
        <!-- Header -->
        <header class="modal-header">
          <div class="header-left">
            <span class="header-icon">📑</span>
            <div>
              <h3>ALTERAÇÕES DO WORKSPACE (/diff)</h3>
              <p>Status do Git e diferenças de código em tempo real</p>
            </div>
          </div>
          <div class="header-actions">
            <button class="btn-action" @click="loadDiff" :disabled="isLoading" title="Recarregar">
              <span>🔄 Atualizar</span>
            </button>
            <button class="btn-action" @click="copyDiff" :disabled="!hasChanges" title="Copiar Diff">
              <span>{{ copied ? '✓ Copiado!' : '📋 Copiar Diff' }}</span>
            </button>
            <button class="close-btn" @click="close" title="Fechar (Esc)">✕</button>
          </div>
        </header>

        <!-- Main Body -->
        <div class="modal-body">
          <div v-if="isLoading" class="loading-state">
            <div class="spinner"></div>
            <span>Calculando alterações do Git...</span>
          </div>

          <div v-else-if="!hasChanges" class="empty-state">
            <span class="empty-icon">🌿</span>
            <h4>Workspace limpo (Clean tree)</h4>
            <p>Nenhuma alteração pendente ou arquivos modificados no repositório.</p>
          </div>

          <div v-else class="diff-content">
            <!-- Sidebar com arquivos alterados -->
            <aside class="files-sidebar">
              <div class="sidebar-header">
                ARQUIVOS ({{ allFiles.length }})
              </div>
              <ul class="files-list">
                <li v-for="(file, idx) in allFiles" :key="idx" class="file-item">
                  <span class="status-badge" :class="file.status === 'M' ? 'badge-mod' : 'badge-untracked'">
                    {{ file.status }}
                  </span>
                  <span class="file-name" :title="file.path">{{ file.path }}</span>
                </li>
              </ul>
            </aside>

            <!-- Unified Diff Viewer -->
            <div class="diff-viewer">
              <div v-if="parsedLines.length > 0" class="diff-code-wrapper">
                <div 
                  v-for="item in parsedLines" 
                  :key="item.id" 
                  class="diff-line"
                  :class="'diff-' + item.type"
                >
                  <span class="diff-gutter">{{ item.type === 'add' ? '+' : item.type === 'del' ? '-' : ' ' }}</span>
                  <span class="diff-text">{{ item.line }}</span>
                </div>
              </div>
              <div v-else class="no-diff-text">
                <p>Nenhum diff textual gerado (arquivos podem ser novos ou não rastreados).</p>
                <pre class="raw-status">{{ diffData.statusText }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.diff-modal-backdrop {
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

.diff-modal {
  width: 100%;
  max-width: 1050px;
  height: 85vh;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.8);
  overflow: hidden;
}

.modal-header {
  padding: 16px 24px;
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

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.btn-action {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #cbd5e1;
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-action:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  color: white;
}

.btn-action:disabled {
  opacity: 0.4;
  cursor: not-allowed;
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
  flex: 1;
  display: flex;
  overflow: hidden;
  background: #090d16;
}

.loading-state, .empty-state {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #94a3b8;
}

.empty-icon { font-size: 3rem; }

.empty-state h4 {
  margin: 0;
  font-size: 16px;
  color: #f1f5f9;
}

.empty-state p {
  margin: 0;
  font-size: 13px;
  color: #64748b;
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

.diff-content {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.files-sidebar {
  width: 260px;
  background: rgba(15, 23, 42, 0.7);
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
}

.sidebar-header {
  padding: 12px 16px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #64748b;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.files-list {
  list-style: none;
  margin: 0;
  padding: 8px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.file-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  font-size: 12px;
  color: #cbd5e1;
  background: rgba(255, 255, 255, 0.02);
}

.status-badge {
  font-size: 10px;
  font-family: monospace;
  font-weight: 800;
  padding: 2px 6px;
  border-radius: 4px;
  min-width: 24px;
  text-align: center;
}

.badge-mod { background: rgba(234, 179, 8, 0.2); color: #facc15; }
.badge-untracked { background: rgba(148, 163, 184, 0.2); color: #94a3b8; }

.file-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.diff-viewer {
  flex: 1;
  overflow: auto;
  padding: 12px;
  background: #0b1120;
}

.diff-code-wrapper {
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
}

.diff-line {
  display: flex;
  padding: 1px 6px;
  white-space: pre-wrap;
  word-break: break-all;
}

.diff-gutter {
  user-select: none;
  width: 20px;
  min-width: 20px;
  opacity: 0.5;
}

.diff-text {
  flex: 1;
}

.diff-add {
  background: rgba(34, 197, 94, 0.15);
  color: #86efac;
}

.diff-del {
  background: rgba(239, 68, 68, 0.15);
  color: #fca5a5;
}

.diff-chunk {
  background: rgba(56, 189, 248, 0.1);
  color: #38bdf8;
  font-weight: 700;
  margin: 6px 0;
  border-radius: 4px;
}

.diff-meta {
  color: #818cf8;
  font-weight: 700;
  margin-top: 8px;
}

.diff-file-header {
  color: #fbbf24;
  font-weight: 700;
}

.no-diff-text {
  padding: 30px;
  color: #94a3b8;
  font-size: 13px;
}

.raw-status {
  font-family: monospace;
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  border-radius: 8px;
  margin-top: 10px;
  color: #cbd5e1;
}

.diff-fade-enter-active, .diff-fade-leave-active {
  transition: opacity 0.25s ease;
}
.diff-fade-enter-from, .diff-fade-leave-to {
  opacity: 0;
}
</style>
