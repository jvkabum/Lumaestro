<script setup>
import { ref, computed } from 'vue'
import { useOrchestratorStore } from '../stores/orchestrator'

const orchestrator = useOrchestratorStore()

const query = ref('')
const pathFilter = ref('')
const isLiteral = ref(false)
const isSearching = ref(false)
const searchResult = ref({ totalCount: 0, files: [] })
const hasSearched = ref(false)
const copiedKey = ref(null)

const runSearch = async () => {
  if (!query.value.trim()) return
  isSearching.value = true
  hasSearched.value = true
  try {
    const data = await orchestrator.runCodeSearch(query.value.trim(), pathFilter.value.trim(), isLiteral.value)
    searchResult.value = data || { totalCount: 0, files: [] }
  } catch (err) {
    console.error('[CodeSearch] Erro:', err)
    searchResult.value = { totalCount: 0, files: [] }
  } finally {
    isSearching.value = false
  }
}

const handleKeydown = (e) => {
  if (e.key === 'Enter') {
    runSearch()
  } else if (e.key === 'Escape') {
    close()
  }
}

const close = () => {
  orchestrator.toggleCodeSearch(false)
}

const copyReference = (filePath, lineNum, key) => {
  const refText = `${filePath}:${lineNum}`
  navigator.clipboard?.writeText(refText)
  copiedKey.value = key
  setTimeout(() => {
    if (copiedKey.value === key) copiedKey.value = null
  }, 1800)
}

const totalMatches = computed(() => searchResult.value.totalCount || 0)
const totalFiles = computed(() => searchResult.value.files?.length || 0)
</script>

<template>
  <Transition name="search-fade">
    <div v-if="orchestrator.isCodeSearchOpen" class="search-modal-backdrop" @click.self="close">
      <div class="search-modal glass-heavy" @keydown="handleKeydown">
        <!-- Header -->
        <header class="modal-header">
          <div class="header-left">
            <span class="header-icon">🔎</span>
            <div>
              <h3>PESQUISA DE CÓDIGO (/codesearch)</h3>
              <p>Explore símbolos, regex e termos no workspace com contexto de código</p>
            </div>
          </div>
          <button class="close-btn" @click="close" title="Fechar (Esc)">✕</button>
        </header>

        <!-- Search Bar -->
        <div class="search-controls">
          <div class="input-group main-input">
            <span class="input-icon">⚡</span>
            <input 
              v-model="query" 
              type="text" 
              placeholder="Digite o termo ou padrão Regex..."
              autofocus
              @keydown.enter="runSearch"
            />
          </div>

          <div class="input-group filter-input">
            <span class="input-icon">📁</span>
            <input 
              v-model="pathFilter" 
              type="text" 
              placeholder="Filtro de pasta/arquivo (ex: internal/ ou .go)"
              @keydown.enter="runSearch"
            />
          </div>

          <div class="search-options">
            <label class="checkbox-label" title="Busca literal exata sem interpretação regex">
              <input type="checkbox" v-model="isLiteral" />
              <span>Literal</span>
            </label>
            <button class="btn-search" :disabled="!query.trim() || isSearching" @click="runSearch">
              <span v-if="isSearching">Buscando...</span>
              <span v-else>Buscar</span>
            </button>
          </div>
        </div>

        <!-- Summary -->
        <div v-if="hasSearched" class="search-summary">
          <span v-if="totalMatches > 0" class="summary-found">
            Encontradas <strong>{{ totalMatches }}</strong> ocorrências em <strong>{{ totalFiles }}</strong> arquivos
          </span>
          <span v-else class="summary-empty">
            Nenhuma ocorrência encontrada para "{{ query }}"
          </span>
        </div>

        <!-- Results Area -->
        <div class="results-scroll">
          <div v-if="isSearching" class="loading-state">
            <div class="spinner"></div>
            <span>Vasculhando arquivos do workspace...</span>
          </div>

          <div v-else-if="searchResult.files && searchResult.files.length > 0" class="results-list">
            <div v-for="fileItem in searchResult.files" :key="fileItem.path" class="file-group">
              <div class="file-header">
                <span class="file-icon">📄</span>
                <span class="file-path">{{ fileItem.path }}</span>
                <span class="file-badge">{{ fileItem.matches.length }}</span>
              </div>

              <div class="file-matches">
                <div 
                  v-for="(match, mIdx) in fileItem.matches" 
                  :key="fileItem.path + '-' + match.lineNumber + '-' + mIdx" 
                  class="match-card"
                  @click="copyReference(fileItem.path, match.lineNumber, fileItem.path + '-' + match.lineNumber)"
                  title="Clique para copiar referência (arquivo:linha)"
                >
                  <div class="match-line-meta">
                    <span class="line-badge">Linha {{ match.lineNumber }}</span>
                    <span class="copy-status" v-if="copiedKey === (fileItem.path + '-' + match.lineNumber)">✓ Copiado!</span>
                  </div>
                  
                  <div class="code-snippet">
                    <pre v-if="match.contextBefore" class="ctx-before"><code>{{ match.contextBefore }}</code></pre>
                    <pre class="match-code"><code>{{ match.lineContent }}</code></pre>
                    <pre v-if="match.contextAfter" class="ctx-after"><code>{{ match.contextAfter }}</code></pre>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div v-else-if="!hasSearched" class="empty-placeholder">
            <p>💡 Digite um termo acima e pressione <strong>Enter</strong> para pesquisar no projeto.</p>
            <p class="hint">Suporta expressões regulares (Regex) inteligentes ou busca literal exata.</p>
          </div>
        </div>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.search-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1200;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(2, 6, 23, 0.85);
  backdrop-filter: blur(10px);
  padding: 30px;
}

