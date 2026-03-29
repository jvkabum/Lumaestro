<template>
  <div class="chat-log-container" ref="logContainer">
    <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role]">
      <div class="message-bubble">
        <!-- Ícone do Remetente -->
        <div class="sender-icon">
          <svg v-if="msg.role === 'assistant'" viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 2L4.5 20.29l.71.71L12 18l6.79 3 .71-.71L12 2z"/>
          </svg>
          <span v-else>U</span>
        </div>
        
        <div class="message-content">
          <div class="message-text" v-html="formatMessage(msg.text)"></div>
          
          <!-- Metadados (Agente/Modo se for assistente) -->
          <div v-if="msg.role === 'assistant' && msg.agent" class="message-meta">
            {{ msg.agent }} • {{ msg.mode }}
          </div>
        </div>
      </div>
    </div>
    
    <!-- Indicador de Digitação / Thinking -->
    <div v-if="isThinking" class="message-row assistant thinking">
      <div class="message-bubble">
        <div class="sender-icon">
          <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor">
            <path d="M12 2L4.5 20.29l.71.71L12 18l6.79 3 .71-.71L12 2z"/>
          </svg>
        </div>
        <div class="thinking-dots">
          <span></span><span></span><span></span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, nextTick } from 'vue';

const props = defineProps({
  messages: {
    type: Array,
    required: true
  },
  isThinking: {
    type: Boolean,
    default: false
  }
});

const logContainer = ref(null);

const scrollToBottom = async () => {
  await nextTick();
  if (logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight;
  }
};

watch(() => props.messages, scrollToBottom, { deep: true });
watch(() => props.isThinking, scrollToBottom);

onMounted(scrollToBottom);

// Formatação básica de Markdown (Code blocks e Breaks)
const formatMessage = (text) => {
  if (!text) return '';
  
  // Escapar HTML básico
  let formatted = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Blocos de código simples (```code```)
  formatted = formatted.replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>');
  
  // Código inline (`code`)
  formatted = formatted.replace(/`([^`]+)`/g, '<code>$1</code>');
  
  // Links (http://...)
  formatted = formatted.replace(/(https?:\/\/[^\s]+)/g, '<a href="$1" target="_blank">$1</a>');

  // Quebras de linha
  return formatted.replace(/\n/g, '<br>');
};
</script>

<style scoped>
.chat-log-container {
  flex: 1;
  overflow-y: auto;
  padding: 30px 20px;
  display: flex;
  flex-direction: column;
  gap: 32px;
  scrollbar-width: thin;
  scrollbar-color: rgba(255,255,255,0.1) transparent;
}

.message-row {
  display: flex;
  width: 100%;
}

.message-row.user {
  justify-content: flex-end;
}

.message-row.assistant {
  justify-content: flex-start;
}

.message-bubble {
  max-width: 80%;
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.user .message-bubble {
  flex-direction: row-reverse;
}

.sender-icon {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  background: rgba(255,255,255,0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #888;
  font-size: 14px;
  flex-shrink: 0;
  border: 1px solid rgba(255,255,255,0.08);
}

.user .sender-icon {
  background: #fff;
  color: #000;
  border: none;
}

.message-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.message-text {
  color: #efefef;
  font-size: 15px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.user .message-text {
  background: rgba(255, 255, 255, 0.04);
  padding: 12px 18px;
  border-radius: 18px 4px 18px 18px;
  color: #fff;
}

.assistant .message-text {
  padding: 4px 0;
}

.message-meta {
  font-size: 10px;
  color: #666;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

/* Code Styles */
:deep(pre) {
  background: #000;
  padding: 16px;
  border-radius: 12px;
  overflow-x: auto;
  margin: 12px 0;
  border: 1px solid rgba(255,255,255,0.1);
}

:deep(code) {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  color: #d1d1d1;
}

:deep(a) {
  color: #3b82f6;
  text-decoration: none;
}

:deep(a:hover) {
  text-decoration: underline;
}

/* Thinking Indicator */
.thinking-dots {
  display: flex;
  gap: 4px;
  padding: 12px 0;
}

.thinking-dots span {
  width: 4px;
  height: 4px;
  background: #666;
  border-radius: 50%;
  animation: bounce 1.4s infinite ease-in-out both;
}

.thinking-dots span:nth-child(1) { animation-delay: -0.32s; }
.thinking-dots span:nth-child(2) { animation-delay: -0.16s; }

@keyframes bounce {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}
</style>
