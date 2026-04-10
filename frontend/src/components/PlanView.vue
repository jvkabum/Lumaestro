<script setup>
import { computed, ref, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useOrchestratorStore } from '../stores/orchestrator'
import MarkdownIt from 'markdown-it'

const orchestrator = useOrchestratorStore()
const { messages, activeAgent } = storeToRefs(orchestrator)
const md = new MarkdownIt({ html: true, linkify: true, typographer: true })

// 🔍 Encontra a última mensagem de plano (assistant) para exibir no overlay
const currentPlan = computed(() => {
  const assistantMsgs = messages.value.filter(m => m.role === 'assistant' && !m.mode)
  if (assistantMsgs.length === 0) return { text: '# Plano não encontrado\nAguardando o Maestro gerar a estratégia...' }
  return assistantMsgs[assistantMsgs.length - 1]
})

const renderedPlan = computed(() => md.render(currentPlan.value.text))

// --- Ações ---
const approvePlan = () => {
    orchestrator.showPlanOverlay = false
    orchestrator.isPlanMode = false // Sai do modo leitura ao aprovar
    orchestrator.pushStatus("Plano aprovado! Iniciando execução...", "status")
}

const iteratePlan = () => {
    orchestrator.showPlanOverlay = false
    // O foco volta para o input para o usuário digitar o feedback
}

const closeOverlay = () => {
    orchestrator.showPlanOverlay = false
}
</script>

<template>
  <Transition name="overlay-fade">
    <div v-if="orchestrator.showPlanOverlay" class="plan-overlay-parent">
      <div class="overlay-backdrop" @click="closeOverlay"></div>
      
      <div class="plan-modal glass-heavy">
        <header class="modal-header">
          <div class="header-main">
            <span class="plan-icon">📋</span>
            <div class="titles">
              <h3>ESTRATÉGIA DE EXECUÇÃO</h3>
              <p>Revisão de Plano pelo Maestro</p>
            </div>
          </div>
          <button @click="closeOverlay" class="close-btn">
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5">
              <line x1="18" y1="6" x2="6" y2="18"></line>
              <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
          </button>
        </header>

        <section class="plan-content-scroll">
          <div class="markdown-body" v-html="renderedPlan"></div>
        </section>

        <footer class="modal-footer glass">
          <div class="footer-hint">
            <p>O <strong>Plano</strong> define os passos lógicos. Aprovar iniciará a execução das ferramentas.</p>
          </div>
          <div class="footer-actions">
            <button @click="iteratePlan" class="btn-secondary">
              <span class="btn-icon">🔄</span> FEEDBACK
            </button>
            <button @click="approvePlan" class="btn-primary">
              <span class="btn-icon">✅</span> APROVAR E EXECUTAR
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
  padding: 40px;
}

.overlay-backdrop {
  position: absolute;
  inset: 0;
  background: rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(8px);
}

.plan-modal {
  position: relative;
  width: 100%;
  max-width: 900px;
  height: 100%;
  max-height: 80vh;
  background: #1e293b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 40px 100px rgba(0, 0, 0, 0.8);
}

.modal-header {
  padding: 24px 32px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-main { display: flex; align-items: center; gap: 16px; }
.plan-icon { font-size: 1.8rem; }

.titles h3 {
  font-size: 14px;
  font-weight: 900;
  letter-spacing: 2px;
  color: #fff;
  margin: 0;
}

.titles p {
  font-size: 12px;
  color: #94a3b8;
  margin: 2px 0 0 0;
}

.close-btn {
  background: transparent; border: none; color: #64748b; cursor: pointer;
  padding: 8px; border-radius: 12px; transition: all 0.2s;
}
.close-btn:hover { background: rgba(255, 255, 255, 0.05); color: #fff; }

.plan-content-scroll {
  flex: 1;
  padding: 32px;
  overflow-y: auto;
  background: rgba(0, 0, 0, 0.1);
}

.markdown-body {
  color: #cbd5e1;
  line-height: 1.7;
  font-size: 15px;
}

.markdown-body :deep(h1), .markdown-body :deep(h2) {
  color: #fff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  padding-bottom: 8px;
  margin-top: 24px;
}

.markdown-body :deep(ul) { padding-left: 20px; }
.markdown-body :deep(li) { margin-bottom: 8px; }
.markdown-body :deep(code) {
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.9em;
  color: #38bdf8;
}

.modal-footer {
  padding: 24px 32px;
  background: rgba(15, 23, 42, 0.6);
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.footer-hint p {
  font-size: 12px;
  color: #64748b;
  max-width: 300px;
  margin: 0;
}

.footer-actions { display: flex; gap: 12px; }

.btn-primary {
  background: #3b82f6;
  color: white;
  border: none;
  padding: 12px 24px;
  border-radius: 12px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  box-shadow: 0 4px 15px rgba(59, 130, 246, 0.4);
  transition: all 0.2s;
}

.btn-primary:hover {
  transform: translateY(-2px);
  background: #2563eb;
  box-shadow: 0 6px 20px rgba(59, 130, 246, 0.6);
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.05);
  color: #94a3b8;
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 12px 24px;
  border-radius: 12px;
  font-weight: 700;
  font-size: 13px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.2s;
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.btn-icon { font-size: 14px; }

/* Transições */
.overlay-fade-enter-active, .overlay-fade-leave-active {
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
}
.overlay-fade-enter-from, .overlay-fade-leave-to {
  opacity: 0;
  backdrop-filter: blur(0);
}
.overlay-fade-enter-from .plan-modal { transform: scale(0.9) translateY(20px); }
.overlay-fade-leave-to .plan-modal { transform: scale(0.9) translateY(20px); }
</style>
