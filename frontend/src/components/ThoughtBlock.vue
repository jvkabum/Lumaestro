<script setup>
import { ref, watch, computed } from 'vue';

const props = defineProps({
  thought: {
    type: String,
    required: true
  },
  agent: {
    type: String,
    default: 'Gemini'
  },
  isStreaming: {
    type: Boolean,
    default: false
  }
});

// Auto-abre quando o streaming começa, fecha (suavemente) quando termina
const isOpen = ref(false);
const wasStreaming = ref(false);

watch(() => props.isStreaming, (streaming) => {
  if (streaming && !wasStreaming.value) {
    isOpen.value = true; // abre automaticamente
  }
  wasStreaming.value = streaming;
}, { immediate: true });

const toggle = () => {
  isOpen.value = !isOpen.value;
};

// Contador de chars do pensamento para dar sensação de progresso vivo
const charCount = computed(() => props.thought?.length ?? 0);

const displayCount = computed(() => {
  if (charCount.value > 1000) return `~${Math.floor(charCount.value / 100) / 10}k chars`;
  return `${charCount.value} chars`;
});
</script>

<template>
  <div class="thought-wrapper" :class="{ 'is-open': isOpen, 'is-live': isStreaming }">
    <div class="thought-header" @click="toggle" title="Clique para ver o raciocínio">
      <div class="header-content">
        <span class="icon" :class="{ 'pulse': isStreaming }">🧠</span>
        <div class="header-text">
          <span class="label">Raciocínio do {{ agent }}</span>
          <span v-if="isStreaming" class="live-badge">
            <span class="live-dot"></span>
            AO VIVO · {{ displayCount }}
          </span>
          <span v-else-if="charCount > 0" class="done-badge">{{ displayCount }}</span>
        </div>
      </div>
      <div class="chevron-wrapper" :class="{ 'rotate': isOpen }">
        <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </div>
    </div>
    
    <transition name="premium-expand">
      <div v-if="isOpen" class="thought-content scroll-shadows">
        <div class="content-inner" :class="{ 'streaming-active': isStreaming }">
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

/* Glow roxo pulsante enquanto o raciocínio está ativo */
.thought-wrapper.is-live {
  border-color: rgba(168, 85, 247, 0.35);
  box-shadow:
    0 0 0 1px rgba(168, 85, 247, 0.1),
    0 0 20px rgba(168, 85, 247, 0.08),
    0 4px 20px rgba(0, 0, 0, 0.3);
  animation: live-pulse 2.5s infinite ease-in-out;
}

@keyframes live-pulse {
  0%, 100% { box-shadow: 0 0 0 1px rgba(168, 85, 247, 0.1), 0 0 20px rgba(168, 85, 247, 0.05); }
  50%       { box-shadow: 0 0 0 1px rgba(168, 85, 247, 0.3), 0 0 30px rgba(168, 85, 247, 0.15); }
}

.thought-wrapper.is-open {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(168, 85, 247, 0.2);
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

.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.label {
  font-size: 0.85rem;
  font-weight: 600;
  letter-spacing: 0.3px;
  background: linear-gradient(90deg, #a78bfa, #e2e8f0);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Badge "AO VIVO" */
.live-badge {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 9px;
  font-weight: 800;
  letter-spacing: 1px;
  color: #c084fc;
  text-transform: uppercase;
}

.live-dot {
  width: 5px;
  height: 5px;
  background: #c084fc;
  border-radius: 50%;
  animation: dot-blink 1s infinite;
  box-shadow: 0 0 6px #c084fc;
}

@keyframes dot-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.2; }
}

.done-badge {
  font-size: 9px;
  font-weight: 600;
  color: #64748b;
  letter-spacing: 0.5px;
}

.icon {
  font-size: 1.1rem;
  filter: drop-shadow(0 0 5px rgba(168, 85, 247, 0.3));
  display: inline-block;
  flex-shrink: 0;
}

@keyframes pulse {
  0%   { transform: scale(1)    rotate(0deg);   opacity: 0.8; }
  25%  { transform: scale(1.15) rotate(-5deg);  opacity: 1;   }
  75%  { transform: scale(1.15) rotate(5deg);   opacity: 1;   }
  100% { transform: scale(1)    rotate(0deg);   opacity: 0.8; }
}

.icon.pulse {
  animation: pulse 1.8s infinite ease-in-out;
}

.chevron-wrapper {
  color: #94a3b8;
  transition: all 0.4s cubic-bezier(0.34, 1.56, 0.64, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0.6;
  flex-shrink: 0;
}

.chevron-wrapper.rotate {
  transform: rotate(180deg);
  color: #a78bfa;
  opacity: 1;
}

.thought-content {
  border-top: 1px solid rgba(168, 85, 247, 0.1);
  background: rgba(0, 0, 0, 0.15);
  max-height: 420px;
  overflow-y: auto;
}

.content-inner {
  padding: 16px 20px;
  font-size: 0.88rem;
  line-height: 1.75;
  color: rgba(196, 181, 253, 0.85);
  font-style: italic;
  white-space: pre-wrap;
  font-family: inherit;
}

.content-inner.streaming-active::after {
  content: "▋";
  display: inline-block;
  vertical-align: text-bottom;
  animation: txt-blink 0.8s step-end infinite;
  color: #c084fc;
  margin-left: 5px;
  text-shadow: 0 0 10px rgba(192, 132, 252, 0.8);
}

@keyframes txt-blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}

/* Animações Premium */
.premium-expand-enter-active {
  animation: expandIn 0.45s cubic-bezier(0.23, 1, 0.32, 1);
}

.premium-expand-leave-active {
  animation: expandIn 0.3s cubic-bezier(0.23, 1, 0.32, 1) reverse;
}

@keyframes expandIn {
  from { max-height: 0; opacity: 0; transform: translateY(-8px); }
  to   { max-height: 420px; opacity: 1; transform: translateY(0); }
}

/* Scrollbar sutil */
.scroll-shadows::-webkit-scrollbar { width: 4px; }
.scroll-shadows::-webkit-scrollbar-track { background: transparent; }
.scroll-shadows::-webkit-scrollbar-thumb { background: rgba(168, 85, 247, 0.15); border-radius: 10px; }
.scroll-shadows::-webkit-scrollbar-thumb:hover { background: rgba(168, 85, 247, 0.3); }
</style>
