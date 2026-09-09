<script setup>
import { computed, ref, watch, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { storeToRefs } from 'pinia'
import { useOrchestratorStore } from '../stores/orchestrator'
import MarkdownIt from 'markdown-it'
import mermaid from 'mermaid'

const orchestrator = useOrchestratorStore()
const { messages, showPlanOverlay, currentACPID } = storeToRefs(orchestrator)

const md = new MarkdownIt({ html: true, linkify: true, typographer: true })

// Configuração do Mermaid
mermaid.initialize({
  startOnLoad: false,
  theme: 'dark',
  securityLevel: 'loose',
  fontFamily: 'Inter, system-ui, sans-serif'
})

// Estado de Artefatos
const artifacts = ref([])
const activeArtifactIndex = ref(0)
const viewMode = ref('rendered') // 'rendered' ou 'lines'
const activeLineNumber = ref(null)
const lineComments = ref({}) // { [lineNumber]: "comentario" }
const newCommentText = ref('')
const generalFeedback = ref('')
const isSubmitting = ref(false)
const mermaidContainer = ref(null)

// Fallback de plano derivado do chat se nenhum arquivo no disco for encontrado
const chatStrategy = computed(() => {
  const assistantMsgs = messages.value.filter(m => m.role === 'assistant' && !m.mode)
  if (assistantMsgs.length === 0) {
    return {
      name: 'Estratégia do Chat',
      content: '# Nenhum artefato encontrado\nO Maestro ainda não gerou um plano ou artefato na sessão ativa.',
      isPlan: true,
      totalLines: 2
    }
  }
  const last = assistantMsgs[assistantMsgs.length - 1]
  const content = last.text || ''
  return {
    name: 'Estratégia Ativa',
    content: content,
    isPlan: true,
    totalLines: content.split('\n').length
  }
})

// Carrega artefatos da sessão atual ou brain
const loadArtifacts = async () => {
  try {
    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.GetSessionArtifacts === 'function') {
      const list = await bridge.GetSessionArtifacts(currentACPID.value || '')
      if (Array.isArray(list) && list.length > 0) {
        artifacts.value = list
        activeArtifactIndex.value = 0
        return
      }
    }
  } catch (err) {
    console.warn('[PlanView] Falha ao listar artefatos:', err)
  }
  artifacts.value = [chatStrategy.value]
  activeArtifactIndex.value = 0
}

const currentArtifact = computed(() => {
  if (artifacts.value.length === 0) return chatStrategy.value
  return artifacts.value[activeArtifactIndex.value] || artifacts.value[0] || chatStrategy.value
})

const artifactLines = computed(() => {
  const text = currentArtifact.value?.content || ''
  return text.split('\n')
})

// Renderiza Markdown e detecta blocos Mermaid
const renderedMarkdown = computed(() => {
  const raw = currentArtifact.value?.content || ''
  // Pré-processamento simples para blocos ```mermaid
  const processed = raw.replace(/```mermaid([\s\S]*?)```/g, (match, code) => {
    return `<div class="mermaid-diagram" data-diagram="${encodeURIComponent(code.trim())}"></div>`
  })
  return md.render(processed)
})

const renderMermaidDiagrams = async () => {
  await nextTick()
  if (!mermaidContainer.value) return
  const diagramElements = mermaidContainer.value.querySelectorAll('.mermaid-diagram')
  diagramElements.forEach(async (el, idx) => {
    const raw = decodeURIComponent(el.getAttribute('data-diagram') || '')
    if (!raw) return
    try {
      const id = `mermaid-graph-${Date.now()}-${idx}`
      const { svg } = await mermaid.render(id, raw)
      el.innerHTML = svg
    } catch (err) {
      console.warn('[Mermaid] Erro na renderização do diagrama:', err)
      el.innerHTML = `<pre class="mermaid-error">⚠️ Diagrama Mermaid inválido:\n${raw}</pre>`
    }
  })
}

watch(currentArtifact, () => {
  lineComments.value = {}
  activeLineNumber.value = null
  newCommentText.value = ''
  if (viewMode.value === 'rendered') {
    renderMermaidDiagrams()
  }
})

