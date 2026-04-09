<template>
  <div class="chat-input-container">
    <div class="chat-input-wrapper glass">
      <!-- Toolbar Premium -->
      <div class="input-toolbar">
        <div class="toolbar-left">
          <div class="agent-switcher">
            <!-- Custom Premium Model Selector -->
            <div 
              class="agent-btn-wrapper" 
              :class="{ active: selectedAgent === 'gemini', 'menu-open': showModelMenu }"
              @click.stop="toggleModelMenu"
            >
              <span class="dot gemini"></span>
              <span class="agent-label">Gemini</span>
              <span class="chevron-icon" :class="{ rotate: showModelMenu }">▾</span>

              <!-- Dropdown List (Custom UI) -->
              <Transition name="menu-pop">
                <div v-if="showModelMenu" class="model-dropdown-menu glass" @click.stop>
                  <div class="menu-section">
                    <label>⚡ AUTOMÁTICO</label>
                    <div 
                      class="menu-item" 
                      :class="{ selected: activeGeminiModel === 'auto-gemini-2.5' }"
                      @click="selectModel('auto-gemini-2.5')"
                    >
                      <span class="item-icon">⚡</span>
                      <div class="item-info">
                        <span class="item-name">Auto (Gemini 2.5)</span>
                        <span class="item-desc">Equilíbrio sugerido</span>
                      </div>
                    </div>
                    <div 
                      class="menu-item" 
                      :class="{ selected: activeGeminiModel === 'auto-gemini-3' }"
                      @click="selectModel('auto-gemini-3')"
                    >
                      <span class="item-icon">🧪</span>
                      <div class="item-info">
                        <span class="item-name">Auto (Gemini 3)</span>
                        <span class="item-desc">Experimental / Preview</span>
                      </div>
                    </div>
                  </div>

                  <div class="menu-section">
                    <label>🧠 RACIOCÍNIO (PRO)</label>
                    <div 
                      class="menu-item" 
                      :class="{ selected: activeGeminiModel === 'gemini-2.5-pro' }"
                      @click="selectModel('gemini-2.5-pro')"
                    >
                      <span class="item-icon">🧠</span>
                      <div class="item-info">
                        <span class="item-name">2.5 Pro</span>
                        <span class="item-desc">Lógica complexa</span>
                      </div>
                    </div>
                  </div>

                  <div class="menu-section">
                    <label>🚀 VELOCIDADE (FLASH)</label>
                    <div 
                      class="menu-item" 
                      :class="{ selected: activeGeminiModel === 'gemini-2.5-flash' }"
                      @click="selectModel('gemini-2.5-flash')"
                    >
                      <span class="item-icon">🚀</span>
                      <div class="item-info">
                        <span class="item-name">2.5 Flash</span>
                        <span class="item-desc">Respostas rápidas</span>
                      </div>
                    </div>
                    <div 
                      class="menu-item" 
                      :class="{ selected: activeGeminiModel === 'gemini-2.5-flash-lite' }"
                      @click="selectModel('gemini-2.5-flash-lite')"
                    >
                      <span class="item-icon">⚡</span>
                      <div class="item-info">
                        <span class="item-name">Flash Lite</span>
                        <span class="item-desc">Ultra leve</span>
                      </div>
                    </div>
                  </div>
                </div>
              </Transition>
            </div>

            <button 
              type="button" 
              :class="{ active: selectedAgent === 'claude' }" 
              @click="selectedAgent = 'claude'"
            >
              <span class="dot claude"></span> Claude
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
          @keydown="handleKeydown"
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
import { useSettingsStore } from '../stores/settings';

const settings = useSettingsStore();
const activeGeminiModel = ref('auto-gemini-2.5');

onMounted(() => {
  if (settings.config.gemini_model) {
    activeGeminiModel.value = settings.config.gemini_model;
  }
});

const showModelMenu = ref(false);

const toggleModelMenu = () => {
  if (selectedAgent.value !== 'gemini') {
    selectedAgent.value = 'gemini';
    showModelMenu.value = true; // Abre direto ao selecionar
  } else {
    showModelMenu.value = !showModelMenu.value;
  }
};

const updateGeminiModel = async () => {
  // 1. Atualiza a store local
  settings.config.gemini_model = activeGeminiModel.value;
  
  // 2. Notifica o Backend para persistir e mudar o modelo em tempo real via ACP
  try {
    await SetAgentModel('gemini', activeGeminiModel.value);
  } catch (e) {
    console.error("[ChatInput] Erro ao trocar modelo no backend:", e);
  }
};

