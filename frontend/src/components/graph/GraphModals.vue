<script setup>
import { useGraphStore } from '../../stores/graph'
import { useGraphActions } from '../../composables/deck/useGraphActions'

const props = defineProps({
  nodesCount: { type: Number, default: 0 }
})

const store = useGraphStore()
const { confirmSync, resolveConflict } = useGraphActions()
</script>

<template>
  <div class="modals-layer">
    <!-- MODAL DE CONFIRMAÇÃO DINÂMICO -->
    <div v-if="store.showConfirmModal" class="premium-modal-overlay">
      <div class="premium-modal-content">
        <div class="modal-icon">{{ store.modalMode === 'full' ? '⚙️' : '🚀' }}</div>
        <h3 class="modal-title">{{ store.modalMode === 'full' ? 'Reindexação Forçada' : 'Sincronização Inteligente' }}</h3>
        
        <div class="modal-body">
          <p v-if="store.modalMode === 'full'" class="modal-text">
            Deseja forçar uma varredura completa de todos os <strong>{{ store.graphHealth.active_nodes || nodesCount }} arquivos</strong>?<br/>
            <span class="warning-sub">Isso reconstrói o cache de auditoria e garante 100% de integridade. Use apenas se notar dados faltando.</span>
          </p>
          <p v-else class="modal-text">
            Deseja iniciar a sincronização incremental?<br/>
            <span class="info-sub">O Maestro buscará apenas notas <strong>novas ou modificadas</strong>. É o método mais rápido e econômico.</span>
          </p>
        </div>

        <div class="modal-actions">
           <button @click="store.showConfirmModal = false" class="btn-cancel">CANCELAR</button>
           <button @click="confirmSync" class="btn-confirm" :class="store.modalMode">
             {{ store.modalMode === 'full' ? 'INICIAR FAXINA' : 'SINCRONIZAR AGORA' }}
           </button>
        </div>
      </div>
    </div>

    <!-- POP-UP DE VALIDAÇÃO (AGENTE DA VERDADE) -->
    <div v-if="store.currentConflict" class="conflict-overlay">
      <div class="conflict-modal glass">
        <div class="conflict-header">
          <span class="alert-icon">⚠️</span>
          <h4>Contradição Semântica</h4>
        </div>
        <p>A IA detectou uma divergência sobre <b>{{ store.currentConflict.subject }}</b>:</p>
        <div class="conflict-options">
          <div class="opt old" @click="resolveConflict('old')">
            <span class="lab">PASSADO</span>
            <span class="val">{{ store.currentConflict.old }}</span>
          </div>
          <div class="opt new" @click="resolveConflict('new')">
            <span class="lab">PRESENTE</span>
            <span class="val">{{ store.currentConflict.new }}</span>
          </div>
        </div>
        <p class="hint">Escolha a verdade ativa. A outra será marcada como legado.</p>
      </div>
    </div>
  </div>
</template>