watch(viewMode, (newMode) => {
  if (newMode === 'rendered') {
    renderMermaidDiagrams()
  }
})

watch(showPlanOverlay, (isOpen) => {
  if (isOpen) {
    loadArtifacts().then(() => {
      renderMermaidDiagrams()
    })
  }
})

// --- Gestão de Anotações por Linha ---
const selectLine = (lineNum) => {
  activeLineNumber.value = lineNum
  newCommentText.value = lineComments.value[lineNum] || ''
}

const saveLineComment = () => {
  if (activeLineNumber.value === null) return
  const trimmed = newCommentText.value.trim()
  if (trimmed) {
    lineComments.value[activeLineNumber.value] = trimmed
  } else {
    delete lineComments.value[activeLineNumber.value]
  }
  newCommentText.value = ''
  activeLineNumber.value = null
}

const deleteLineComment = (lineNum) => {
  delete lineComments.value[lineNum]
  if (activeLineNumber.value === lineNum) {
    newCommentText.value = ''
    activeLineNumber.value = null
  }
}

// --- Ações de Co-Steering (Aprovação / Solicitação de Alterações) ---
const approveArtifact = async () => {
  if (isSubmitting.value) return
  isSubmitting.value = true

  const commentsArray = Object.entries(lineComments.value).map(([line, text]) => ({
    lineNumber: parseInt(line, 10),
    comment: text
  }))

  try {
    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.SubmitArtifactReview === 'function') {
      await bridge.SubmitArtifactReview({
        artifactName: currentArtifact.value?.name || 'plano.md',
        approved: true,
        generalFeedback: generalFeedback.value,
        comments: commentsArray
      })
    }
    orchestrator.pushStatus(`✅ Artefato '${currentArtifact.value?.name}' aprovado. Execução autorizada!`, 'success')
  } catch (e) {
    console.error('[PlanView] Erro ao aprovar artefato:', e)
  } finally {
    isSubmitting.value = false
    orchestrator.showPlanOverlay = false
    orchestrator.isPlanMode = false // Sai do modo leitura ao aprovar
  }
}

const rejectArtifact = async () => {
  if (isSubmitting.value) return
  isSubmitting.value = true

  const commentsArray = Object.entries(lineComments.value).map(([line, text]) => ({
    lineNumber: parseInt(line, 10),
    comment: text
  }))

  try {
    const bridge = window.go?.core?.App || window.go?.main?.App
    if (bridge && typeof bridge.SubmitArtifactReview === 'function') {
      await bridge.SubmitArtifactReview({
        artifactName: currentArtifact.value?.name || 'plano.md',
        approved: false,
        generalFeedback: generalFeedback.value,
        comments: commentsArray
      })
    }
    orchestrator.pushStatus(`⚠️ Ajustes solicitados no artefato '${currentArtifact.value?.name}'. Instruções enviadas ao agente.`, 'warning')
  } catch (e) {
    console.error('[PlanView] Erro ao enviar revisão de artefato:', e)
  } finally {
    isSubmitting.value = false
    orchestrator.showPlanOverlay = false
  }
}

const closeOverlay = () => {
  orchestrator.showPlanOverlay = false
}

