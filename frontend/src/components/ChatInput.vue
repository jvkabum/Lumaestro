<template>
  <div class="chat-input-container">
    <div class="chat-input-wrapper glass">
      <!-- Toolbar Premium -->
      <div class="input-toolbar">
        <div class="toolbar-left">
          <div class="agent-switcher">
            <button 
              type="button" 
              :class="{ active: selectedAgent === 'gemini' }" 
              @click="selectedAgent = 'gemini'"
            >
              <span class="dot gemini"></span> Gemini
            </button>
            <button 
              type="button" 
              :class="{ active: selectedAgent === 'claude' }" 
              @click="selectedAgent = 'claude'"
            >
              <span class="dot claude"></span> Claude
            </button>
            <button 
              type="button" 
              :class="{ active: selectedAgent === 'lmstudio' }" 
              @click="selectedAgent = 'lmstudio'"
            >
              <span class="dot lmstudio"></span> LM Studio
            </button>
          </div>
        </div>

        <div class="toolbar-right">
          <!-- Toggle Modo Autônomo Premium -->
          <div class="safety-toggle" @click="isAutonomous = !isAutonomous; toggleAutonomous()">
            <span class="toggle-label">Autônomo</span>
            <div class="switch" :class="{ on: isAutonomous }">
              <div class="handle"></div>
            </div>
          </div>

          <div class="divider"></div>

          <!-- Mode Toggle (Act/Chat) -->
          <div class="mode-pills">
            <button 
              type="button" 
              :class="{ active: mode === 'act' }" 
              @click="mode = 'act'"
            >Act</button>
            <button 
              type="button" 
              :class="{ active: mode === 'chat' }" 
              @click="mode = 'chat'"
            >Chat</button>
          </div>
        </div>
      </div>

      <!-- Previews de Imagem (Miniaturas) -->
      <div v-if="attachedImages.length > 0" class="image-previews-container">
        <div v-for="(img, idx) in attachedImages" :key="idx" class="image-preview-card">
          <img :src="img.preview" />
          <button class="remove-img" @click="removeImage(idx)">×</button>
        </div>
      </div>

      <!-- Área de Texto e Enviar -->
      <div class="textarea-section">
        <textarea
          ref="textarea"
          v-model="messageText"
          placeholder="Comande o Maestro para construir algo extraordinário..."
          @keydown.enter.prevent="handleEnter"
          @paste="handlePaste"
          :disabled="isThinking"
          :rows="1"
        ></textarea>
        
        <div class="actions">
          <button 
            class="send-btn" 
            :disabled="(!messageText.trim() && attachedImages.length === 0) || isThinking"
            @click="sendMessage"
            :class="{ ready: (messageText.trim() || attachedImages.length > 0) && !isThinking }"
          >
            <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M7 11L12 6L17 11M12 18V7" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick, onMounted } from 'vue';

const messageText = ref('');
const selectedAgent = ref('gemini');
const mode = ref('act');
const textarea = ref(null);
const isAutonomous = ref(false);
const attachedImages = ref([]); // [{ preview, base64, type }]
const AGENT_STORAGE_KEY = 'lumaestro.chat.agent';
const MODE_STORAGE_KEY = 'lumaestro.chat.mode';

const props = defineProps({
  isThinking: { type: Boolean, default: false }
});

const emit = defineEmits(['send']);

const syncLocalState = () => {
  localStorage.setItem(AGENT_STORAGE_KEY, selectedAgent.value);
  localStorage.setItem(MODE_STORAGE_KEY, mode.value);
};

onMounted(async () => {
  const savedAgent = localStorage.getItem(AGENT_STORAGE_KEY);
  const savedMode = localStorage.getItem(MODE_STORAGE_KEY);

  if (savedAgent === 'gemini' || savedAgent === 'claude' || savedAgent === 'lmstudio') {
    selectedAgent.value = savedAgent;
  }
  if (savedMode === 'act' || savedMode === 'chat') {
    mode.value = savedMode;
  }

  if (window.go?.main?.App?.GetAutonomousMode) {
    try {
      isAutonomous.value = await window.go.main.App.GetAutonomousMode();
    } catch (err) {
      console.warn('[ChatInput] Falha ao sincronizar modo autônomo:', err);
    }
  }
});

