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
