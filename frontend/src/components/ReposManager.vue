<script setup>
import { useSettingsStore } from '../stores/settings'
import { useSettingsProjects } from '../composables/useSettingsProjects'

const store = useSettingsStore()
const { handleAddProject, handleSelectDirectory } = useSettingsProjects()
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
              <label>Caminho Absoluto do Repositório</label>
              <div style="display: flex; gap: 10px;">
                <input v-model="store.repoPathInput" placeholder="Ex: C:\git\Lumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4); flex: 1;" />
                <button @click="handleSelectDirectory" class="btn-glow-blue" style="flex: 0 0 auto; padding: 0 24px; font-size: 1.2rem; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5); border-radius: 14px;" title="Navegar e Escolher Pasta">
                  📁
                </button>
              </div>
           </div>
           <div class="premium-form-group">
              <label>Nome do Núcleo Satélite (Core Node)</label>
              <input v-model="store.coreNodeInput" placeholder="Ex: ProjetoLumaestro" class="maestro-input" style="border-color: rgba(139, 92, 246, 0.4);" />
           </div>
         </div>

         <!-- Code RAG Toggle Switch Premium -->
         <div class="sec-card" style="margin-top: 1rem; margin-bottom: 2.5rem; border-color: rgba(16, 185, 129, 0.3); background: rgba(16, 185, 129, 0.05); padding: 1.5rem 2.5rem; display: flex; align-items: center; justify-content: space-between;">
            <div class="sec-info" style="flex: 1;">
               <h5 style="margin: 0; font-weight: 800; font-size: 1rem; color: #10b981;">Devorar Código Fonte (Code RAG)</h5>
               <p style="margin: 8px 0 0; font-size: 0.8rem; color: var(--p-text-dim);">Ativando isto, além de .MD e Imagens, a IA irá ler, processar e gerar semânticas de todos os códigos .js, .go, .py e .ts.</p>
            </div>
            <label class="hybrid-toggle-maestro">
               <input type="checkbox" v-model="store.includeCodeToggle" />
               <span class="m-slider-sec" style="background: rgba(16, 185, 129, 0.1);"></span>
            </label>
         </div>

         <button @click="handleAddProject" :disabled="store.repoStatusMsg !== ''" class="btn-glow-blue" style="width: 100%; background: linear-gradient(135deg, #a855f7, #6366f1); border: 1px solid rgba(168, 85, 247, 0.5);">
            <span v-if="store.repoStatusMsg === ''">VINCULAR REPOSITÓRIO À SINFORNIA 🪐</span>
            <span v-else>{{ store.repoStatusMsg }}</span>
         </button>
      </div>

      <div style="margin-top: 4rem;">
        <h3 style="font-size: 1rem; color: #fff; letter-spacing: 2px; margin-bottom: 1.5rem;">SISTEMAS SOLARES (Orquestrados)</h3>
        
        <div v-if="!store.config.external_projects || store.config.external_projects.length === 0" style="color: var(--p-text-dim); text-align: center; padding: 2rem; border-radius: 12px; border: 1px dashed rgba(255,255,255,0.1);">
           O Universo ainda não possui outros projetos em órbita.
        </div>
        
        <div v-else class="satellites-grid">
           <div v-for="proj in store.config.external_projects" :key="proj.path" class="satellite-card">
             <div class="sat-core">
               <div class="sat-ring-icon">🪐</div>
               <h4 class="sat-node-name">{{ proj.core_node }}</h4>
               <div class="sat-badge" :class="proj.include_code ? 'neo-active' : 'neo-docs'">
                 {{ proj.include_code ? '⚡ CODE RAG' : '📄 APENAS DOCS' }}
               </div>
             </div>
             
             <div class="sat-path-box">
               <span style="opacity: 0.6; margin-right: 8px;">📂</span>
               <span>{{ proj.path }}</span>
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
</style>
