<script setup>
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useOrchestratorStore } from '../stores/orchestrator'
import { useSettingsProjects } from '../composables/useSettingsProjects'
import { GetConfig, ToggleProjectCodeRAG, SetWorkspace, GetWorkspace } from '../../wailsjs/go/core/App'

const store = useSettingsStore()
const orchestrator = useOrchestratorStore()
const { handleAddProject, handleSelectDirectory } = useSettingsProjects()

onMounted(async () => {
  const cfg = await GetConfig()
  if (cfg) store.config = cfg
})

const isCurrentWS = (path) => {
  if (!path || !orchestrator.workspace.path) return false
  return path.toLowerCase().replace(/\\/g, '/') === orchestrator.workspace.path.toLowerCase().replace(/\\/g, '/')
}

const handleQuickSelect = async (proj) => {
  try {
    await SetWorkspace(proj.path)
    const updatedWs = await GetWorkspace()
    orchestrator.workspace = updatedWs
    store.notify(`🚀 Órbita alterada: ${proj.core_node}`, "success")
  } catch (err) {
    store.notify(`❌ Falha na transição: ${err}`, "error")
  }
}

const handleToggleCode = async (proj) => {
  const res = await ToggleProjectCodeRAG(proj.path)
  if (res.success) {
    const cfg = await GetConfig()
    if (cfg) store.config = cfg
    store.notify(`⚡ Modo de análise atualizado: ${proj.core_node}`, "success")
  }
}

const errors = ref({ path: false, core: false })
const showErrorMessage = ref(false)

const validateAndAdd = async () => {
  errors.value.path = !store.repoPathInput
  errors.value.core = !store.coreNodeInput
  
  if (errors.value.path || errors.value.core) {
    showErrorMessage.value = true
    setTimeout(() => { showErrorMessage.value = false }, 3000)
    return
  }
  
  await handleAddProject()
}
</script>

<template>
  <div class="repos-manager-container animate-slide-up">
    <div class="settings-header">
      <div class="brand-badge pulse-aura">LUMAESTRO RADIAL</div>
      <h1 class="gradient-text">Aglomerados Estelares</h1>
      <p class="subtitle">Gerenciamento de repositórios locais e RAG radial.</p>
    </div>

    <section class="glass-panel">
      <h2 class="section-title">Injeção de Repositórios Radiais</h2>
      <p style="color: var(--p-text-dim); margin-bottom: 2rem; font-size: 0.9rem;">
        Injete pastas de projetos locais no Grafo do Lumaestro. Estes projetos formarão órbitas concêntricas independentes (RAG Radial) protegidas de poluição vetorial, orbitando seu respectivo <b>Nó Núcleo</b>.
      </p>

      <div class="mcp-restored-form" style="border: 1px solid rgba(139, 92, 246, 0.3); background: rgba(139, 92, 246, 0.03); padding: 2.5rem; border-radius: 20px;">
         <div class="form-grid">
            <div class="premium-form-group">
               <label :class="{ 'label-error': errors.path }">Caminho Absoluto do Repositório</label>
               <div style="display: flex; gap: 10px;">
                 <input v-model="store.repoPathInput" :class="{ 'input-error': errors.path }" placeholder="Ex: C:\git\Lumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4); flex: 1;" @input="errors.path = false" />
                 <button @click="handleSelectDirectory" class="btn-glow-blue" style="flex: 0 0 auto; padding: 0 24px; font-size: 1.2rem; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5); border-radius: 14px;" title="Navegar e Escolher Pasta">
                   📁
                 </button>
               </div>
            </div>
            <div class="premium-form-group">
               <label :class="{ 'label-error': errors.core }">Nome do Núcleo Satélite (Core Node)</label>
               <input v-model="store.coreNodeInput" :class="{ 'input-error': errors.core }" placeholder="Ex: ProjetoLumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4);" @input="errors.core = false" />
            </div>
          </div>

          <!-- Code RAG Toggle Switch Premium -->
          <div class="sec-card" style="margin-top: 1rem; margin-bottom: 1.5rem; border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05); padding: 1.5rem 2.5rem; display: flex; align-items: center; justify-content: space-between;">
             <div class="sec-info" style="flex: 1;">
                <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #10b981;">Devorar Código Fonte (Code RAG)</h5>
                <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Ativando isto, além de .MD e Imagens, a IA irá ler, processar e gerar semânticas de todos os códigos .js, .go, .py e .ts.</p>
             </div>
             <label class="hybrid-toggle-maestro">
                <input type="checkbox" v-model="store.includeCodeToggle" />
                <span class="m-slider-sec" style="background: rgba(16, 185, 129, 0.1);"></span>
             </label>
          </div>

          <transition name="fade">
            <p v-if="showErrorMessage" class="error-text">⚠️ Preencha todos os campos obrigatórios para orbitar este projeto.</p>
          </transition>

          <button @click="validateAndAdd" :disabled="store.repoStatusMsg !== ''" class="btn-glow-blue" style="width: 100%; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5);">
             <span v-if="store.repoStatusMsg === ''">VINCULAR REPOSITÓRIO À SINFORNIA 🪐</span>
             <span v-else>{{ store.repoStatusMsg }}</span>
          </button>
       </div>

      <div style="margin-top: 4rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem;">
          <h3 style="font-size: 1rem; color: #fff; letter-spacing: 2px; margin: 0;">SISTEMAS SOLARES (Orquestrados)</h3>
          <button @click="handleAddProject" class="btn-refresh" title="Recarregar Órbitas">🔄</button>
        </div>
        
        <div v-if="!store.config.external_projects || store.config.external_projects.length === 0" style="color: var(--p-text-dim); text-align: center; padding: 3rem; border-radius: 12px; border: 1px dashed rgba(255,255,255,0.1); background: rgba(255,255,255,0.01);">
           O Universo ainda não possui outros projetos em órbita.
        </div>
        
        <div v-else class="satellites-grid">
           <div 
             v-for="proj in store.config.external_projects" 
             :key="proj.path" 
             class="satellite-card"
             :class="{ 'neo-selected': isCurrentWS(proj.path) }"
             @click="handleQuickSelect(proj)"
           >
             <div class="sat-core">
               <div class="sat-ring-icon">🪐</div>
               <h4 class="sat-node-name">{{ proj.core_node }}</h4>
               <div class="sat-badge" :class="proj.include_code ? 'neo-active' : 'neo-docs'" @click.stop="handleToggleCode(proj)" title="Clique para alternar entre Código e Documentação">
                 {{ proj.include_code ? '⚡ CODE RAG' : '📄 APENAS DOCS' }}
               </div>
             </div>
             
             <div class="sat-path-box">
               <span style="opacity: 0.6; margin-right: 8px;">📂</span>
               <span class="path-text">{{ proj.path }}</span>
             </div>

             <div class="sat-actions">
                <button class="btn-select-proj" :disabled="isCurrentWS(proj.path)">
                   {{ isCurrentWS(proj.path) ? 'ATIVO AGORA' : 'ENTRAR NO PROJETO' }}
                </button>
             </div>
             
             <div v-if="isCurrentWS(proj.path)" class="ws-indicator-pulse">
                SESSÃO ATIVA
             </div>
           </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
