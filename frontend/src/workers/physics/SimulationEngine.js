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
    onTick,
    onEnd
}) {
    const manualVelDecay = 0.3;

    // 1. Inicializa a simulação
    const simulation = d3.forceSimulation(nodesData, 3)
        .alphaDecay(0.08)
        .velocityDecay(0.45);

    // 2. Registro de Forças (Arsenal Premium - d3-force-registry inspired)
    const registry = {
        // Arestas: O "esqueleto" que mantém a estrutura
        link: d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                const sDeg = nodeDegrees.get(link.source.id) || 0;
                const tDeg = nodeDegrees.get(link.target.id) || 0;
                return (sDeg > 3 && tDeg > 3) ? 250 : 35;
            })
            .strength(1.0),
        
        // Repulsão Base (ManyBody): Evita sobreposição imediata
        charge: d3.forceManyBody().strength(-300).distanceMax(2000),
        
        // 🧲 Força Magnética (Inverso do Quadrado): Gera o agrupamento orgânico
        magnetic: d3.forceManyBody().strength(d => {
            return (d.weight || 1.0) * -150; 
        }).distanceMin(20).distanceMax(800),

        // ⭕ Força Radial: Cria o efeito de "Órbita" ao redor do centro (0,0,0)
        radial: d3.forceRadial(200, 0, 0, 0).strength(0.1),
        
        // 🧱 Força de Limite (Boundary): Mantém o cosmos contido em uma esfera
        limit: d3.forceRadial(0, 0, 0, 0).strength(0.01),

        center: d3.forceCenter(0, 0, 0).strength(0.01),
        
        collide: d3.forceCollide(node => {
            const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 18) : (nodeDegrees.get(node.id) || 0);
            return (1 + Math.pow(importance, 0.5) * 4.0) + 6;
        })
    };

    // 3. Aplica as forças do registro na simulação
    Object.keys(registry).forEach(key => {
        simulation.force(key, registry[key]);
    });

    let tickCount = 0;

    simulation.on('tick', () => {
        // Integração Z Manual
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
    
    simulation.on('end', () => {
        if (onEnd) onEnd(nodesData);
    });

    // 4. Retorna a interface de controle (API do Registry)
    return {
        simulation,
        updateForce: (name, params) => {
            const force = registry[name];
            if (!force) return;
            
            Object.keys(params).forEach(key => {
                if (typeof force[key] === 'function') {
                    force[key](params[key]);
                }
            });
            
            // "Acorda" a simulação para aplicar a mudança
            simulation.alpha(0.3).restart();
        }
    };
}
