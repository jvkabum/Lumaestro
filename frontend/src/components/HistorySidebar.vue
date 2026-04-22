<script setup>
import { computed, onMounted, watch, ref } from 'vue';
import { useOrchestratorStore } from '../stores/orchestrator';

const store = useOrchestratorStore();

const formatRelativeTime = (dateStr) => {
  if (!dateStr) return 'Sem data';
  const date = new Date(dateStr);
  const now = new Date();
  const diffInSeconds = Math.floor((now - date) / 1000);

  if (diffInSeconds < 60) return 'Agora mesmo';
  if (diffInSeconds < 3600) return `${Math.floor(diffInSeconds / 60)}m atrás`;
  if (diffInSeconds < 86400) return `${Math.floor(diffInSeconds / 3600)}h atrás`;
  if (diffInSeconds < 604800) return `${Math.floor(diffInSeconds / 86400)}d atrás`;
  
  return date.toLocaleDateString();
};

const handleNewSession = async () => {
  if (store.activeAgent) {
    await store.newSession(store.activeAgent);
  }
};

const handleLoadSession = async (sessionId) => {
  if (store.activeAgent) {
    await store.loadSession(store.activeAgent, sessionId);
  }
};

const sessionToDelete = ref(null);

const handleDelete = (session) => {
  sessionToDelete.value = session;
};

const confirmDelete = async () => {
  const session = sessionToDelete.value;
  if (!session) return;
  
  sessionToDelete.value = null; // Fecha o modal imediatamente
  
  try {
    // 🚀 Chama o backend via Wails bridge
    await window.go.core.App.DeleteSession(session.file);
  } catch (err) {
    console.error("Erro ao apagar sessão:", err);
    // Se o erro for que o arquivo não existe, ignora graciosamente (já foi apagado)
    if (!String(err).includes("não pode encontrar") && !String(err).includes("no such file")) {
       // Poderíamos ter um toast aqui, por hora vamos apenas registrar
       console.warn("⚠️ Não foi possível apagar: " + err);
    }
  } finally {
    // Força a recarga visual garantindo que a lixeira limpe a visualização
    await store.fetchSessions(store.activeAgent);
    
    // Se a conversa apagada for a que estava aberta, inicia uma nova tela limpa
    if (session.sessionId === store.currentACPID) {
       store.currentACPID = null;
       store.messages = [];
       // Opcionalmente podemos disparar a criação no backend também
       await store.newSession(store.activeAgent);
    }
  }
};

onMounted(async () => {
  if (store.activeAgent) {
    await store.fetchSessions(store.activeAgent);
  }
});

watch(() => store.activeAgent, async (newAgent) => {
  if (newAgent) {
    await store.fetchSessions(newAgent);
  }
});
</script>