// --- Atalhos de Teclado Globais (y / n / c / d / Escape) ---
const handleKeydown = (e) => {
  if (!orchestrator.showPlanOverlay) {
    // Ctrl+R abre a revisão de artefatos
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'r') {
      e.preventDefault()
      orchestrator.showPlanOverlay = true
    }
    return
  }

  // Se o foco estiver em um input ou textarea, não intercepta letras simples
  const isInputFocused = ['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)

  if (e.key === 'Escape') {
    e.preventDefault()
    closeOverlay()
  } else if (!isInputFocused) {
    if ((e.key.toLowerCase() === 'y' && !e.ctrlKey) || (e.shiftKey && e.key.toUpperCase() === 'A')) {
      e.preventDefault()
      approveArtifact()
    } else if ((e.key.toLowerCase() === 'n' && !e.ctrlKey) || (e.shiftKey && e.key.toUpperCase() === 'R')) {
      e.preventDefault()
      rejectArtifact()
    } else if (e.key.toLowerCase() === 'c' && activeLineNumber.value !== null) {
      e.preventDefault()
      const el = document.getElementById('line-comment-input')
      if (el) el.focus()
    } else if (e.key.toLowerCase() === 'd' && activeLineNumber.value !== null) {
      e.preventDefault()
      deleteLineComment(activeLineNumber.value)
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Transition name="overlay-fade">
    <div v-if="orchestrator.showPlanOverlay" class="plan-overlay-parent">
      <div class="overlay-backdrop" @click="closeOverlay"></div>
      
      <div class="plan-modal glass-heavy">
        <!-- Cabeçalho com Informações e Tabs de Artefatos -->
        <header class="modal-header">
          <div class="header-main">
            <span class="plan-icon">📜</span>
            <div class="titles">
              <div class="title-row">
                <h3>CO-STEERING & REVISÃO DE ARTEFATOS</h3>
                <span class="badge-shortcut">Ctrl+R</span>
              </div>
              <p>Inspecione planos, diagramas Mermaid e anote diretrizes por linha antes da execução</p>
            </div>
          </div>

          <div class="header-controls">
            <!-- Alternador de visualização (Renderizado vs Linhas) -->
            <div class="view-mode-toggle">
              <button 
                :class="{ active: viewMode === 'rendered' }" 
                @click="viewMode = 'rendered'"
                title="Visualização Markdown e Diagramas"
              >
                📊 Renderizado
              </button>
              <button 
                :class="{ active: viewMode === 'lines' }" 
                @click="viewMode = 'lines'"
                title="Anotação e Comentários por Linha (c / d)"
              >
                ✍️ Linhas ({{ Object.keys(lineComments).length }})
              </button>
            </div>

            <button @click="closeOverlay" class="close-btn" title="Fechar (Esc)">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </header>

        <!-- Barra de Abas de Artefatos Descobertos -->
        <div class="artifacts-tabs-bar" v-if="artifacts.length > 0">
          <button 
            v-for="(art, idx) in artifacts" 
            :key="art.name"
            class="artifact-tab-btn"
            :class="{ active: activeArtifactIndex === idx }"
            @click="activeArtifactIndex = idx"
          >
            <span class="tab-icon">{{ art.isPlan ? '📋' : '📑' }}</span>
            <span class="tab-name">{{ art.name }}</span>
            <span class="tab-lines">{{ art.totalLines || art.content.split('\n').length }}L</span>
          </button>
        </div>

        <!-- Conteúdo do Artefato -->
        <section class="plan-content-scroll" ref="mermaidContainer">
          <!-- Modo 1: Markdown Renderizado com Diagramas Mermaid -->
          <div v-if="viewMode === 'rendered'" class="markdown-body" v-html="renderedMarkdown"></div>

          <!-- Modo 2: Visualização por Linhas com Comentários Cirúrgicos -->
          <div v-else class="lines-mode-container">
            <div class="lines-viewport">
              <div 
                v-for="(line, idx) in artifactLines" 
                :key="idx" 
                class="code-line-row"
                :class="{ 
                  selected: activeLineNumber === idx + 1,
                  has_comment: lineComments[idx + 1]
                }"
                @click="selectLine(idx + 1)"
              >
                <span class="line-num">{{ idx + 1 }}</span>
                <span class="line-content">{{ line || ' ' }}</span>
                <span v-if="lineComments[idx + 1]" class="line-comment-chip" title="Linha comentada">
                  💬 {{ lineComments[idx + 1] }}
                  <button class="chip-del-btn" @click.stop="deleteLineComment(idx + 1)" title="Remover comentário (d)">×</button>
                </span>
              </div>
            </div>

            <!-- Painel Lateral de Comentário para a Linha Selecionada -->
            <div class="line-comment-drawer glass" v-if="activeLineNumber !== null">
              <div class="drawer-header">
                <span>Comentário na Linha {{ activeLineNumber }}</span>
                <span class="drawer-shortcut">Pressione Enter para salvar</span>
              </div>
              <textarea 
                id="line-comment-input"
                v-model="newCommentText" 
                placeholder="Ex: Não altere esta assinatura pública; use valor default..."
                rows="3"
                class="comment-textarea"
                @keydown.enter.exact.prevent="saveLineComment"
                @keydown.esc="activeLineNumber = null"
              ></textarea>
              <div class="drawer-actions">
                <button class="btn-cancel-comment" @click="activeLineNumber = null">Cancelar</button>
                <button class="btn-save-comment" @click="saveLineComment">Salvar Comentário</button>
              </div>
            </div>
          </div>
        </section>

        <!-- Rodapé de Decisão Co-Steering -->
        <footer class="modal-footer glass">
          <div class="footer-input-box">
            <input 
              v-model="generalFeedback" 
              type="text" 
              placeholder="Diretriz geral para o agente (opcional)..." 
              class="general-feedback-input"
              @keydown.enter="rejectArtifact"
            />
          </div>

          <div class="footer-actions">
            <button 
              @click="rejectArtifact" 
              class="btn-reject" 
              :disabled="isSubmitting"
              title="Solicitar alterações e rejeitar (Atalho: n ou Shift+R)"
            >
              <span class="btn-icon">❌</span>
              <span>SOLICITAR AJUSTES (n)</span>
            </button>

            <button 
              @click="approveArtifact" 
              class="btn-approve" 
              :disabled="isSubmitting"
              title="Aprovar plano e iniciar execução (Atalho: y ou Shift+A)"
            >
              <span class="btn-icon">✅</span>
              <span>APROVAR & EXECUTAR (y)</span>
            </button>
          </div>
        </footer>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.plan-overlay-parent {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 30px;
}

.overlay-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 23, 0.88);
  backdrop-filter: blur(10px);
}