.search-modal {
  width: 100%;
  max-width: 900px;
  height: 85vh;
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.8);
  overflow: hidden;
}

.modal-header {
  padding: 18px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(255, 255, 255, 0.02);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.header-icon { font-size: 1.6rem; }

.header-left h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 800;
  letter-spacing: 1.5px;
  color: #f1f5f9;
}

.header-left p {
  margin: 2px 0 0 0;
  font-size: 11px;
  color: #94a3b8;
}

.close-btn {
  background: transparent;
  border: none;
  color: #94a3b8;
  font-size: 16px;
  cursor: pointer;
  padding: 6px 10px;
  border-radius: 8px;
  transition: all 0.2s;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: white;
}

.search-controls {
  padding: 16px 24px;
  display: flex;
  gap: 12px;
  background: rgba(0, 0, 0, 0.2);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
  flex-wrap: wrap;
}

.input-group {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(15, 23, 42, 0.8);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 8px 12px;
}

.input-group:focus-within {
  border-color: #3b82f6;
  box-shadow: 0 0 0 1px #3b82f6;
}

.main-input {
  flex: 1;
  min-width: 250px;
}

.filter-input {
  width: 240px;
}

.input-icon { font-size: 14px; opacity: 0.7; }

.input-group input {
  background: transparent;
  border: none;
  color: #f8fafc;
  font-size: 13px;
  outline: none;
  width: 100%;
}

.search-options {
  display: flex;
  align-items: center;
  gap: 12px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #94a3b8;
  cursor: pointer;
}

.btn-search {
  background: #2563eb;
  color: white;
  border: none;
  border-radius: 8px;
  padding: 8px 18px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: background 0.2s;
}

.btn-search:hover:not(:disabled) {
  background: #1d4ed8;
}

.btn-search:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.search-summary {
  padding: 8px 24px;
  font-size: 11px;
  background: rgba(255, 255, 255, 0.02);
  border-bottom: 1px solid rgba(255, 255, 255, 0.04);
}

.summary-found { color: #38bdf8; }
.summary-empty { color: #f87171; }

.results-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px;
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
  gap: 12px;
  color: #94a3b8;
  font-size: 13px;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: #3b82f6;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.empty-placeholder {
  text-align: center;
  padding: 80px 20px;
  color: #64748b;
  font-size: 13px;
}

.empty-placeholder .hint {
  font-size: 11px;
  color: #475569;
  margin-top: 6px;
}

.file-group {
  margin-bottom: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 12px;
  overflow: hidden;
}

.file-header {
  padding: 8px 14px;
  background: rgba(255, 255, 255, 0.04);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  color: #e2e8f0;
}

.file-badge {
  margin-left: auto;
  font-size: 10px;
  background: rgba(59, 130, 246, 0.2);
  color: #60a5fa;
  padding: 1px 7px;
  border-radius: 10px;
}

.file-matches {
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.match-card {
  background: rgba(0, 0, 0, 0.3);
  border-radius: 8px;
  padding: 8px 12px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.match-card:hover {
  background: rgba(255, 255, 255, 0.05);
  border-color: rgba(59, 130, 246, 0.3);
}

.match-line-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 4px;
}

.line-badge {
  font-size: 10px;
  color: #f59e0b;
  font-family: monospace;
  font-weight: 700;
}

.copy-status {
  font-size: 10px;
  color: #10b981;
  font-weight: 700;
}

.code-snippet {
  display: flex;
  flex-direction: column;
}

.match-code, .ctx-before, .ctx-after {
  margin: 0;
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.match-code {
  color: #f8fafc;
  background: rgba(59, 130, 246, 0.12);
  padding: 2px 4px;
  border-radius: 4px;
  border-left: 2px solid #3b82f6;
}

.ctx-before, .ctx-after {
  color: #64748b;
  padding: 1px 4px;
  opacity: 0.7;
}

.search-fade-enter-active, .search-fade-leave-active {
  transition: opacity 0.25s ease;
}
.search-fade-enter-from, .search-fade-leave-to {
  opacity: 0;
}
</style>