@import '../assets/css/Settings.css';

.repos-manager-container {
  padding: 40px;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

.input-error {
  border-color: #ef4444 !important;
  box-shadow: 0 0 10px rgba(239, 68, 68, 0.2);
  animation: shake 0.4s ease-in-out;
}

.label-error {
  color: #ef4444 !important;
}

.error-text {
  color: #ef4444;
  font-size: 0.85rem;
  font-weight: 700;
  margin-bottom: 1rem;
  text-align: center;
}

@keyframes shake {
  0%, 100% { transform: translateX(0); }
  25% { transform: translateX(-5px); }
  75% { transform: translateX(5px); }
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.3s;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}

.btn-refresh {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: #fff;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s;
}

.btn-refresh:hover {
  background: rgba(139, 92, 246, 0.2);
  border-color: rgba(139, 92, 246, 0.4);
  transform: rotate(180deg);
}

/* 🪐 Satellite Card Enhancements */
.satellite-card {
  cursor: pointer;
  position: relative;
  transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}

.satellite-card:hover {
  transform: translateY(-5px) scale(1.02);
  border-color: rgba(139, 92, 246, 0.5);
  box-shadow: 0 10px 30px rgba(139, 92, 246, 0.2);
}

.neo-selected {
  border-color: #a855f7 !important;
  background: rgba(168, 85, 247, 0.08) !important;
  box-shadow: 0 0 20px rgba(168, 85, 247, 0.3) !important;
}

.ws-indicator-pulse {
  position: absolute;
  top: 10px;
  right: 10px;
  font-size: 8px;
  font-weight: 900;
  color: #a855f7;
  letter-spacing: 1px;
  background: rgba(168, 85, 247, 0.1);
  padding: 4px 8px;
  border-radius: 4px;
  animation: pulse-ws 2s infinite;
}

.path-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sat-actions {
  margin-top: 15px;
  padding-top: 15px;
  border-top: 1px solid rgba(255,255,255,0.05);
}

.btn-select-proj {
  width: 100%;
  background: rgba(139, 92, 246, 0.1);
  border: 1px solid rgba(139, 92, 246, 0.3);
  color: #a855f7;
  padding: 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1px;
  cursor: pointer;
  transition: all 0.3s;
}

.btn-select-proj:hover:not(:disabled) {
  background: #a855f7;
  color: #fff;
}

.btn-select-proj:disabled {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.3);
  color: #34d399;
  cursor: default;
}
</style>
