<script setup>
import { ref } from 'vue'

const props = defineProps({
  isOpen: Boolean,
  title: String,
  message: String,
  confirmText: { type: String, default: 'CONFIRMAR' },
  cancelText: { type: String, default: 'ABORTAR' },
  type: { type: String, default: 'danger' } // 'danger', 'info', 'warning'
})

const emit = defineEmits(['confirm', 'cancel'])

const handleConfirm = () => emit('confirm')
const handleCancel = () => emit('cancel')
</script>

<template>
  <Transition name="modal-fade">
    <div v-if="isOpen" class="maestro-modal-overlay" @click.self="handleCancel">
      <div class="maestro-modal-card glass animate-modal-in" :class="'modal-' + type">
        <div class="modal-glow"></div>
        
        <div class="modal-header">
          <div class="modal-icon">
            <span v-if="type === 'danger'">🚫</span>
            <span v-else-if="type === 'warning'">⚠️</span>
            <span v-else>ℹ️</span>
          </div>
          <h2 class="modal-title">{{ title }}</h2>
        </div>

        <div class="modal-body">
          <p class="modal-message">{{ message }}</p>
        </div>

        <div class="modal-actions">
          <button class="btn-modal-cancel" @click="handleCancel">{{ cancelText }}</button>
          <button class="btn-modal-confirm" :class="'btn-' + type" @click="handleConfirm">
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.maestro-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 11000;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.maestro-modal-card {
  position: relative;
  width: 100%;
  max-width: 450px;
  background: rgba(13, 17, 23, 0.9);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 24px;
  padding: 32px;
  overflow: hidden;
  box-shadow: 0 25px 60px rgba(0, 0, 0, 0.8);
}

.modal-glow {
  position: absolute;
  top: -50%;
  left: -20%;
  width: 150px;
  height: 300px;
  background: radial-gradient(circle, var(--modal-color, #ef4444) 0%, transparent 70%);
  opacity: 0.1;
  filter: blur(40px);
  pointer-events: none;
}

.modal-danger { --modal-color: #ef4444; border-color: rgba(239, 68, 68, 0.2); }
.modal-warning { --modal-color: #f59e0b; border-color: rgba(245, 158, 11, 0.2); }
.modal-info { --modal-color: #3b82f6; border-color: rgba(59, 130, 246, 0.2); }

.modal-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  text-align: center;
}

.modal-icon {
  font-size: 2.5rem;
  background: rgba(255, 255, 255, 0.03);
  width: 80px;
  height: 80px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: inset 0 0 20px rgba(0, 0, 0, 0.5);
}

.modal-title {
  font-size: 1.4rem;
  font-weight: 900;
  color: #fff;
  letter-spacing: 1px;
  margin: 0;
}

.modal-body {
  margin-bottom: 32px;
  text-align: center;
}

.modal-message {
  font-size: 1rem;
  color: rgba(255, 255, 255, 0.7);
  line-height: 1.6;
  white-space: pre-wrap;
}

.modal-actions {
  display: flex;
  gap: 12px;
}

.btn-modal-cancel {
  flex: 1;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #94a3b8;
  padding: 14px;
  border-radius: 14px;
  font-weight: 800;
  font-size: 0.8rem;
  letter-spacing: 1px;
  cursor: pointer;
  transition: all 0.3s;
}

.btn-modal-cancel:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
}

.btn-modal-confirm {
  flex: 1.5;
  border: 1px solid transparent;
  color: #fff;
  padding: 14px;
  border-radius: 14px;
  font-weight: 900;
  font-size: 0.8rem;
  letter-spacing: 2px;
  cursor: pointer;
  transition: all 0.3s;
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.2);
}

.btn-danger { background: linear-gradient(135deg, #ef4444, #b91c1c); border-color: rgba(239, 68, 68, 0.4); }
.btn-warning { background: linear-gradient(135deg, #f59e0b, #d97706); border-color: rgba(245, 158, 11, 0.4); }
.btn-info { background: linear-gradient(135deg, #3b82f6, #1d4ed8); border-color: rgba(59, 130, 246, 0.4); }

.btn-modal-confirm:hover {
  transform: translateY(-2px);
  filter: brightness(1.2);
}

/* Animações */
.modal-fade-enter-active, .modal-fade-leave-active {
  transition: opacity 0.4s ease;
}
.modal-fade-enter-from, .modal-fade-leave-to {
  opacity: 0;
}

.animate-modal-in {
  animation: modalScaleIn 0.5s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes modalScaleIn {
  0% { transform: scale(0.8) translateY(20px); opacity: 0; }
  100% { transform: scale(1) translateY(0); opacity: 1; }
}
</style>
