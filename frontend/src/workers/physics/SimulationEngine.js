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
        .alphaDecay(0.04)
        .velocityDecay(velocityDecay)
        
        // 1. Força de Elástico (Links)
        .force('link', d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                const sC = link.source?.community;
                const tC = link.target?.community;
                const sType = link.source?.['document-type'] || 'chunk';
                const tType = link.target?.['document-type'] || 'chunk';
                
                if (sType === 'memory' || tType === 'memory') return 120;
                return (sC === tC) ? 850 : 15000;
            })
            .strength(link => {
                const sC = link.source?.community;
                const tC = link.target?.community;
                return (sC === tC) ? 0.3 : 0.02;
            })
        )

        // 2. Forças Celestiais (Custom)
        .force('cosmos', forceAll(communityCenters, parentMap))

        // 3. Repulsão (ManyBody)
        .force('charge', d3.forceManyBody().strength(d => {
            const deg = nodeDegrees.get(d.id) || 0;
            const pr = (d.pagerank && d.pagerank > 0) ? (d.pagerank * 20) : deg;
            const baseRepulsion = -25000;
            
            if (deg > HUB_PHYS_LIMIT) return -185000;
            return baseRepulsion - (pr * 500); 
        }).distanceMax(80000))
        
        // 4. Colisão
        .force('collide', d3.forceCollide().radius(d => {
            const deg = nodeDegrees.get(d.id) || 0;
            const pr = (d.pagerank && d.pagerank > 0) ? (d.pagerank * 20) : deg;
            if (deg > HUB_PHYS_LIMIT) return 2500;
            return (1 + Math.pow(pr, 0.5) * 15) + 70;
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
