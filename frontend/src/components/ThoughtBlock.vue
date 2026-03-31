<script setup>
import { ref } from 'vue';

const props = defineProps({
  thought: {
    type: String,
    required: true
  },
  agent: {
    type: String,
    default: 'Gemini'
  }
});

const isOpen = ref(false);

const toggle = () => {
  isOpen.value = !isOpen.value;
};
</script>

<template>
  <div class="thought-wrapper" :class="{ 'is-open': isOpen }">
    <div class="thought-header" @click="toggle" title="Clique para ver o raciocínio">
      <div class="header-content">
        <span class="icon" :class="{ 'pulse': !isOpen }">🧠</span>
        <span class="label">Raciocínio do {{ agent }}...</span>
      </div>
      <div class="chevron-wrapper" :class="{ 'rotate': isOpen }">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </div>
    </div>
    
    <transition name="premium-expand">
      <div v-if="isOpen" class="thought-content scroll-shadows">
        <div class="content-inner markdown-body">
          {{ thought }}
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.thought-wrapper {
  margin: 12px 0 20px 0;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.07);
  overflow: hidden;
  transition: all 0.4s cubic-bezier(0.23, 1, 0.32, 1);
  max-width: 95%;
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  position: relative;
}

.thought-wrapper::before {
  content: '';
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  border-radius: 14px;
  padding: 1px;
  background: linear-gradient(135deg, rgba(255,255,255,0.1), transparent, rgba(255,255,255,0.05));
  -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  pointer-events: none;
  opacity: 0.5;
  transition: opacity 0.3s;
}

.thought-wrapper.is-open {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.12);
  box-shadow: 
    0 10px 30px -10px rgba(0, 0, 0, 0.5),
    0 0 20px rgba(59, 130, 246, 0.05);
}

.thought-wrapper.is-open::before {
  opacity: 1;
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.3), transparent, rgba(168, 85, 247, 0.3));
}

.thought-header {
  padding: 12px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  user-select: none;
  transition: background 0.3s;
}

.thought-header:hover {
  background: rgba(255, 255, 255, 0.04);
}

.header-content {
  display: flex;
  align-items: center;
  gap: 10px;
}

.label {
  font-size: 0.85rem;
  font-weight: 600;
  letter-spacing: 0.3px;
  background: linear-gradient(90deg, #94a3b8, #e2e8f0);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  opacity: 0.8;
}

.icon {
  font-size: 1.1rem;
  filter: drop-shadow(0 0 5px rgba(255, 255, 255, 0.2));
  display: inline-block;
}

@keyframes pulse {
  0% { transform: scale(1); opacity: 0.8; }
  50% { transform: scale(1.1); opacity: 1; }
  100% { transform: scale(1); opacity: 0.8; }
}

.icon.pulse {
  animation: pulse 2s infinite ease-in-out;
}

.chevron-wrapper {
  color: #94a3b8;
  transition: all 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.6;
}

.chevron-wrapper.rotate {
  transform: rotate(180deg);
  color: #60a5fa;
  opacity: 1;
}

.thought-content {
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  background: rgba(0, 0, 0, 0.15);
  max-height: 400px;
  overflow-y: auto;
}

.content-inner {
  padding: 16px 20px;
  font-size: 0.88rem;
  line-height: 1.7;
  color: rgba(226, 232, 240, 0.85);
  font-style: italic;
  white-space: pre-wrap;
  font-family: inherit;
}

/* Animações Premium */
.premium-expand-enter-active {
  animation: expandIn 0.5s cubic-bezier(0.23, 1, 0.32, 1);
}

.premium-expand-leave-active {
  animation: expandIn 0.35s cubic-bezier(0.23, 1, 0.32, 1) reverse;
}

@keyframes expandIn {
  from {
    max-height: 0;
    opacity: 0;
    transform: translateY(-10px) scale(0.98);
  }
  to {
    max-height: 400px;
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* Scrollbar Sutil */
.scroll-shadows::-webkit-scrollbar {
  width: 5px;
}
.scroll-shadows::-webkit-scrollbar-track {
  background: transparent;
}
.scroll-shadows::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.08);
  border-radius: 10px;
}
.scroll-shadows::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.15);
}
</style>
