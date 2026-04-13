<template>
  <div class="chat-input-container">
    <div class="chat-input-wrapper glass">
      <!-- Toolbar Premium -->
      <div class="input-toolbar">
        <div class="toolbar-left">
          <div class="agent-switcher">
            <!-- Gemini Wrapper -->
            <div 
              class="agent-pill gemini-pill" 
              :class="{ active: selectedAgent === 'gemini', 'menu-open': showModelMenu }"
              @click.stop="toggleModelMenu"
            >
              <span class="dot gemini"></span>
              <span class="agent-label">Gemini</span>
              <span class="chevron-icon" :class="{ rotate: showModelMenu }">▾</span>

              <!-- Dropdown List -->
              <Transition name="menu-pop">
                <div v-if="showModelMenu" class="model-dropdown-menu glass" @click.stop>
                  <!-- ... (seções do menu permanecem iguais) ... -->
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
              class="agent-pill claude-pill"
              :class="{ active: selectedAgent === 'claude' }" 
              @click="selectedAgent = 'claude'"
            >
              <span class="dot claude"></span> Claude
            </button>
            
            <button 
              type="button" 
              class="agent-pill lmstudio-pill"
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

          <!-- Toggle Plan Mode 🔒 -->
          <div class="safety-toggle plan-toggle" @click="orchestrator.togglePlanMode(selectedAgent)">
            <span class="toggle-label">{{ orchestrator.isPlanMode ? '🔒 Plano' : 'Plano' }}</span>
            <div class="switch plan" :class="{ on: orchestrator.isPlanMode }">
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

      <!-- 📊 Token & Cache Stats Bar -->
      <div v-if="orchestrator.modelStats && orchestrator.modelStats.info" class="model-stats-bar">
        <span class="stats-text">{{ orchestrator.modelStats.info }}</span>
      </div>

      <!-- Área de Texto e Enviar -->
      <div class="textarea-section" :class="{ 'steering-mode': isThinking && messageText.trim() }">
        <textarea
          ref="textarea"
          v-model="messageText"
          :placeholder="isThinking ? 'Direcione o Maestro (Steering hint)...' : 'Comande o Maestro para construir algo extraordinário...'"
          @keydown.enter.prevent="handleEnter"
          @paste="handlePaste"
          :rows="1"
        ></textarea>
        
        <div class="actions">
          <!-- Botão Dinâmico: STOP ou STEERING (quando isThinking é true) -->
          <template v-if="isThinking">
            <button 
              v-if="!messageText.trim()"
              class="stop-btn"
              @click="orchestrator.forceUnlock()"
              title="Parar processamento e desbloquear o chat"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
                <rect x="6" y="6" width="12" height="12" rx="2" />
              </svg>
            </button>
            <button 
              v-else
              class="steer-btn"
              @click="sendMessage"
              title="Enviar direcionamento (Steering Hint) em tempo real"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
                <path d="M13,2L3,14H10V22L20,10H13V2Z" />
              </svg>
            </button>
          </template>

          <button 
            v-else
            class="send-btn" 
            :disabled="(!messageText.trim() && attachedImages.length === 0)"
            @click="sendMessage"
            :class="{ ready: (messageText.trim() || attachedImages.length > 0), 'plan-ready': orchestrator.isPlanMode }"
          >
            <template v-if="orchestrator.isPlanMode">
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </template>
            <template v-else>
              <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5">
                <path d="M7 11L12 6L17 11M12 18V7" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </template>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { nextTick, onMounted, ref, watch } from 'vue';
import { useOrchestratorStore } from '../stores/orchestrator';
import { useSettingsStore } from '../stores/settings';

const settings = useSettingsStore();
const orchestrator = useOrchestratorStore();
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
    showModelMenu.value = true;
  } else {
    showModelMenu.value = !showModelMenu.value;
  }
};

