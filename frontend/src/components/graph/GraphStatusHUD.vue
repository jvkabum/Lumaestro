<script setup>
import { useGraphStore } from '../../stores/graph'

const store = useGraphStore()
</script>

<template>
  <div class="hud-layer">
    <!-- 📊 FPS MONITOR (F1 para toggle) -->
    <Transition name="fade">
      <div v-if="store.showFps" class="fps-hud">
        <span class="fps-label">FPS</span>
        <span class="fps-value" :class="{
          'fps-good': store.currentFps >= 50,
          'fps-warn': store.currentFps >= 20 && store.currentFps < 50,
          'fps-bad': store.currentFps < 20
        }">{{ store.currentFps }}</span>
      </div>
    </Transition>

    <!-- 🧠 DISCOVERY STATUS (Efeito de Descoberta) -->
    <Transition name="slide-up">
      <div v-if="store.discoveryStatus" class="discovery-hud" :class="store.discoveryStatus">
        <div class="discovery-ring" v-if="store.discoveryStatus === 'searching'"></div>
        <span class="status-icon">
          {{ store.discoveryStatus === 'searching' ? '🔍' : store.discoveryStatus === 'found' ? '🎯' : '⚠️' }}
        </span>
        <span class="status-text">
          {{ store.discoveryStatus === 'searching' ? 'Localizando neurônio...' : 
             store.discoveryStatus === 'found' ? 'Alvo Identificado!' : 'Nó não renderizado' }}
        </span>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
.hud-layer {
  position: absolute;
  top: 1rem;
  left: 1rem;
  right: 1rem;
  pointer-events: none;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  z-index: 1000;
}

/* 🧊 Premium Glassmorphism Base */
.fps-hud, .discovery-hud {
  background: rgba(15, 15, 25, 0.6);
  backdrop-filter: blur(12px) saturate(180%);
  -webkit-backdrop-filter: blur(12px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.37);
  border-radius: 12px;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  width: fit-content;
  transition: all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

/* 📊 FPS Styles */
.fps-label {
  font-family: 'Inter', sans-serif;
  font-size: 0.7rem;
  font-weight: 800;
  color: rgba(255, 255, 255, 0.5);
  letter-spacing: 1px;
}

.fps-value {
  font-family: 'JetBrains Mono', monospace;
  font-size: 1rem;
  font-weight: 700;
}

.fps-good { color: #00ffaa; text-shadow: 0 0 10px rgba(0, 255, 170, 0.5); }
.fps-warn { color: #ffcc00; text-shadow: 0 0 10px rgba(255, 204, 0, 0.5); }
.fps-bad  { color: #ff3366; text-shadow: 0 0 10px rgba(255, 51, 102, 0.5); }

/* 🧠 Discovery Styles */
.discovery-hud {
  position: fixed;
  bottom: 2rem;
  left: 50%;
  transform: translateX(-50%);
  padding: 12px 24px;
}

.status-icon { font-size: 1.2rem; }
.status-text {
  font-family: 'Outfit', sans-serif;
  font-weight: 500;
  color: #fff;
  letter-spacing: 0.5px;
}

/* 🌀 Animação de Ring (Searching) */
.discovery-ring {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(0, 255, 255, 0.2);
  border-top: 2px solid #00f3ff;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

/* 🌓 Transições */
.fade-enter-active, .fade-leave-active { transition: opacity 0.5s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.slide-up-enter-active { transition: all 0.5s cubic-bezier(0.175, 0.885, 0.32, 1.275); }
.slide-up-leave-active { transition: all 0.4s ease-in; }
.slide-up-enter-from { transform: translate(-50%, 100px); opacity: 0; }
.slide-up-leave-to { transform: translate(-50%, 100px); opacity: 0; }

/* Discovery Status States */
.searching { border-color: rgba(0, 243, 255, 0.4); box-shadow: 0 0 20px rgba(0, 243, 255, 0.2); }
.found { border-color: rgba(0, 255, 170, 0.4); box-shadow: 0 0 25px rgba(0, 255, 170, 0.3); }
.failed { border-color: rgba(255, 51, 102, 0.4); }
</style>