.plan-modal {
  position: relative;
  width: 100%;
  max-width: 1000px;
  height: 90vh;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 40px 100px rgba(0, 0, 0, 0.85);
}

.modal-header {
  padding: 18px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(15, 23, 42, 0.95);
}

.header-main { display: flex; align-items: center; gap: 14px; }
.plan-icon { font-size: 1.6rem; }

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.titles h3 {
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 1.5px;
  color: #f8fafc;
  margin: 0;
}

.badge-shortcut {
  font-size: 10px;
  font-weight: 700;
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.3);
  padding: 1px 6px;
  border-radius: 4px;
}

.titles p {
  font-size: 11px;
  color: #94a3b8;
  margin: 2px 0 0 0;
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 14px;
}

.view-mode-toggle {
  display: flex;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 8px;
  padding: 2px;
}

.view-mode-toggle button {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 600;
  padding: 5px 10px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}

.view-mode-toggle button.active {
  background: #3b82f6;
  color: #fff;
  box-shadow: 0 2px 8px rgba(59, 130, 246, 0.4);
}

.close-btn {
  background: transparent; border: none; color: #64748b; cursor: pointer;
  padding: 6px; border-radius: 8px; transition: all 0.2s;
}
.close-btn:hover { background: rgba(255, 255, 255, 0.08); color: #fff; }

/* Tabs de Artefatos */
.artifacts-tabs-bar {
  display: flex;
  gap: 6px;
  padding: 6px 18px;
  background: rgba(0, 0, 0, 0.3);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  overflow-x: auto;
}

.artifact-tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  color: #94a3b8;
  padding: 5px 12px;
  border-radius: 6px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.15s;
}

.artifact-tab-btn.active {
  background: rgba(59, 130, 246, 0.15);
  border-color: rgba(59, 130, 246, 0.4);
  color: #93c5fd;
  font-weight: 700;
}

.tab-lines {
  font-size: 9px;
  background: rgba(0, 0, 0, 0.4);
  padding: 1px 5px;
  border-radius: 4px;
  color: #64748b;
}

/* Área de Rolagem do Conteúdo */
.plan-content-scroll {
  flex: 1;
  padding: 24px;
  overflow-y: auto;
  background: rgba(0, 0, 0, 0.2);
}

.markdown-body {
  color: #cbd5e1;
  line-height: 1.7;
  font-size: 14px;
}

.markdown-body :deep(h1), .markdown-body :deep(h2) {
  color: #fff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  padding-bottom: 6px;
  margin-top: 20px;
}