const handlePaste = async (e) => {
  const items = (e.clipboardData || e.originalEvent.clipboardData).items;
  for (const item of items) {
    if (item.type.indexOf('image') !== -1) {
      const file = item.getAsFile();
      const reader = new FileReader();
      reader.onload = (event) => {
        attachedImages.value.push({
          preview: event.target.result,
          base64: event.target.result.split(',')[1],
          type: file.type
        });
      };
      reader.readAsDataURL(file);
    }
  }
};

const removeImage = (idx) => {
  attachedImages.value.splice(idx, 1);
};

const toggleAutonomous = async () => {
  if (window.go && window.go.main && window.go.main.App) {
    try {
      await window.go.main.App.SetAutonomousMode(isAutonomous.value);
    } catch (err) {
      isAutonomous.value = !isAutonomous.value;
      console.error('[ChatInput] Falha ao alterar modo autônomo:', err);
    }
  }
};

const adjustHeight = () => {
  if (!textarea.value) return;
  textarea.value.style.height = 'auto';
  textarea.value.style.height = (textarea.value.scrollHeight) + 'px';
};

watch(messageText, () => {
  nextTick(adjustHeight);
});

watch([selectedAgent, mode], () => {
  syncLocalState();
});

const handleEnter = (e) => {
  if (!e.shiftKey) sendMessage();
};

const sendMessage = () => {
  if (props.isThinking) return;
  const text = messageText.value.trim();
  const images = attachedImages.value.map(img => ({ data: img.base64, type: img.type }));
  
  if (!text && images.length === 0) return;
  
  emit('send', { text, agent: selectedAgent.value, mode: mode.value, images });
  messageText.value = '';
  attachedImages.value = [];
  nextTick(() => { if (textarea.value) textarea.value.style.height = 'auto'; });
};
</script>

<style scoped>
.chat-input-container {
  width: 100%;
  padding: 0;
  margin-top: auto;
}