<template>
  <aside class="history-sidebar glass">
    <div class="sidebar-header">
      <h2 class="title">Sinfonias</h2>
      <button @click="handleNewSession" class="new-btn" title="Nova Sinfonia">
        <span class="icon">+</span>
      </button>
    </div>

    <div class="sessions-list scroll-shadows">
      <!-- Estado de Carregamento (Skeleton Shimmer) -->
      <template v-if="store.isThinking && store.sessions.length === 0">
        <div v-for="i in 5" :key="i" class="skeleton-item shimmer">
          <div class="skeleton-line title"></div>
          <div class="skeleton-line meta"></div>
        </div>
      </template>

      <div v-else-if="store.sessions.length === 0" class="empty-state">
        Nenhuma sinfonia gravada ainda.
      </div>
      
      <div 
        v-for="session in store.sessions" 
        :key="session.sessionId"
        class="session-item"
        :class="{ active: store.currentACPID === session.sessionId }"
        @click="handleLoadSession(session.sessionId)"
      >
        <div class="session-info">
          <div class="session-title">{{ session.title || 'Conversa sem título' }}</div>
          <div class="session-meta">
            <span class="id-badge">{{ session.sessionId.substring(0, 8) }}</span>
            <span class="time">{{ formatRelativeTime(session.updatedAt) }}</span>
          </div>
        </div>
        
        <!-- Botão de Apagar (Lixeira Premium) -->
        <button 
          class="delete-btn" 
          @click.stop="handleDelete(session)"
          title="Apagar Sinfonia Permanente"
        >
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="3 6 5 6 21 6"></polyline>
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"></path>
          </svg>
        </button>
      </div>
    </div>

    <div class="sidebar-footer">
      <div class="agent-badge" v-if="store.activeAgent">
        <span class="pulse-dot"></span>
        {{ store.activeAgent.toUpperCase() }} ON
      </div>
    </div>

    <!-- Modal Customizado -->
    <Teleport to="body">
      <Transition name="modal-fade">
        <div v-if="sessionToDelete" class="custom-modal-overlay" @click.self="sessionToDelete = null">
          <div class="custom-modal">
            <div class="modal-icon">⚠️</div>
            <h3 class="modal-title">Apagar Sinfonia</h3>
            <p class="modal-text">
              Deseja apagar permanentemente a conversa 
              <strong class="highlight-id">"{{ sessionToDelete?.title || 'sem título' }}"</strong>?
            </p>
            <p class="modal-subtext">Esta ação apagará todo o histórico e não pode ser desfeita.</p>
            
            <div class="modal-actions">
              <button class="btn-cancel" @click="sessionToDelete = null">Cancelar</button>
              <button class="btn-confirm" @click="confirmDelete">Sim, apagar</button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

  </aside>
</template>

<style scoped>
.history-sidebar {
  width: 250px;
  height: calc(100vh - 40px);
  margin: 20px 0 20px 0;
  display: flex;
  flex-direction: column;
  border-radius: 12px;
  background: rgba(13, 17, 23, 0.4);
  backdrop-filter: blur(8px);
  border-right: 1px solid rgba(255, 255, 255, 0.03);
  overflow: hidden;
  flex-shrink: 0;
  transition: all 0.3s ease;
}

.sidebar-header {
  padding: 16px 20px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.title {
  font-size: 11px;
  font-weight: 500;
  color: rgba(139, 148, 158, 0.6);
  letter-spacing: 1px;
  text-transform: uppercase;
  margin: 0;
}

.new-btn {
  width: 24px;
  height: 24px;
  border-radius: 4px;
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.05);
  color: rgba(139, 148, 158, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
}

.new-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: #fff;
  border-color: rgba(255, 255, 255, 0.2);
}

.icon {
  font-size: 16px;
  line-height: 1;
}

.sessions-list {
  flex: 1;
  overflow-y: auto;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
}

.empty-state {
  padding: 40px 20px;
  text-align: center;
  font-size: 12px;
  color: rgba(139, 148, 158, 0.4);
  font-style: italic;
}

.session-item {
  position: relative; /* 📌 Ancora o botão de delete para cada chat individualmente */
  padding: 10px 12px;
  margin-bottom: 4px;
  border-radius: 6px;
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.2s;
}

.session-item:hover {
  background: rgba(255, 255, 255, 0.03);
}

.session-item.active {
  background: rgba(30, 41, 59, 0.4);
  border-color: rgba(56, 189, 248, 0.1);
}

.session-title {
  font-size: 13px;
  font-weight: 400;
  color: rgba(240, 246, 252, 0.85);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  margin-bottom: 2px;
}

.session-item.active .session-title {
  color: #38bdf8;
}

.session-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.id-badge {
  font-family: 'Inter', sans-serif;
  font-size: 9px;
  color: rgba(139, 148, 158, 0.4);
  background: rgba(255, 255, 255, 0.03);
  padding: 1px 4px;
  border-radius: 3px;
}

.time {
  font-size: 10px;
  color: rgba(139, 148, 158, 0.5);
}

