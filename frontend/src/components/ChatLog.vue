<template>
  <div class="chat-log-container" ref="logContainer">
    <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role]">
      <div class="message-bubble">
        <!-- Ícone do Remetente -->
        <div class="sender-icon" :class="{ 'glass-icon': msg.role === 'assistant', 'user-icon': msg.role === 'user' }">
          <svg v-if="msg.role === 'assistant'" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 12 20 22 4 22 4 12"></polyline>
            <rect x="2" y="7" width="20" height="5" rx="2" ry="2"></rect>
            <line x1="12" y1="22" x2="12" y2="7"></line>
            <path d="M12 7H7.5a2.5 2.5 0 0 1 0-5C11 2 12 7 12 7z"></path>
            <path d="M12 7h4.5a2.5 2.5 0 0 0 0-5C13 2 12 7 12 7z"></path>
          </svg>
          <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
             <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
             <circle cx="12" cy="7" r="4"></circle>
          </svg>
        </div>
        
        <div class="message-content">
          <div class="message-text" v-html="formatMessage(msg.text)"></div>
          
          <!-- Metadados (Agente/Modo se for assistente) -->
          <div v-if="msg.role === 'assistant' && msg.agent" class="message-meta">
            <span class="agent-badge">{{ msg.agent }}</span>
            <span class="mode-badge">{{ msg.mode }}</span>
          </div>
        </div>
      </div>
    </div>
    
    <!-- Indicador de Digitação / Thinking -->
    <div v-if="isThinking" class="message-row assistant thinking">
      <div class="message-bubble">
        <div class="sender-icon glass-icon">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <path d="M12 16v-4"></path>
            <path d="M12 8h.01"></path>
          </svg>
        </div>
        <div class="thinking-wrapper">
           <div class="thinking-text">Processando</div>
           <div class="thinking-dots">
             <span></span><span></span><span></span>
           </div>
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
    logContainer.value.scrollTo({
      top: logContainer.value.scrollHeight,
      behavior: 'smooth'
    });
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
  padding: 40px 20px;
  display: flex;
  flex-direction: column;
  gap: 36px;
  scrollbar-width: thin;
  scrollbar-color: rgba(255, 255, 255, 0.1) transparent;
  scroll-behavior: smooth;
}

.message-row {
  display: flex;
  width: 100%;
  animation: slideUp 0.4s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  opacity: 0;
  transform: translateY(15px);
}

@keyframes slideUp {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.message-row.user {
  justify-content: flex-end;
}

.message-row.assistant {
  justify-content: flex-start;
}

.message-bubble {
  max-width: 85%;
  display: flex;
  gap: 20px;
  align-items: flex-start;
}

.user .message-bubble {
  flex-direction: row-reverse;
}

.sender-icon {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
  transition: transform 0.3s ease;
}

.sender-icon:hover {
  transform: scale(1.05);
}

.glass-icon {
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.1) 0%, rgba(255, 255, 255, 0.03) 100%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #60a5fa; /* A nice premium blue */
  backdrop-filter: blur(10px);
}

.user-icon {
  background: linear-gradient(135deg, #f8fafc 0%, #cbd5e1 100%);
  color: #0f172a;
}

.message-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.message-text {
  font-size: 15px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
  color: #f1f5f9;
}

.user .message-text {
  background: linear-gradient(135deg, rgba(59, 130, 246, 0.15) 0%, rgba(37, 99, 235, 0.05) 100%);
  border: 1px solid rgba(59, 130, 246, 0.2);
  padding: 14px 20px;
  border-radius: 20px 4px 20px 20px;
  color: #f8fafc;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.assistant .message-text {
  padding: 6px 0;
  color: #cbd5e1;
}

.message-meta {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-top: 4px;
}

.agent-badge, .mode-badge {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.8px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 8px;
}

.agent-badge {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.mode-badge {
  background: rgba(255, 255, 255, 0.05);
  color: #94a3b8;
  border: 1px solid rgba(255, 255, 255, 0.08);
}

/* Code Styles - Sleek */
:deep(pre) {
  background: #09090b;
  padding: 20px;
  border-radius: 12px;
  overflow-x: auto;
  margin: 16px 0;
  border: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: inset 0 2px 8px rgba(0, 0, 0, 0.3);
}

:deep(code) {
  font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
  font-size: 13.5px;
  color: #e2e8f0;
}

:deep(p > code) {
  background: rgba(255, 255, 255, 0.1);
  padding: 3px 6px;
  border-radius: 6px;
  color: #93c5fd;
}

:deep(a) {
  color: #60a5fa;
  text-decoration: none;
  font-weight: 500;
  transition: color 0.2s;
  border-bottom: 1px solid transparent;
}

:deep(a:hover) {
  color: #93c5fd;
  border-bottom-color: #93c5fd;
}

/* Thinking Indicator */
.thinking-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 18px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 16px 16px 16px 4px;
}

.thinking-text {
  font-size: 13px;
  background: linear-gradient(90deg, #94a3b8, #cbd5e1);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-weight: 500;
  animation: pulseOpacity 2s infinite;
}

@keyframes pulseOpacity {
  0%, 100% { opacity: 0.6; }
  50% { opacity: 1; }
}

.thinking-dots {
  display: flex;
  gap: 5px;
}

.thinking-dots span {
  width: 5px;
  height: 5px;
  background: #60a5fa;
  border-radius: 50%;
  animation: gentleBounce 1.4s infinite ease-in-out both;
}

.thinking-dots span:nth-child(1) { animation-delay: -0.32s; }
.thinking-dots span:nth-child(2) { animation-delay: -0.16s; }

@keyframes gentleBounce {
  0%, 80%, 100% { transform: translateY(0); opacity: 0.4; }
  40% { transform: translateY(-4px); opacity: 1; box-shadow: 0 4px 8px rgba(96, 165, 250, 0.5); }
}
</style>
