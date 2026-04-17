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
        
        // 1. Força de Elástico (Links)
        .force('link', d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                const sType = link.source?.['document-type'] || 'chunk';
                const tType = link.target?.['document-type'] || 'chunk';
                if (sType === 'memory' || tType === 'memory') return 80;   // Órbita próxima
                return 350;  // Distância interestelar para notas
            })
            .strength(0.7)  // ← Rigidez da conexão (0.0 a 1.0)
        )

        // 2. Forças Celestiais (Custom)
        .force('cosmos', forceAll(communityCenters, parentMap))

        // 3. Repulsão (ManyBody)
        .force('charge', d3.forceManyBody().strength(d => {
            const importance = (d.pagerank && d.pagerank > 0) ? (d.pagerank * 15) : (nodeDegrees.get(d.id) || 0);
            const baseRepulsion = -400;
            return baseRepulsion - (importance * 60);  // ← Mais importante = mais repulsão
        }).distanceMax(3000))
        
        // 4. Centro Global (Mínimo)
        .force('center', d3.forceCenter(0, 0, 0).strength(0.01))

        // 5. Colisão física (impede sobreposição visual)
        .force('collide', d3.forceCollide(node => {
            const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (nodeDegrees.get(node.id) || 0);
            return (1 + Math.pow(importance, 0.5) * 4) + 10;  // ← Raio de colisão
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