/* --- Botão Delete Premium --- */
.delete-btn {
  position: absolute;
  right: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 26px;
  height: 26px;
  border-radius: 6px;
  background: rgba(244, 63, 94, 0.1);
  border: 1px solid rgba(244, 63, 94, 0.2);
  color: #f43f5e;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  opacity: 0;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  backdrop-filter: blur(4px);
}

.session-item:hover .delete-btn {
  opacity: 1;
}

.delete-btn:hover {
  background: #f43f5e;
  color: #fff;
  transform: translateY(-50%) scale(1.1);
  box-shadow: 0 0 15px rgba(244, 63, 94, 0.4);
}

.sidebar-footer {
  padding: 12px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.03);
  background: rgba(0, 0, 0, 0.1);
}

.agent-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.5px;
  color: rgba(139, 148, 158, 0.6);
}

.pulse-dot {
  width: 5px;
  height: 5px;
  background: #238636;
  border-radius: 50%;
  box-shadow: 0 0 6px rgba(35, 134, 54, 0.3);
}

/* Custom Scrollbar */
.sessions-list::-webkit-scrollbar {
  width: 4px;
}

.sessions-list::-webkit-scrollbar-track {
  background: transparent;
}

.sessions-list::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 10px;
}

/* --- Skeleton Shimmer Animation --- */
.shimmer {
  position: relative;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.03) !important;
}

.shimmer::after {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  transform: translateX(-100%);
  background-image: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0,
    rgba(255, 255, 255, 0.03) 20%,
    rgba(255, 255, 255, 0.06) 60%,
    rgba(255, 255, 255, 0)
  );
  animation: shimmer-anim 2s infinite;
}

@keyframes shimmer-anim {
  100% {
    transform: translateX(100%);
  }
}

.skeleton-item {
  padding: 12px;
  margin-bottom: 8px;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.skeleton-line {
  height: 10px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.05);
}

.skeleton-line.title {
  width: 70%;
}

.skeleton-line.meta {
  width: 40%;
  height: 6px;
}

/* --- Modal Premium Customizado --- */
.custom-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.custom-modal {
  background: rgba(22, 27, 34, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  width: 400px;
  max-width: 90vw;
  padding: 30px;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.modal-icon {
  font-size: 32px;
  margin-bottom: 20px;
  background: rgba(244, 63, 94, 0.1);
  width: 64px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1px solid rgba(244, 63, 94, 0.2);
  color: #f43f5e;
  box-shadow: 0 0 20px rgba(244, 63, 94, 0.15);
}

.modal-title {
  font-size: 18px;
  font-weight: 600;
  color: #f0f6fc;
  margin: 0 0 12px 0;
  letter-spacing: -0.5px;
}

.modal-text {
  font-size: 14px;
  color: rgba(139, 148, 158, 0.9);
  line-height: 1.5;
  margin: 0 0 8px 0;
}

.modal-subtext {
  font-size: 12px;
  color: rgba(244, 63, 94, 0.7);
  margin: 0 0 24px 0;
  font-style: italic;
}

.highlight-id {
  color: #38bdf8;
  font-weight: 500;
}

.modal-actions {
  display: flex;
  gap: 12px;
  width: 100%;
}

.btn-cancel,
.btn-confirm {
  flex: 1;
  padding: 12px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.btn-cancel {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: rgba(240, 246, 252, 0.8);
}

.btn-cancel:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.btn-confirm {
  background: linear-gradient(180deg, #f43f5e 0%, #e11d48 100%);
  border: 1px solid #be123c;
  color: white;
  box-shadow: 0 2px 10px rgba(225, 29, 72, 0.3);
}

.btn-confirm:hover {
  background: linear-gradient(180deg, #fb7185 0%, #f43f5e 100%);
  box-shadow: 0 4px 15px rgba(225, 29, 72, 0.5);
  transform: translateY(-1px);
}

/* Transição do Modal */
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.3s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

.modal-fade-enter-active .custom-modal {
  transition: transform 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

.modal-fade-enter-from .custom-modal {
  transform: scale(0.9);
}
</style>