.chat-input-wrapper {
  background: rgba(15, 23, 42, 0.6);
  backdrop-filter: blur(40px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 20px;
  padding: 12px;
  box-shadow: 
    0 30px 60px -12px rgba(0, 0, 0, 0.5),
    inset 0 1px 1px rgba(255, 255, 255, 0.05);
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
}

.chat-input-wrapper:focus-within {
  border-color: rgba(59, 130, 246, 0.4);
  background: rgba(15, 23, 42, 0.7);
  box-shadow: 
    0 40px 80px -20px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(59, 130, 246, 0.2);
}

.input-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px 14px;
  flex-wrap: wrap;
  padding-bottom: 10px;
  margin-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.toolbar-left, .toolbar-right {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
  flex-wrap: wrap;
}

.toolbar-left {
  flex: 1 1 320px;
}

.toolbar-right {
  flex: 1 1 240px;
  justify-content: flex-end;
}

.label {
  font-size: 10px;
  font-weight: 800;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 1.5px;
}

.agent-switcher {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  background: rgba(0, 0, 0, 0.3);
  padding: 3px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  max-width: 100%;
}

.agent-switcher button {
  background: transparent;
  border: none;
  font-size: 11px;
  font-weight: 700;
  color: #94a3b8;
  padding: 5px 12px;
  border-radius: 7px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  transition: all 0.2s;
  white-space: nowrap;
  flex: 1 1 auto;
}

.agent-switcher button.active {
  background: rgba(255, 255, 255, 0.05);
  color: #fff;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}

.dot { width: 5px; height: 5px; border-radius: 50%; }
.dot.gemini { background: #60a5fa; box-shadow: 0 0 6px #3b82f6; }
.dot.claude { background: #34d399; box-shadow: 0 0 6px #10b981; }
.dot.lmstudio { background: #2dd4bf; box-shadow: 0 0 6px #14b8a6; }

/* Safety Toggle (Switch) */
.safety-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 100px;
  transition: all 0.2s;
  flex-shrink: 0;
}
.safety-toggle:hover { background: rgba(255, 255, 255, 0.03); }

.toggle-label { font-size: 11px; font-weight: 700; color: #94a3b8; }

.switch {
  width: 32px;
  height: 18px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 100px;
  position: relative;
  transition: all 0.3s;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.switch.on { background: #3b82f6; border-color: #60a5fa; }

.handle {
  width: 12px;
  height: 12px;
  background: #fff;
  border-radius: 50%;
  position: absolute;
  top: 2px;
  left: 3px;
  transition: all 0.3s cubic-bezier(0.17, 0.67, 0.83, 0.67);
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
}

.switch.on .handle { left: 16px; }

.divider { width: 1px; height: 16px; background: rgba(255, 255, 255, 0.1); }

/* Mode Pills */
.mode-pills { display: flex; gap: 4px; flex-wrap: wrap; }
.mode-pills button {
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.05);
  color: #64748b;
  padding: 3px 10px;
  border-radius: 100px;
  font-size: 10px;
  font-weight: 800;
  text-transform: uppercase;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}
.mode-pills button.active {
  background: rgba(59, 130, 246, 0.1);
  color: #60a5fa;
  border-color: rgba(59, 130, 246, 0.3);
}

/* Previews de Imagem */
.image-previews-container {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  padding: 8px 4px;
  margin-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.image-preview-card {
  position: relative;
  width: 80px;
  height: 80px;
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0, 0, 0, 0.2);
  animation: popIn 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
}

@keyframes popIn {
  from { transform: scale(0.8); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

.image-preview-card img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.remove-img {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 20px;
  height: 20px;
  background: rgba(0, 0, 0, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.2);
  color: #fff;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  cursor: pointer;
  backdrop-filter: blur(4px);
  transition: all 0.2s;
}

.remove-img:hover {
  background: #ef4444;
  border-color: #ef4444;
  transform: scale(1.1);
}

/* Textarea Section */
.textarea-section {
  display: flex;
  align-items: flex-end;
  gap: 12px;
  padding: 4px;
}

textarea {
  flex: 1;
  min-width: 0;
  background: transparent;
  border: none;
  font-family: 'Inter', system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.6;
  color: #f1f5f9;
  resize: none;
  outline: none;
  max-height: 250px;
  padding: 8px 0;
}

textarea::placeholder { color: #475569; font-weight: 400; }

.send-btn {
  width: 38px;
  height: 38px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  color: #475569;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  flex-shrink: 0;
  margin-bottom: 4px;
}

.send-btn.ready {
  background: #fff;
  color: #000;
  border-color: #fff;
  box-shadow: 0 4px 15px rgba(255, 255, 255, 0.25);
}

.send-btn.ready:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px rgba(255, 255, 255, 0.4);
}

.send-btn:disabled { cursor: not-allowed; opacity: 0.5; }

@keyframes popIn {
  from { transform: scale(0.8); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

@media (max-width: 900px) {
  .toolbar-right {
    justify-content: flex-start;
  }

  .agent-switcher {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .chat-input-wrapper {
    padding: 10px;
    border-radius: 18px;
  }

  .input-toolbar {
    align-items: stretch;
  }

  .toolbar-left,
  .toolbar-right {
    width: 100%;
    flex: 1 1 100%;
    justify-content: flex-start;
  }

  .agent-switcher button {
    flex: 1 1 92px;
    padding: 7px 10px;
    font-size: 10px;
  }

  .safety-toggle {
    padding: 6px 10px;
  }

  .divider {
    display: none;
  }

  .mode-pills {
    flex: 1 1 auto;
  }

  .mode-pills button {
    flex: 1 1 72px;
    padding: 6px 10px;
  }

  .textarea-section {
    gap: 8px;
  }

  textarea {
    font-size: 14px;
  }
}
</style>