const selectModel = async (modelId) => {
  activeGeminiModel.value = modelId;
  showModelMenu.value = false;
  await updateGeminiModel();
};

// Fecha o menu ao clicar fora
onMounted(() => {
  window.addEventListener('click', (e) => {
    if (!e.target.closest('.agent-btn-wrapper')) {
      showModelMenu.value = false;
    }
  });
});

const messageText = ref('');
const selectedAgent = ref('gemini');
const mode = ref('act');
const textarea = ref(null);
const isAutonomous = ref(false);
const attachedImages = ref([]); // [{ preview, base64, type }]

const props = defineProps({
  isThinking: { type: Boolean, default: false }
});

const emit = defineEmits(['send']);

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
    await window.go.main.App.SetAutonomousMode(isAutonomous.value);
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

const handleKeydown = (e) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    console.log("[ChatInput] Enter detectado. Disparando envio...");
    sendMessage();
  }
};

const handleEnter = (e) => {
  // Mantido para compatibilidade se houver chamadas via @enter
  if (!e.shiftKey) {
     e.preventDefault();
     sendMessage();
  }
};

const sendMessage = () => {
  if (props.isThinking) {
    console.warn("[ChatInput] Bloqueado: IA ainda está pensando.");
    return;
  }
  const text = messageText.value.trim();
  const images = attachedImages.value.map(img => ({ data: img.base64, type: img.type }));
  
  if (!text && images.length === 0) return;
  
  console.log("[ChatInput] Enviando mensagem:", text.substring(0, 20) + "...");
  emit('send', { text, agent: selectedAgent.value, mode: mode.value, images });
  
  // Limpeza imediata para feedback visual de sucesso
  messageText.value = '';
  attachedImages.value = [];
  
  nextTick(() => { 
    if (textarea.value) {
      textarea.value.style.height = 'auto';
      textarea.value.focus(); // Retorna o foco após o envio
    }
  });
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
  padding-bottom: 10px;
  margin-bottom: 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.toolbar-left, .toolbar-right { display: flex; align-items: center; gap: 14px; }

.label {
  font-size: 10px;
  font-weight: 800;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 1.5px;
}

.agent-switcher {
  display: flex;
  background: rgba(0, 0, 0, 0.3);
  padding: 3px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  gap: 4px;
}

.agent-btn-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  border-radius: 7px;
  cursor: pointer;
  background: transparent;
  color: #94a3b8;
  font-size: 11px;
  font-weight: 700;
  transition: all 0.2s;
}

.agent-btn-wrapper.active {
  background: rgba(59, 130, 246, 0.15);
  color: #fff;
  border: 1px solid rgba(59, 130, 246, 0.3);
  box-shadow: 0 4px 15px rgba(59, 130, 246, 0.2);
}

.chevron-icon {
  font-size: 10px;
  opacity: 0.5;
  transition: transform 0.3s ease;
  margin-left: -2px;
}

.chevron-icon.rotate { transform: rotate(180deg); }

/* --- Dropdown Menu Premium --- */
.model-dropdown-menu {
  position: absolute;
  bottom: calc(100% + 12px);
  left: 0;
  width: 240px;
  padding: 12px;
  border-radius: 16px;
  z-index: 1000;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
  animation: menu-pop 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  transform-origin: bottom left;
}

.menu-section { margin-bottom: 12px; }
.menu-section:last-child { margin-bottom: 0; }

.menu-section label {
  display: block;
  font-size: 9px;
  font-weight: 900;
  color: #64748b;
  letter-spacing: 1.5px;
  margin-bottom: 8px;
  padding-left: 6px;
}

.menu-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-bottom: 2px;
}

.menu-item:hover { background: rgba(59, 130, 246, 0.1); }
.menu-item.selected { background: rgba(59, 130, 246, 0.2); border: 1px solid rgba(59, 130, 246, 0.2); }

.item-icon { font-size: 1.2rem; }
.item-info { display: flex; flex-direction: column; gap: 1px; }
.item-name { font-size: 12px; font-weight: 700; color: #f1f5f9; }
.item-desc { font-size: 10px; color: #94a3b8; }

/* Transição de Menu */
.menu-pop-enter-active, .menu-pop-leave-active {
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}
.menu-pop-enter-from, .menu-pop-leave-to {
  opacity: 0;
  transform: translateY(10px) scale(0.95);
}

.agent-model-select { display: none; }

/* Safety Toggle (Switch) */
.safety-toggle {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 100px;
  transition: all 0.2s;
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
.mode-pills { display: flex; gap: 4px; }
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
</style>
