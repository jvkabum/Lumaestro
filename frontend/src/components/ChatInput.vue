<template>
  <div class="chat-input-container">
    <div class="chat-input-wrapper">
      <!-- Toolbar interna do input -->
      <div class="input-toolbar">
        <div class="selector-group">
          <label>Assistant</label>
          <select v-model="selectedAgent" class="premium-select">
            <option value="gemini">Gemini CLI</option>
            <option value="claude">Claude Code</option>
          </select>
        </div>
        <div class="selector-group" style="margin-left: auto;">
          <div class="mode-toggle">
            <button 
              type="button"
              :class="{ active: mode === 'act' }" 
              @click="mode = 'act'"
              title="A IA pode alterar arquivos"
            >Act</button>
            <button 
              type="button"
              :class="{ active: mode === 'chat' }" 
              @click="mode = 'chat'"
              title="Apenas conversa"
            >Chat</button>
          </div>
        </div>
      </div>

      <!-- Área de Texto -->
      <div class="textarea-wrapper">
        <textarea
          ref="textarea"
          v-model="messageText"
          placeholder="Peça ao Maestro para construir algo..."
          @keydown.enter.prevent="handleEnter"
          :disabled="isThinking"
          :rows="1"
        ></textarea>
        
        <button 
          class="send-btn" 
          :disabled="!messageText.trim() || isThinking"
          @click="sendMessage"
        >
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <line x1="22" y1="2" x2="11" y2="13"></line>
            <polygon points="22 2 15 22 11 13 2 9 22 2"></polygon>
          </svg>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue';

const messageText = ref('');
const selectedAgent = ref('gemini');
const mode = ref('act');
const textarea = ref(null);

const props = defineProps({
  isThinking: { type: Boolean, default: false }
});

const emit = defineEmits(['send']);

const adjustHeight = () => {
  if (!textarea.value) return;
  textarea.value.style.height = 'auto';
  textarea.value.style.height = (textarea.value.scrollHeight) + 'px';
};

watch(messageText, () => {
  nextTick(adjustHeight);
});

const handleEnter = (e) => {
  if (!e.shiftKey) {
    sendMessage();
  }
};

const sendMessage = () => {
  if (props.isThinking) return;

  const text = messageText.value.trim();
  if (!text) return;
  
  emit('send', {
    text: text,
    agent: selectedAgent.value,
    mode: mode.value
  });
  
  messageText.value = '';
  nextTick(() => {
    if (textarea.value) textarea.value.style.height = 'auto';
  });
};
</script>

<style scoped>
.chat-input-container {
  padding: 0 24px 32px 24px;
  background: transparent;
  width: 100%;
}

.chat-input-wrapper {
  max-width: 860px;
  margin: 0 auto;
  background: rgba(15, 23, 42, 0.75);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 28px;
  padding: 16px 20px;
  box-shadow: 0 24px 50px -12px rgba(0, 0, 0, 0.6), inset 0 1px 0 rgba(255, 255, 255, 0.05);
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  position: relative;
}

.chat-input-wrapper:focus-within {
  border-color: rgba(96, 165, 250, 0.4);
  box-shadow: 0 24px 50px -12px rgba(0, 0, 0, 0.6), 0 0 0 4px rgba(59, 130, 246, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.input-toolbar {
  display: flex;
  align-items: center;
  gap: 24px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.selector-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.selector-group label {
  font-size: 11px;
  color: #94a3b8;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.premium-select {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  color: #f1f5f9;
  font-size: 13px;
  padding: 6px 14px;
  border-radius: 10px;
  outline: none;
  cursor: pointer;
  transition: all 0.2s;
  font-family: inherit;
  font-weight: 500;
  -webkit-appearance: none;
  -moz-appearance: none;
  appearance: none;
}

.premium-select:hover, .premium-select:focus {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.15);
}

.mode-toggle {
  display: flex;
  background: rgba(0, 0, 0, 0.5);
  padding: 4px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.mode-toggle button {
  padding: 6px 16px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 8px;
  color: #94a3b8;
  border: none;
  background: transparent;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

.mode-toggle button:hover {
  color: #e2e8f0;
}

.mode-toggle button.active {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.2) 0%, rgba(37, 99, 235, 0.1) 100%);
  color: #60a5fa;
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.2);
  border: 1px solid rgba(59, 130, 246, 0.3);
}

.textarea-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  position: relative;
  padding: 4px;
}

textarea {
  flex: 1;
  background: transparent;
  border: none;
  color: #f8fafc;
  font-size: 15.5px;
  line-height: 1.6;
  resize: none;
  outline: none;
  padding: 8px 0;
  max-height: 300px;
  font-family: 'Inter', system-ui, sans-serif;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.2) transparent;
}

textarea::placeholder {
  color: #64748b;
  font-weight: 400;
}

.send-btn {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  color: white;
  border: none;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  margin-bottom: 2px;
  flex-shrink: 0;
  box-shadow: 0 8px 16px rgba(37, 99, 235, 0.3);
}

.send-btn:hover:not(:disabled) {
  transform: translateY(-2px) scale(1.05);
  box-shadow: 0 12px 24px rgba(37, 99, 235, 0.45);
}

.send-btn:active:not(:disabled) {
  transform: translateY(1px) scale(0.95);
}

.send-btn:disabled {
  background: rgba(255, 255, 255, 0.05);
  color: #475569;
  cursor: not-allowed;
  box-shadow: none;
  transform: none;
}
</style>
