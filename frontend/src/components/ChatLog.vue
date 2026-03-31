<template>
  <div class="chat-log-container" ref="logContainer" @click="handleLogClick">
    <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role, msg.mode]">
      <div class="message-bubble" :class="{ 'system-message': msg.mode === 'system' }">
        
        <!-- Ícone Dinâmico por Agente -->
        <div 
          v-if="msg.mode !== 'system'"
          class="sender-icon" 
          :class="getIconClass(msg)"
        >
          <template v-if="msg.role === 'user'">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
               <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path>
               <circle cx="12" cy="7" r="4"></circle>
            </svg>
          </template>
          
          <template v-else-if="msg.agent === 'Terminal'">
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
          </template>
          
          <template v-else-if="msg.agent === 'Claude'">
             <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"></path>
            </svg>
          </template>

          <template v-else>
            <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon>
            </svg>
          </template>
        </div>
        
        <div class="message-content">
          <!-- Bloco de Raciocínio (Colapsável) -->
          <ThoughtBlock 
            v-if="msg.role === 'assistant' && msg.thought && msg.mode !== 'system'" 
            :thought="msg.thought" 
            :agent="msg.agent" 
          />

          <!-- Renderização Premium via Markdown-It -->
          <div v-if="msg.text" class="message-text markdown-body" v-html="renderMarkdown(msg.text)"></div>
          
          <!-- Metadados -->
          <div v-if="msg.role === 'assistant' && msg.agent && msg.mode !== 'system'" class="message-meta">
            <span class="agent-badge">{{ msg.agent }}</span>

          </div>
        </div>
      </div>
    </div>
    
    <!-- Indicador de Digitação (Thinking) -->
    <div v-if="isThinking" class="message-row assistant thinking">
      <div class="message-bubble">
        <div class="sender-icon assistant-icon pulsing">
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
          </svg>
        </div>
        <div class="thinking-wrapper">
           <div class="thinking-waves">
             <span></span><span></span><span></span><span></span>
           </div>
           <div class="thinking-text">Harmonizando...</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, onMounted, nextTick } from 'vue';
import { useClipboard } from '@vueuse/core';
import MarkdownIt from 'markdown-it';
import ThoughtBlock from './ThoughtBlock.vue';

const props = defineProps({
  messages: { type: Array, required: true },
  isThinking: { type: Boolean, default: false }
});

const md = new MarkdownIt({
    html: true,
    linkify: true,
    typographer: true,
});

// Custom Rule para blocos de código com botão de cópia
const defaultRender = md.renderer.rules.fence || function(tokens, idx, options, env, self) {
    return self.renderToken(tokens, idx, options);
};

md.renderer.rules.fence = function (tokens, idx, options, env, self) {
    const token = tokens[idx];
    const code = token.content.trim();
    const lang = token.info.trim();
    
    return `<div class="code-block-wrapper">
              <div class="code-header">
                <span>${lang || 'code'}</span>
                <button class="copy-btn">COPY</button>
              </div>
              <pre><code>${md.utils.escapeHtml(code)}</code></pre>
            </div>`;
};

const { copy } = useClipboard();
const logContainer = ref(null);

const renderMarkdown = (text) => {
    if (!text) return '';
    return md.render(text);
};

const handleLogClick = (e) => {
  const btn = e.target.closest('.copy-btn');
  if (!btn) return;

  const wrapper = btn.closest('.code-block-wrapper');
  const code = wrapper.querySelector('code').innerText;
  
  copy(code);

  const originalText = btn.innerHTML;
  btn.classList.add('copied');
  btn.innerText = 'COPIED!';
  
  setTimeout(() => {
    btn.classList.remove('copied');
    btn.innerHTML = originalText;
  }, 2000);
};

const getIconClass = (msg) => {
  if (msg.role === 'user') return 'user-icon';
  if (msg.agent === 'Terminal') return 'terminal-icon';
  if (msg.agent === 'Claude') return 'claude-icon';
  return 'gemini-icon';
};

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
</script>

<style scoped>
.chat-log-container {
  flex: 1;
  overflow-y: auto;
  padding: 40px 24px;
  display: flex;
  flex-direction: column;
  gap: 32px;
  scrollbar-width: none;
}
.chat-log-container::-webkit-scrollbar { display: none; }

