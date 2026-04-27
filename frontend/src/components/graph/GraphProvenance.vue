<script setup>
import { useGraphStore } from '../../stores/graph'

const store = useGraphStore()
</script>

<template>
  <transition name="slide-fade">
    <aside v-if="store.selectedNode" class="provenance-panel glass">
      <header class="panel-header">
        <div class="header-content">
          <div class="source-icon">🔎</div>
          <h3>Proveniência</h3>
        </div>
        <button @click="store.closeDetails" class="close-btn">×</button>
      </header>

      <div class="panel-body">
        <div v-if="!store.nodeDetails || store.nodeDetails.loading" class="loading-provenance">
          <div class="spinner"></div>
          <span>Sintonizando Base...</span>
        </div>

        <div v-else class="details-content">
          <div class="provenance-metadata">
            <div class="meta-item">
              <span class="lab">DOCUMENTO ORIGEM</span>
              <div class="val-box">{{ store.nodeDetails?.path || 'Escaneando...' }}</div>
            </div>
            
            <div v-if="store.nodeDetails?.semanticNeighbors?.length" class="meta-item">
              <span class="lab">💎 SINAPSES RELACIONADAS (VÍNCULOS)</span>
              <div class="synapse-list">
                <button v-for="neighbor in store.nodeDetails.semanticNeighbors" 
                        :key="neighbor.id"
                        @click="store.graphInstance?.focusNodeById(neighbor.id)"
                        class="synapse-item glass">
                   <span class="syn-icon">🧠</span> {{ neighbor.name }}
                </button>
              </div>
            </div>

            <div class="meta-item">
              <span class="lab">TRECHO FUNDAMENTADO (CHUNK)</span>
              <div class="content-box glass">
                 {{ store.nodeDetails?.content || 'Aguardando recuperação semântica...' }}
              </div>
            </div>
          </div>

          <button v-if="store.nodeDetails && store.nodeDetails.path && !store.nodeDetails.isVirtual && store.nodeDetails.path !== 'Conceito Neural'" 
                  @click="store.openSource" class="open-btn premium-btn">
            ABRIR ARQUIVO FONTE ✨
          </button>
        </div>
      </div>
    </aside>
  </transition>
</template>

<style scoped>
.provenance-panel {
  position: absolute;
  top: 20px;
  left: 20px;
  width: 320px;
  max-height: calc(100vh - 120px);
  z-index: 1000;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.4);
}

.panel-header {
  padding: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.synapse-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 10px;
}

.synapse-item {
  width: 100%;
  text-align: left;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 0.85rem;
  color: #fff;
  border: 1px solid rgba(255, 255, 255, 0.1);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  gap: 10px;
}

.synapse-item:hover {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(0, 255, 159, 0.5); /* Verde Matrix Soft */
  transform: translateX(4px);
  box-shadow: 0 0 15px rgba(0, 255, 159, 0.2);
}

.syn-icon {
  font-size: 1rem;
  filter: drop-shadow(0 0 5px rgba(0, 184, 255, 0.8));
}

/* Glassmorphism redundante para garantir profundidade */
.glass {
  background: rgba(255, 255, 255, 0.05);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}
</style>
