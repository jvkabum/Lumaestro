import * as d3 from 'd3-force-3d';
import { forceAll } from './CustomForces';

/**
 * ⚙️ SimulationEngine — O Motor de D3
 * 
 * Gerencia a instância da simulação d3-force-3d,
 * a integração manual do eixo Z e os ciclos de resfriamento (alpha).
 */

export function createSimulation({
    nodesData,
    linksData,
    nodeDegrees,
    communityCenters,
    parentMap,
    onTick
}) {
    const HUB_PHYS_LIMIT = 40;
    const velocityDecay = 0.35;
    const manualVelDecay = 0.3; // Para o eixo Z manual

    const simulation = d3.forceSimulation(nodesData, 3)
        .alphaDecay(0.08)       // ← Velocidade de "esfriamento" (convergência)
        .velocityDecay(0.45)    // ← Amortecimento do movimento (estabilidade)

        // 1. Força de Elástico (Links) - Estilo Dente-de-Leão (Árvore/Star)
        .force('link', d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                // Se ambos são nós centrais/importantes, afaste-os agressivamente (linhas compridas entre os núcleos)
                const sDeg = nodeDegrees.get(link.source.id) || 0;
                const tDeg = nodeDegrees.get(link.target.id) || 0;
                if (sDeg > 3 && tDeg > 3) return 200; // Reduzido de 500 para 200 (aproxima os clusters)
                
                // Se for um nó folha ligado a um núcleo, mantenha curto
                return 25; // Reduzido de 70 para 25 (encurta as hastes das folhas)
            })
            .strength(1.0)  // ← Tensão de chumbo: 1.0 forçará a distância ser obedecida rigorosamente!
        )

        // 2. Forças Celestiais (Custom)
        // DESATIVADO: A força direcional do cosmos estava gerando o efeito "Vassoura/Cone".
        // Para uma árvore radial perfeita 360º, queremos apenas repulsão natural (ManyBody).
        // .force('cosmos', forceAll(communityCenters, parentMap))

        // 3. Repulsão (ManyBody) - Essencial para o formato visual de exploração
        .force('charge', d3.forceManyBody().strength(d => {
            // Alta repulsão garante que as 'folhas' vizinhas no mesmo hub se isolem e fiquem perfeitamente espaçadas formando uma esfera
            return -500; 
        }).distanceMax(3000))
        
        // 4. Centro Global (Mínimo)
        .force('center', d3.forceCenter(0, 0, 0).strength(0.015))

        // 5. Colisão física (Dita o distanciamento exato entre as folhas)
        .force('collide', d3.forceCollide(node => {
            const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (nodeDegrees.get(node.id) || 0);
            return (1 + Math.pow(importance, 0.5) * 3.5) + 4;  // +4 gera um bom respiro visual
        }));

    let tickCount = 0;

    simulation.on('tick', () => {
        // Integração Z Manual (Bug fix para d3-force-3d v3)
        for (let i = 0; i < nodesData.length; i++) {
            const n = nodesData[i];
            if (n.fz !== undefined && n.fz !== null) {
                n.z = n.fz;
                n.vz = 0;
            } else {
                n.vz *= (1 - manualVelDecay);
                n.z = (n.z || 0) + n.vz;
            }
        }

        tickCount++;
        if (tickCount % 2 === 0 && onTick) {
            onTick(nodesData);
        }
    });

    return simulation;
}