.message-row {
  display: flex;
  width: 100%;
  animation: slideUp 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards;
  opacity: 0;
  transform: translateY(20px);
}

@keyframes slideUp { to { opacity: 1; transform: translateY(0); } }

.message-row.user { justify-content: flex-end; }
.message-row.assistant { justify-content: flex-start; }

.message-bubble {
  max-width: 85%;
  display: flex;
  gap: 16px;
  align-items: flex-start;
}
.user .message-bubble { flex-direction: row-reverse; }

.sender-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.2);
}

.gemini-icon { background: linear-gradient(135deg, #1e3a8a 0%, #3b82f6 100%); color: #fff; }
.claude-icon { background: linear-gradient(135deg, #064e3b 0%, #10b981 100%); color: #fff; }
.terminal-icon { background: linear-gradient(135deg, #78350f 0%, #f59e0b 100%); color: #fff; }
.user-icon { background: #f8fafc; color: #0f172a; }

.message-content { min-width: 0; flex: 1; }

.message-text {
  font-size: 15px;
  line-height: 1.65;
  color: #e2e8f0;
}

.user .message-text {
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.2);
  padding: 12px 18px;
  border-radius: 18px 2px 18px 18px;
  box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
}

.system-message .message-text {
  font-style: italic;
  font-size: 13px;
  color: #94a3b8;
  border-left: 2px solid #3b82f6;
  padding-left: 15px;
}

/* Markdown Premium Styles */
:deep(.markdown-body) {
  color: #e2e8f0;
}
:deep(.markdown-body p) { margin-bottom: 16px; }
:deep(.markdown-body h1, .markdown-body h2) { margin-top: 24px; margin-bottom: 16px; font-weight: 800; color: #fff; }
:deep(.markdown-body h1) { font-size: 1.5rem; }
:deep(.markdown-body h2) { font-size: 1.25rem; }
:deep(.markdown-body ul, .markdown-body ol) { padding-left: 24px; margin-bottom: 16px; }
:deep(.markdown-body li) { margin-bottom: 8px; }
:deep(.markdown-body table) { width: 100%; border-collapse: collapse; margin-bottom: 16px; background: rgba(255, 255, 255, 0.03); border-radius: 8px; overflow: hidden; }
:deep(.markdown-body th, .markdown-body td) { border: 1px solid rgba(255, 255, 255, 0.08); padding: 10px 14px; text-align: left; }
:deep(.markdown-body th) { background: rgba(255, 255, 255, 0.05); color: #fff; font-weight: 800; }

:deep(.code-block-wrapper) {
  background: #0d0d0f;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  margin: 15px 0;
  overflow: hidden;
}
:deep(.code-header) {
  background: rgba(255, 255, 255, 0.03);
  padding: 8px 14px;
  font-size: 11px;
  text-transform: uppercase;
  color: #64748b;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

:deep(.copy-btn) {
  background: rgba(59, 130, 246, 0.1);
  border: 1px solid rgba(59, 130, 246, 0.2);
  color: #60a5fa;
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 800;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 10px;
}
:deep(.copy-btn:hover) { background: #3b82f6; color: #fff; }
:deep(.copy-btn.copied) { background: #10b981; border-color: #059669; color: #fff; }
:deep(pre) { padding: 16px; margin: 0; overflow-x: auto; }
:deep(code) { font-family: 'JetBrains Mono', monospace; font-size: 13px; }

/* Thinking Waviness */
.thinking-wrapper {
  background: rgba(255, 255, 255, 0.03);
  padding: 10px 16px;
  border-radius: 4px 16px 16px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.thinking-waves { display: flex; align-items: flex-end; gap: 3px; height: 12px; }
.thinking-waves span {
  width: 3px;
  background: #3b82f6;
  border-radius: 1px;
  animation: waviness 1.2s infinite ease-in-out;
}
.thinking-waves span:nth-child(2) { animation-delay: 0.1s; }
.thinking-waves span:nth-child(3) { animation-delay: 0.2s; }
.thinking-waves span:nth-child(4) { animation-delay: 0.3s; }

@keyframes waviness {
  0%, 100% { height: 4px; opacity: 0.3; }
  50% { height: 12px; opacity: 1; }
}

.thinking-text { font-size: 13px; color: #94a3b8; font-weight: 500; }

</style>
