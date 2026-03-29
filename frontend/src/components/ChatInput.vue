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
        <div class="selector-group">
          <label>Mode</label>
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
          :rows="1"
        ></textarea>
        
        <button 
          class="send-btn" 
          :disabled="!messageText.trim()"
          @click="sendMessage"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2">
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
  padding: 24px;
  background: transparent;
  width: 100%;
}

.chat-input-wrapper {
  max-width: 800px;
  margin: 0 auto;
  background: rgba(28, 28, 30, 0.85);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 20px;
  padding: 14px 18px;
  box-shadow: 0 12px 48px rgba(0,0,0,0.5);
}

.input-toolbar {
  display: flex;
  gap: 28px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.selector-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.selector-group label {
  font-size: 10px;
  color: #6a6a6a;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 1px;
}

.premium-select {
  background: rgba(255,255,255,0.05);
  border: 1px solid rgba(255,255,255,0.1);
  color: #efefef;
  font-size: 11px;
  padding: 4px 10px;
  border-radius: 8px;
  outline: none;
  cursor: pointer;
}

.mode-toggle {
  display: flex;
  background: rgba(0,0,0,0.3);
  padding: 3px;
  border-radius: 8px;
}

.mode-toggle button {
  padding: 3px 14px;
  font-size: 11px;
  font-weight: 500;
  border-radius: 6px;
  color: #555;
  border: none;
  background: transparent;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.mode-toggle button.active {
  background: rgba(255,255,255,0.1);
  color: #fff;
  box-shadow: 0 2px 8px rgba(0,0,0,0.25);
}

.textarea-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 14px;
}

textarea {
  flex: 1;
  background: transparent;
  border: none;
  color: #fff;
  font-size: 16px;
  line-height: 1.6;
  resize: none;
  outline: none;
  padding: 10px 0;
  max-height: 250px;
}

textarea::placeholder {
  color: rgba(255,255,255,0.15);
}

.send-btn {
  background: #ffffff;
  color: #000;
  border: none;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  margin-bottom: 6px;
  flex-shrink: 0;
}

.send-btn:hover {
  transform: scale(1.1);
  box-shadow: 0 0 15px rgba(255,255,255,0.2);
}

.send-btn:active {
  transform: scale(0.95);
}

.send-btn:disabled {
  opacity: 0.1;
  cursor: not-allowed;
  transform: none;
  background: #333;
}
</style>