const updateGeminiModel = async () => {
  settings.config.gemini_model = activeGeminiModel.value;
  
  try {
    // 🚀 Chama o backend para mudar o modelo e reiniciar a sessão se necessário
    const bridge = window.go?.core?.App || window.go?.main?.App;
    if (bridge && bridge.SetAgentModel) {
      await bridge.SetAgentModel('gemini', activeGeminiModel.value);
    }
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

  const savedAgent = localStorage.getItem('lumaestro.chat.agent');
  const savedMode = localStorage.getItem('lumaestro.chat.mode');
  if (savedAgent) selectedAgent.value = savedAgent;
  if (savedMode) mode.value = savedMode;
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

watch([selectedAgent, mode], () => {
  localStorage.setItem('lumaestro.chat.agent', selectedAgent.value);
  localStorage.setItem('lumaestro.chat.mode', mode.value);
});

const handlePaste = async (e) => {
  const isLocalMode = (selectedAgent.value === 'lmstudio') || 
                      (settings.config.rag_provider === 'lmstudio') || 
                      (settings.config.embeddings_provider === 'native');

  const items = (e.clipboardData || e.originalEvent.clipboardData).items;
  for (const item of items) {
    if (item.type.indexOf('image') !== -1) {
      if (isLocalMode) {
        orchestrator.messages.push({
          role: 'assistant',
          text: `⚠️ **Multimídia Desativada**: Motores Locais (LM Studio / Native Embeddings) suportam apenas processamento semântico de código e texto. Para visão computacional, mude para Nuvem (Gemini/Claude).`,
          mode: 'system'
        });
        return;
      }
      
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
  const bridge = window.go?.core?.App || window.go?.main?.App;
  if (bridge && bridge.SetAutonomousMode) {
    await bridge.SetAutonomousMode(isAutonomous.value);
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

const handleEnter = (e) => {
  if (!e.shiftKey) sendMessage();
};

const sendMessage = () => {
  const text = messageText.value.trim();
  
  if (props.isThinking) {
    if (!text) return;
    orchestrator.sendSteeringHint(selectedAgent.value, text);
    messageText.value = '';
    nextTick(() => { if (textarea.value) textarea.value.style.height = 'auto'; });
    return;
  }

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
  padding: 8px 12px; /* 🗜️ Reduzido de 12px */
  box-shadow: 
    0 30px 60px -12px rgba(0, 0, 0, 0.5),
    inset 0 1px 1px rgba(255, 255, 255, 0.05);
  transition: all 0.4s cubic-bezier(0.16, 1, 0.3, 1);
  position: relative;
  z-index: 30; /* 🚀 Acima do container principal */
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
  gap: 8px;
  /* flex-wrap: wrap; 🔄 Removido para manter tudo na linha de cima */
  padding-bottom: 6px; 
  margin-bottom: 4px; 
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.toolbar-left, .toolbar-right { 
  display: flex; 
  align-items: center; 
  gap: 8px;
  flex-shrink: 0; /* Impede encolhimento que quebre o layout */
}

/* Switcher de Agentes Unificado */
.agent-switcher {
  display: flex;
  gap: 2px;
}

.agent-pill {
  position: relative;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 8px;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.03);
  color: #94a3b8;
  font-size: 10px;
  font-weight: 700;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.agent-pill:hover { background: rgba(255, 255, 255, 0.05); color: #cbd5e1; }

/* Estados Ativos por Agente */
.agent-pill.active { color: #fff; }

.agent-pill.active.gemini-pill {
  background: rgba(59, 130, 246, 0.15);
  border-color: rgba(59, 130, 246, 0.3);
  box-shadow: 0 4px 15px rgba(59, 130, 246, 0.2);
}

.agent-pill.active.claude-pill {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.3);
  box-shadow: 0 4px 15px rgba(16, 185, 129, 0.2);
}

.agent-pill.active.lmstudio-pill {
  background: rgba(234, 179, 8, 0.15);
  border-color: rgba(234, 179, 8, 0.3);
  box-shadow: 0 4px 15px rgba(234, 179, 8, 0.2);
}

.dot { width: 5px; height: 5px; border-radius: 50%; }
.dot.gemini { background: #60a5fa; box-shadow: 0 0 6px #3b82f6; }
.dot.claude { background: #34d399; box-shadow: 0 0 6px #10b981; }
.dot.lmstudio { background: #facc15; box-shadow: 0 0 6px #eab308; }

/* Dropdown Menu Premium */
.model-dropdown-menu {
  position: absolute;
  bottom: calc(100% + 12px);
  left: 0;
  width: 240px;
  padding: 12px;
  border-radius: 16px;
  background: rgba(15, 23, 42, 0.85);
  backdrop-filter: blur(20px);
  z-index: 1000;
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.08);
}

.menu-section { margin-bottom: 12px; }

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
}

.menu-item:hover { background: rgba(59, 130, 246, 0.1); }
.menu-item.selected { background: rgba(59, 130, 246, 0.2); }

.item-icon { font-size: 1.1rem; }
.item-info { display: flex; flex-direction: column; }
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

/* Safety Toggle (Switch) */
.safety-toggle {
  display: flex;
  align-items: center;
  gap: 2px; /* 🗜️ Reduzido de 4px */
  cursor: pointer;
  padding: 2px 3px; /* 🗜️ Reduzido de 6px lateral */
  border-radius: 100px;
  transition: all 0.2s;
}
.safety-toggle:hover { background: rgba(255, 255, 255, 0.03); }
.toggle-label { font-size: 8px; font-weight: 900; color: #64748b; text-transform: uppercase; letter-spacing: 0.3px; }
.switch {
  width: 20px; height: 11px; background: rgba(255, 255, 255, 0.08);
  border-radius: 100px; position: relative; transition: all 0.3s;
}
.switch.on { background: #3b82f6; }
.switch.plan.on { background: #a78bfa; }
.handle {
  width: 7px; height: 7px; background: #fff; border-radius: 50%;
  position: absolute; top: 2px; left: 2px; transition: all 0.3s;
}
.switch.on .handle { left: 11px; }
.send-btn.plan-ready { background: #a78bfa; color: #fff; border-color: #a78bfa; box-shadow: 0 0 15px rgba(167, 139, 250, 0.3); }

.divider { width: 1px; height: 16px; background: rgba(255, 255, 255, 0.1); }

/* Mode Pills */
.mode-pills { display: flex; gap: 4px; }
.mode-pills button {
  background: transparent; border: 1px solid rgba(255, 255, 255, 0.05);
  color: #64748b; padding: 3px 10px; border-radius: 100px;
  font-size: 10px; font-weight: 800; text-transform: uppercase; cursor: pointer;
}
.mode-pills button.active { background: rgba(59, 130, 246, 0.1); color: #60a5fa; border-color: rgba(59, 130, 246, 0.3); }

/* Previews de Imagem */
.image-previews-container {
  display: flex; flex-wrap: wrap; gap: 12px; padding: 8px 4px;
  margin-bottom: 8px; border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}
.image-preview-card {
  position: relative; width: 62px; height: 62px;
  border-radius: 12px; overflow: hidden; border: 1px solid rgba(255, 255, 255, 0.1);
}
.image-preview-card img { width: 100%; height: 100%; object-fit: cover; }
.remove-img {
  position: absolute; top: 2px; right: 2px; width: 16px; height: 16px;
  background: rgba(0,0,0,0.6); color: #fff; border-radius: 50%; border: none; font-size: 12px;
}

/* Textarea Section */
.textarea-section { display: flex; align-items: flex-end; gap: 12px; padding: 4px; transition: all 0.3s ease; }
.textarea-section.steering-mode { border-bottom: 2px solid rgba(167, 139, 250, 0.4); border-radius: 0 0 12px 12px; }

textarea {
  flex: 1; background: transparent; border: none; font-family: inherit; font-size: 15px;
  line-height: 1.6; color: #f1f5f9; resize: none; outline: none; max-height: 250px; padding: 8px 0;
}
textarea::placeholder { color: #475569; }

.send-btn {
  width: 38px; height: 38px; background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05); color: #475569; border-radius: 12px;
  display: flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.3s;
}
.send-btn.ready { background: #fff; color: #000; border-color: #fff; }
.send-btn:disabled { cursor: not-allowed; opacity: 0.5; }

.stop-btn {
  width: 38px; height: 38px; background: rgba(239, 68, 68, 0.15); border: 1px solid rgba(239, 68, 68, 0.4);
  color: #fca5a5; border-radius: 12px; display: flex; align-items: center; justify-content: center; cursor: pointer;
}

.steer-btn {
  width: 38px; height: 38px; background: linear-gradient(135deg, #a78bfa 0%, #7c3aed 100%);
  color: #fff; border: none; border-radius: 12px; display: flex; align-items: center; justify-content: center;
  box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3); cursor: pointer; transition: all 0.2s;
}
.steer-btn:hover { transform: scale(1.05); }

.model-stats-bar {
  margin-bottom: 8px;
  padding: 4px 12px;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 6px;
  display: flex;
  align-items: center;
}
.stats-text {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.4);
  font-family: 'JetBrains Mono', monospace;
  letter-spacing: 0.5px;
}
</style>
