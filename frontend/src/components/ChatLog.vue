<template>
  <div class="chat-log-container" ref="logContainer" @click="handleLogClick">
    <div v-for="(msg, index) in messages" :key="index" :class="['message-row', msg.role, msg.mode]">
      <div class="message-bubble" :class="getMessageClasses(msg)">
        
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

          <!-- Renderização de Imagens (Usuário) -->
          <div v-if="msg.images && msg.images.length > 0" class="message-images">
            <img 
              v-for="(img, idx) in msg.images" 
              :key="idx" 
              :src="'data:' + (img.type || 'image/png') + ';base64,' + img.data" 
              class="chat-image"
            />
          </div>

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
        <div 
          class="sender-icon assistant-icon pulsing"
          :class="orchestrator.currentStatus?.agent ? getIconClass({ agent: orchestrator.currentStatus.agent }) : 'gemini-icon'"
        >
          <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path>
          </svg>
        </div>
        <div class="thinking-wrapper">
           <div class="thinking-waves">
             <span></span><span></span><span></span><span></span>
           </div>
           <div class="thinking-content">
             <div v-if="orchestrator.currentStatus?.tool" class="thinking-tool">
               {{ orchestrator.currentStatus.tool.replace('_', ' ').toUpperCase() }}
             </div>
             <div class="thinking-text">
               {{ orchestrator.currentStatus?.action || 'Harmonizando...' }}
             </div>
           </div>
        </div>
      </div>
    </div>

    <!-- 🧶 Indicador de Tecelagem (WEAVER) -->
    <div v-if="orchestrator.isWeaving" class="message-row assistant weaving">
      <div class="message-bubble">
        <div class="weaver-icon pulsing-cyan">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5"></path>
          </svg>
        </div>
        <div class="weaving-content">
          <div class="weaving-title">KNOWLEDGE WEAVER</div>
          <div class="weaving-status">Tecendo ligações nervosas no Grafo...</div>
          <div class="neural-nodes">
            <span class="node"></span><span class="node"></span><span class="node"></span>
          </div>
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
import { useOrchestratorStore } from '../stores/orchestrator';

const orchestrator = useOrchestratorStore();

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

const getMessageClasses = (msg) => {
  const classes = [];
  if (msg.mode === 'system') classes.push('system-message');
  
  // Detecção de Status via Emojis no conteúdo
  if (msg.mode === 'system' && msg.text) {
      if (msg.text.includes('🟢')) classes.push('success');
      if (msg.text.includes('🟡')) classes.push('warning');
      if (msg.text.includes('🔴')) classes.push('error');
  }
  
  return classes;
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
  padding: 20px 12px;
  display: flex;
  flex-direction: column;
  gap: 20px;
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
  max-width: 96%;
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
:deep(.markdown-body p) { 
  margin-bottom: 0.85rem; 
  line-height: 1.6;
}
:deep(.markdown-body h1) { 
  margin-top: 1.25rem; 
  margin-bottom: 0.75rem; 
  font-weight: 800; 
  color: #fff; 
  font-size: 1.4rem;
  letter-spacing: -0.4px;
}
:deep(.markdown-body h2) { 
  margin-top: 1rem; 
  margin-bottom: 0.6rem; 
  font-weight: 700; 
  color: #f1f5f9; 
  font-size: 1.2rem; 
}
:deep(.markdown-body ul, .markdown-body ol) { 
  padding-left: 20px; 
  margin-bottom: 1rem; 
}
:deep(.markdown-body li) { 
  margin-bottom: 6px; 
}
:deep(.markdown-body hr) {
  border: none;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.1), transparent);
  margin: 1.5rem 0;
}
:deep(.markdown-body table) { 
  width: 100%; 
  border-collapse: collapse; 
  margin-bottom: 1.5rem; 
  background: rgba(255, 255, 255, 0.03); 
  border-radius: 8px; 
  overflow: hidden; 
}
:deep(.markdown-body th, .markdown-body td) { 
  border: 1px solid rgba(255, 255, 255, 0.08); 
  padding: 10px 14px; 
  text-align: left; 
}
:deep(.markdown-body th) { background: rgba(255, 255, 255, 0.05); color: #fff; font-weight: 800; }

:deep(.code-block-wrapper) {
  background: #0d0d0f;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 12px;
  margin: 10px 0;
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

.thinking-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.thinking-tool {
  font-size: 9px;
  font-weight: 800;
  color: #3b82f6;
  letter-spacing: 1px;
}

.thinking-text { font-size: 13px; color: #94a3b8; font-weight: 500; }

/* 🧶 WEAVER Animação Premium */
.weaving .message-bubble {
  background: linear-gradient(135deg, rgba(6, 182, 212, 0.1) 0%, rgba(3, 7, 18, 0.4) 100%);
  border: 1px solid rgba(6, 182, 212, 0.2);
  padding: 12px 18px;
  display: flex;
  align-items: center;
  gap: 15px;
  border-radius: 16px 16px 16px 4px;
}

.weaver-icon {
  width: 36px;
  height: 36px;
  background: rgba(6, 182, 212, 0.2);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #06b6d4;
  box-shadow: 0 0 15px rgba(6, 182, 212, 0.3);
}

.pulsing-cyan {
  animation: cyan-glow 1.5s infinite ease-in-out;
}

@keyframes cyan-glow {
  0%, 100% { box-shadow: 0 0 5px rgba(6, 182, 212, 0.3); transform: scale(1); }
  50% { box-shadow: 0 0 20px rgba(6, 182, 212, 0.6); transform: scale(1.05); }
}

.weaving-content { display: flex; flex-direction: column; gap: 4px; }

.weaving-title {
  font-size: 10px;
  font-weight: 800;
  color: #06b6d4;
  letter-spacing: 2px;
}

.weaving-status {
  font-size: 13px;
  color: #e2e8f0;
  font-weight: 500;
}

.neural-nodes {
  display: flex;
  gap: 6px;
  margin-top: 4px;
}

.node {
  width: 4px;
  height: 4px;
  background: #06b6d4;
  border-radius: 50%;
  animation: node-pulse 1s infinite alternate;
}

.node:nth-child(2) { animation-delay: 0.3s; }
.node:nth-child(3) { animation-delay: 0.6s; }

@keyframes node-pulse {
  from { transform: scale(1); opacity: 0.3; }
  to { transform: scale(1.5); opacity: 1; box-shadow: 0 0 8px #06b6d4; }
}

/* 🖼️ Estilo Premium para Imagens no Chat */
.message-images {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 10px;
  margin-bottom: 4px;
}

.chat-image {
  max-width: 320px;
  max-height: 240px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(0, 0, 0, 0.2);
  object-fit: cover;
  cursor: zoom-in;
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.3s;
}

.chat-image:hover {
  transform: scale(1.02);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.5);
  border-color: rgba(255, 255, 255, 0.2);
}

</style>
