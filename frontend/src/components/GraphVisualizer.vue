<script setup>
import { onMounted, ref, watch, onUnmounted } from 'vue'
import * as d3 from 'd3'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  nodes: { type: Array, default: () => [] },
  edges: { type: Array, default: () => [] }
})

const svgRef = ref(null)
const containerRef = ref(null)
let simulation = null
let g = null
let svg = null

const initGraph = () => {
  if (!svgRef.value || props.nodes.length === 0) return

  const width = containerRef.value.clientWidth
  const height = containerRef.value.clientHeight

  svg = d3.select(svgRef.value)
    .attr('width', '100%')
    .attr('height', '100%')
    .attr('viewBox', `0 0 ${width} ${height}`)

  svg.selectAll("*").remove() // Limpa antes de reconstruir

  // Definições de Filtros (Glow)
  const defs = svg.append('defs')
  const filter = defs.append('filter')
    .attr('id', 'glow')
    .attr('x', '-50%')
    .attr('y', '-50%')
    .attr('width', '200%')
    .attr('height', '200%')

  filter.append('feGaussianBlur')
    .attr('stdDeviation', '2.5')
    .attr('result', 'coloredBlur')

  const feMerge = filter.append('feMerge')
  feMerge.append('feMergeNode').attr('in', 'coloredBlur')
  feMerge.append('feMergeNode').attr('in', 'SourceGraphic')

  g = svg.append('g')

  // Zoom behavior
  const zoom = d3.zoom()
    .scaleExtent([0.1, 4])
    .on('zoom', (event) => {
      g.attr('transform', event.transform)
    })

  svg.call(zoom)

  simulation = d3.forceSimulation(props.nodes)
    .force('link', d3.forceLink(props.edges).id(d => d.id).distance(120))
    .force('charge', d3.forceManyBody().strength(-400))
    .force('center', d3.forceCenter(width / 2, height / 2))
    .force('collision', d3.forceCollide().radius(25))

  const link = g.append('g')
    .attr('class', 'links')
    .selectAll('line')
    .data(props.edges)
    .enter().append('line')
    .attr('stroke', 'rgba(59, 130, 246, 0.15)')
    .attr('stroke-width', 1)

  const node = g.append('g')
    .attr('class', 'nodes')
    .selectAll('g')
    .data(props.nodes)
    .enter().append('g')
    .call(d3.drag()
      .on('start', dragstarted)
      .on('drag', dragged)
      .on('end', dragended))

  // Círculo LUMINOSO (Estrela)
  node.append('circle')
    .attr('r', 6)
    .attr('fill', 'var(--primary)')
    .attr('filter', 'url(#glow)')
    .attr('class', 'node-circle')

  // Label sutil
  node.append('text')
    .text(d => d.name || d.id)
    .attr('x', 10)
    .attr('y', 4)
    .attr('class', 'node-label')

  simulation.on('tick', () => {
    link.attr('x1', d => d.source.x)
        .attr('y1', d => d.source.y)
        .attr('x2', d => d.target.x)
        .attr('y2', d => d.target.y)

    node.attr('transform', d => `translate(${d.x}, ${d.y})`)
  })

  function dragstarted(event, d) {
    if (!event.active) simulation.alphaTarget(0.3).restart()
    d.fx = d.x; d.fy = d.y
  }
  function dragged(event, d) {
    d.fx = event.x; d.fy = event.y
  }
  function dragended(event, d) {
    if (!event.active) simulation.alphaTarget(0)
    d.fx = null; d.fy = null
  }
}

// Watch para recarregar o grafo se os dados mudarem
watch(() => [props.nodes, props.edges], () => {
  initGraph()
}, { deep: true })

onMounted(() => {
  initGraph()

  // Listener para destaque em tempo real disparado pelo Maestro
  EventsOn('node:highlight', (nodeId) => {
    d3.selectAll('.node-circle')
      .filter(d => d.id === nodeId)
      .transition().duration(400)
      .attr('r', 15)
      .style('fill', '#fff')
      .transition().duration(2000)
      .attr('r', 6)
      .style('fill', 'var(--primary)')
  })
})

const resetZoom = () => {
  svg.transition().duration(750).call(
    d3.zoom().transform, 
    d3.zoomIdentity
  )
}
</script>

<template>
  <div class="graph-wrapper animate-fade-in" ref="containerRef">
    <svg ref="svgRef" class="main-svg"></svg>
    
    <!-- Controles Glassmorphism -->
    <div class="graph-ui glass">
      <div class="ui-header">
        <span class="pulse"></span>
        <h3>Conhecimento Obsidian</h3>
      </div>
      <div class="ui-actions">
        <button @click="resetZoom" class="action-btn" title="Centralizar">
          🎯 <span>RESET VIEW</span>
        </button>
        <div class="stat-item">
          <span class="val">{{ nodes.length }}</span>
          <span class="lab">NOTAS</span>
        </div>
      </div>
    </div>

    <!-- Background Imersivo -->
    <div class="graph-bg"></div>
  </div>
</template>

<style scoped>
.graph-wrapper {
  width: 100%;
  height: 100vh;
  background: var(--bg-dark);
  position: relative;
  overflow: hidden;
}

.main-svg {
  position: relative;
  z-index: 2;
  cursor: grab;
}

.main-svg:active { cursor: grabbing; }

/* Node Styling */
:deep(.node-label) {
  font-family: 'Outfit', sans-serif;
  font-size: 10px;
  fill: rgba(255, 255, 255, 0.4);
  pointer-events: none;
  font-weight: 500;
  letter-spacing: 0.5px;
  transition: opacity 0.3s, fill 0.3s;
}

:deep(g:hover .node-label) {
  fill: white;
  font-size: 12px;
  opacity: 1;
}

:deep(.node-circle) {
  transition: r 0.3s, fill 0.3s;
}

/* UI Panel */
.graph-ui {
  position: absolute;
  top: 2rem;
  left: 2rem;
  z-index: 10;
  padding: 1.2rem;
  border-radius: 20px;
  min-width: 220px;
  border: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ui-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ui-header h3 {
  font-size: 0.75rem;
  font-weight: 800;
  letter-spacing: 2px;
  text-transform: uppercase;
  color: var(--primary);
  margin: 0;
}

.pulse {
  width: 6px;
  height: 6px;
  background: var(--primary);
  border-radius: 50%;
  box-shadow: 0 0 8px var(--primary);
  display: inline-block;
}

.ui-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.action-btn {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: white;
  padding: 8px 12px;
  border-radius: 10px;
  font-size: 0.6rem;
  font-weight: 800;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 6px;
  transition: all 0.3s;
}

.action-btn:hover {
  background: var(--primary);
  border-color: var(--primary);
  transform: translateY(-2px);
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
}

.val {
  font-size: 1.2rem;
  font-weight: 900;
  color: white;
  line-height: 1;
}

.lab {
  font-size: 0.55rem;
  font-weight: 800;
  color: var(--text-dim);
  letter-spacing: 1px;
}

/* Background Imersivo */
.graph-bg {
  position: absolute;
  top: 0; left: 0; right: 0; bottom: 0;
  background: radial-gradient(circle at center, rgba(59, 130, 246, 0.05) 0%, transparent 70%);
  pointer-events: none;
  z-index: 1;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.animate-fade-in {
  animation: fadeIn 1s ease-out;
}
</style>
