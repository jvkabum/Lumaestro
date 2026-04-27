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
    onTick,
    onEnd
}) {
    const HUB_PHYS_LIMIT = 40;
    const velocityDecay = 0.35;
    const manualVelDecay = 0.3; // Para o eixo Z manual

    // 1. Inicializa a simulação
    const simulation = d3.forceSimulation(nodesData, 3)
        .alphaDecay(0.02)       // ← Mais tempo para expandir (v18.15)
        .velocityDecay(0.3);    // ← Menos fricção para movimentos mais amplos

    // 2. Registro de Forças (Arsenal Premium - d3-force-registry inspired)
    const registry = {
        // 1. Força de Elástico (Links) - Estilo Dente-de-Leão com Lógica Semântica (Mixer Dev)
        link: d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                const sDeg = nodeDegrees.get(link.source.id) || 0;
                const tDeg = nodeDegrees.get(link.target.id) || 0;
                if (sDeg > 3 && tDeg > 3) return 200;
                return 25;
            })
            .strength(link => {
                // 🧠 [MAGNETISMO INVISÍVEL] Vínculos semânticos puxam de leve
                if (link['edge-type'] === 'semantic') return 0.1;
                return 1.0;
            }),

        // 2. Força Customizada (Expansão de Clusters e Z-Push) - RESTAURADA
        custom: forceAll(communityCenters, parentMap),

        // 3. Repulsão (ManyBody) - Essencial para o formato visual de exploração (Mixer Dev)
        charge: d3.forceManyBody().strength(d => -4200).distanceMax(10000),

        // 🧲 Força Magnética, Radial e Limit (Restauradas do Main para controle UI, iniciam zeradas/suaves)
        magnetic: d3.forceManyBody().strength(d => (d.weight || 1.0) * -15).distanceMin(20).distanceMax(800),
        radial: d3.forceRadial(200, 0, 0, 0).strength(0),
        limit: d3.forceRadial(0, 0, 0, 0).strength(0),

        // 4. Centro Global
        center: d3.forceCenter(0, 0, 0).strength(0.01),

        // 5. Colisão física (Mixer Dev)
        collide: d3.forceCollide(node => {
            const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (nodeDegrees.get(node.id) || 0);
            return (1 + Math.pow(importance, 0.5) * 3.5) + 6;
        })
    };

    // 3. Aplica as forças do registro na simulação
    Object.keys(registry).forEach(key => {
        simulation.force(key, registry[key]);
    });

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

    // Restaurando evento 'end' do Main
    simulation.on('end', () => {
        if (onEnd) onEnd(nodesData);
    });

    // 4. Retorna a interface de controle (API do Registry - Mixer Main)
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