.markdown-body :deep(pre.mermaid-diagram), .markdown-body :deep(div.mermaid-diagram) {
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 16px;
  margin: 16px 0;
  display: flex;
  justify-content: center;
  overflow-x: auto;
}

.markdown-body :deep(code) {
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.9em;
  color: #38bdf8;
}

/* Modo Linhas e Anotações */
.lines-mode-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.lines-viewport {
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  line-height: 1.8;
  background: #090d16;
  border-radius: 8px;
  padding: 10px 0;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.code-line-row {
  display: flex;
  align-items: flex-start;
  padding: 2px 14px;
  cursor: pointer;
  transition: background 0.12s;
  gap: 14px;
}

.code-line-row:hover {
  background: rgba(59, 130, 246, 0.08);
}

.code-line-row.selected {
  background: rgba(59, 130, 246, 0.22);
}

.code-line-row.has_comment {
  border-left: 3px solid #f59e0b;
  background: rgba(245, 158, 11, 0.08);
}

.line-num {
  width: 40px;
  color: #475569;
  text-align: right;
  user-select: none;
}

.line-content {
  flex: 1;
  color: #cbd5e1;
  white-space: pre-wrap;
  word-break: break-all;
}

.line-comment-chip {
  background: rgba(245, 158, 11, 0.2);
  color: #fcd34d;
  border: 1px solid rgba(245, 158, 11, 0.4);
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 10px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.chip-del-btn {
  background: transparent;
  border: none;
  color: #f87171;
  font-weight: bold;
  cursor: pointer;
  padding: 0 2px;
}

/* Gaveta de Comentário */
.line-comment-drawer {
  background: rgba(30, 41, 59, 0.95);
  border: 1px solid rgba(245, 158, 11, 0.3);
  border-radius: 12px;
  padding: 14px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
}

.drawer-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 11px;
  font-weight: 700;
  color: #fcd34d;
}

.drawer-shortcut {
  color: #94a3b8;
  font-size: 10px;
  font-weight: normal;
}

.comment-textarea {
  width: 100%;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 8px 12px;
  color: #f8fafc;
  font-size: 12px;
  outline: none;
  resize: vertical;
}

.drawer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 8px;
}

.btn-cancel-comment {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 11px;
  cursor: pointer;
}

.btn-save-comment {
  background: #f59e0b;
  border: none;
  color: #0f172a;
  font-weight: 700;
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 11px;
  cursor: pointer;
}

/* Rodapé */
.modal-footer {
  padding: 16px 24px;
  background: rgba(15, 23, 42, 0.95);
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
}

.footer-input-box {
  flex: 1;
}

.general-feedback-input {
  width: 100%;
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 9px 14px;
  color: #f8fafc;
  font-size: 12px;
  outline: none;
}

.general-feedback-input:focus {
  border-color: #3b82f6;
}

.footer-actions {
  display: flex;
  gap: 10px;
}

.btn-reject {
  background: rgba(239, 68, 68, 0.12);
  color: #fca5a5;
  border: 1px solid rgba(239, 68, 68, 0.3);
  padding: 9px 18px;
  border-radius: 10px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s;
}

.btn-reject:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.25);
  border-color: rgba(239, 68, 68, 0.6);
}

.btn-approve {
  background: #10b981;
  color: white;
  border: none;
  padding: 9px 20px;
  border-radius: 10px;
  font-weight: 700;
  font-size: 12px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  box-shadow: 0 4px 15px rgba(16, 185, 129, 0.35);
  transition: all 0.2s;
}

.btn-approve:hover:not(:disabled) {
  background: #059669;
  box-shadow: 0 6px 20px rgba(16, 185, 129, 0.5);
  transform: translateY(-1px);
}

/* Transições */
.overlay-fade-enter-active, .overlay-fade-leave-active {
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.overlay-fade-enter-from, .overlay-fade-leave-to {
  opacity: 0;
  backdrop-filter: blur(0);
}
.overlay-fade-enter-from .plan-modal { transform: scale(0.95) translateY(10px); }
.overlay-fade-leave-to .plan-modal { transform: scale(0.95) translateY(10px); }
</style>
