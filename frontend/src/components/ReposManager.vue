<script setup>
import { onMounted, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useOrchestratorStore } from '../stores/orchestrator'
import { useSettingsProjects } from '../composables/useSettingsProjects'
import { GetConfig, ToggleProjectCodeRAG, SetWorkspace, GetWorkspace, UnlinkProject } from '../../wailsjs/go/core/App'

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

const handleUnlinkProject = async (proj) => {
  const confirmed = await orchestrator.confirm({
    title: 'DISSOLVER VÍNCULO',
    message: `Deseja remover o vínculo de "${proj.core_node}" do Nexus?\n\nA matéria física (arquivos) permanecerá intacta no seu disco, mas a galáxia não orbitará mais o Lumaestro.`,
    type: 'warning',
    confirmText: 'DISSOLVER AGORA',
    cancelText: 'ABORTAR'
  })

  if (confirmed) {
    try {
      await UnlinkProject(proj.path)
      const cfg = await GetConfig()
      if (cfg) store.config = cfg
      store.notify(`🛰️ Vínculo dissolvido: ${proj.core_node}`, "success")
    } catch (err) {
      store.notify(`❌ Falha ao desvincular: ${err}`, "error")
    }
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
             class="satellite-card animate-fade-in"
             :class="{ 'neo-selected': isCurrentWS(proj.path) }"
             @click="handleQuickSelect(proj)"
           >
             <!-- CARD HEADER: BRAND & DISSOLVE -->
             <div class="sat-header">
               <div class="sat-brand">
                 <span class="sat-icon-float">🪐</span>
                 <h4 class="sat-node-name">{{ proj.core_node }}</h4>
               </div>
               <button class="btn-dissolve-mini" @click.stop="handleUnlinkProject(proj)" title="Dissolver Vínculo">
                 <span class="dissolve-icon">🔗</span>
               </button>
             </div>

             <!-- CARD BODY: PATH & BADGE -->
             <div class="sat-body">
               <div class="sat-path-display">
                 <span class="folder-icon">📂</span>
                 <span class="path-text">{{ proj.path }}</span>
               </div>
               
               <div class="sat-badge-container">
                 <div class="sat-badge-premium" :class="proj.include_code ? 'neo-active' : 'neo-docs'" @click.stop="handleToggleCode(proj)">
                   <span class="badge-dot"></span>
                   {{ proj.include_code ? 'LIGHTNING CODE' : 'ARCHIVE DOCS' }}
                 </div>
               </div>
             </div>

             <!-- CARD FOOTER: ACTION -->
             <div class="sat-footer">
                <button class="btn-enter-nebula" :disabled="isCurrentWS(proj.path)">
                   <span v-if="isCurrentWS(proj.path)">TRANSITANDO AGORA...</span>
                   <span v-else>ENTRAR NA NEBULOSA ⚡</span>
                </button>
             </div>
             
             <div v-if="isCurrentWS(proj.path)" class="ws-active-aura"></div>
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

/* 🪐 Satellite Card Premium Redesign */
.satellites-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(350px, 1fr));
  gap: 24px;
}

.satellite-card {
  background: rgba(15, 12, 28, 0.6);
  backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 24px;
  padding: 24px;
  cursor: pointer;
  position: relative;
  transition: all 0.5s cubic-bezier(0.19, 1, 0.22, 1);
  display: flex;
  flex-direction: column;
  gap: 20px;
  overflow: hidden;
}

.satellite-card::before {
  content: '';
  position: absolute;
  inset: 0;
  background: radial-gradient(circle at top right, rgba(139, 92, 246, 0.1), transparent);
  opacity: 0;
  transition: opacity 0.5s;
}

.satellite-card:hover {
  transform: translateY(-8px);
  border-color: rgba(139, 92, 246, 0.4);
  box-shadow: 0 20px 40px rgba(0, 0, 0, 0.4), 0 0 20px rgba(139, 92, 246, 0.1);
}

.satellite-card:hover::before {
  opacity: 1;
}

.sat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  z-index: 1;
}

.sat-brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sat-icon-float {
  font-size: 1.8rem;
  filter: drop-shadow(0 0 10px rgba(255, 165, 0, 0.3));
  animation: floatOrbital 3s ease-in-out infinite;
}

@keyframes floatOrbital {
  0%, 100% { transform: translateY(0) rotate(0deg); }
  50% { transform: translateY(-5px) rotate(10deg); }
}

.sat-node-name {
  font-size: 1.2rem;
  font-weight: 900;
  letter-spacing: 2px;
  text-transform: uppercase;
  color: #fff;
  margin: 0;
}

.btn-dissolve-mini {
  background: rgba(239, 68, 68, 0.05);
  border: 1px solid rgba(239, 68, 68, 0.1);
  color: #ef4444;
  width: 38px;
  height: 38px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.3s;
  opacity: 0.3;
}

.satellite-card:hover .btn-dissolve-mini {
  opacity: 1;
}

.btn-dissolve-mini:hover {
  background: #ef4444;
  color: #fff;
  transform: scale(1.1) rotate(-15deg);
  box-shadow: 0 0 15px rgba(239, 68, 68, 0.4);
}

.sat-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  z-index: 1;
}

.sat-path-display {
  background: rgba(255, 255, 255, 0.03);
  padding: 12px 16px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.folder-icon { font-size: 1.1rem; }

.path-text {
  font-size: 0.75rem;
  font-family: 'JetBrains Mono', monospace;
  color: var(--p-text-dim);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sat-badge-premium {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 20px;
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 1px;
  text-transform: uppercase;
  transition: all 0.3s;
}

.sat-badge-premium.neo-active {
  background: rgba(16, 185, 129, 0.1);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #10b981;
}

.sat-badge-premium.neo-docs {
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
  color: #f59e0b;
}

.badge-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  box-shadow: 0 0 8px currentColor;
}

.sat-footer {
  margin-top: auto;
  z-index: 1;
}

.btn-enter-nebula {
  width: 100%;
  background: rgba(139, 92, 246, 0.1);
  border: 1px solid rgba(139, 92, 246, 0.3);
  color: #a855f7;
  padding: 14px;
  border-radius: 16px;
  font-size: 0.8rem;
  font-weight: 900;
  letter-spacing: 2px;
  cursor: pointer;
  transition: all 0.4s;
}

.btn-enter-nebula:hover:not(:disabled) {
  background: linear-gradient(135deg, #a855f7, #6366f1);
  color: #fff;
  transform: scale(1.02);
  box-shadow: 0 10px 25px rgba(139, 92, 246, 0.3);
}

.btn-enter-nebula:disabled {
  background: rgba(16, 185, 129, 0.1);
  border-color: rgba(16, 185, 129, 0.3);
  color: #10b981;
  cursor: default;
}

.neo-selected {
  border-color: rgba(168, 85, 247, 0.6) !important;
  background: rgba(168, 85, 247, 0.05) !important;
}

.ws-active-aura {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 4px;
  background: linear-gradient(90deg, transparent, #a855f7, transparent);
  animation: auraFlow 3s linear infinite;
}

@keyframes auraFlow {
  0% { opacity: 0.3; filter: blur(2px); }
  50% { opacity: 1; filter: blur(5px); }
  100% { opacity: 0.3; filter: blur(2px); }
}
</style>
