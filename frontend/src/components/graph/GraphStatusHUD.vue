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
  </div>
</template>
