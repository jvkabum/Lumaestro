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
        // 1. Força de Elástico (Links) - Espaçamento Cosmológico Dente-de-Leão
        link: d3.forceLink(linksData).id(d => d.id)
            .distance(link => {
                const getCelestial = (n) => n['celestial-type'] || (n.id.startsWith('planet:') ? 'planet' : (n.id.startsWith('galaxy:') ? 'galaxy' : 'moon'));
                const cS = getCelestial(link.source);
                const cT = getCelestial(link.target);
                const edgeType = link['edge-type'] || link.relation_type;

                // 🪐 Galáxia -> Planetas principais (Sistemas Solares Primários)
                // Distância ampla para cada diretório ter seu próprio espaço estelar
                if (cS === 'galaxy' || cT === 'galaxy') {
                    return 1400;
                }

                // 📁 Planeta -> Planeta (Subdiretórios)
                if (cS === 'planet' && cT === 'planet') {
                    return 650;
                }

                // 📄 Planeta -> Luas (Arquivos na pasta)
                if (edgeType === 'orbital') {
                    const sDeg = nodeDegrees.get(link.source.id) || 0;
                    const tDeg = nodeDegrees.get(link.target.id) || 0;
                    const deg = Math.max(sDeg, tDeg);
                    // Pastas densas abrem a corola do dente-de-leão mais amplamente
                    return 240 + Math.min(deg * 5, 350);
                }

                // 🧠 Sinapses Semânticas e Memórias
                // Distância maior e frouxa para não puxar galáxias distantes umas contra as outras
                if (edgeType === 'semantic' || edgeType === 'memory') {
                    return 900;
                }

                return 350;
            })
            .strength(link => {
                const edgeType = link['edge-type'] || link.relation_type;
                if (edgeType === 'semantic') return 0.015; // Muito suave: guia visual sem amassar clusters
                if (edgeType === 'memory') return 0.02;
                if (edgeType === 'orbital') return 0.22;   // Flexível para permitir expansão
                return 0.25;
            }),

        // 2. Força Customizada (Expansão de Clusters e Z-Push) - RESTAURADA
        custom: forceAll(communityCenters, parentMap),

        // 3. Repulsão Hierárquica Diferenciada (ManyBody)
        // Galáxias e planetas se repelem fortemente; arquivos florescem suavemente
        charge: d3.forceManyBody()
            .strength(d => {
                const c = d['celestial-type'] || (d.id.startsWith('planet:') ? 'planet' : (d.id.startsWith('galaxy:') ? 'galaxy' : (d.id.startsWith('asteroid:') ? 'asteroid' : 'moon')));
                if (c === 'galaxy') return -65000;
                if (c === 'solar-system') return -30000;
                if (c === 'planet') return -16000;
                if (c === 'moon') return -1400;
                if (c === 'asteroid') return -180;
                return -800;
            })
            .distanceMin(40)
            .distanceMax(35000),

        // 🧲 Força Magnética, Radial e Limit (iniciam suaves)
        magnetic: d3.forceManyBody().strength(d => (d.weight || 1.0) * -15).distanceMin(20).distanceMax(800),
        radial: d3.forceRadial(200, 0, 0, 0).strength(0),
        limit: d3.forceRadial(0, 0, 0, 0).strength(0),

        // 4. Centro Global suave
        center: d3.forceCenter(0, 0, 0).strength(0.008),

        // 5. Colisão física estendida com amortecimento de respiro
        collide: d3.forceCollide(node => {
            const celestial = node['celestial-type'] || (node.id.startsWith('planet:') ? 'planet' : (node.id.startsWith('galaxy:') ? 'galaxy' : (node.id.startsWith('asteroid:') ? 'asteroid' : 'moon')));
            let baseMass = node.mass || 4.0;
            if (celestial === 'galaxy') baseMass = 80.0;
            if (celestial === 'solar-system') baseMass = 45.0;
            if (celestial === 'planet') baseMass = 25.0;
            if (celestial === 'asteroid') baseMass = 2.0;

            const importance = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : (nodeDegrees.get(node.id) || 0);
            const visualRadius = (baseMass + Math.pow(importance, 0.5) * 1.5);
            
            // Margem de respiro proporcional para evitar nós amontoados
            if (celestial === 'galaxy') return visualRadius + 60;
            if (celestial === 'solar-system') return visualRadius + 40;
            if (celestial === 'planet') return visualRadius + 28;
            return visualRadius + 14;
        }).iterations(3)
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
                    // Se o valor é primitivo (número), converte para função constante para o D3
                    const val = params[key];
                    force[key](typeof val === 'function' ? val : val);
                }
            });

            // "Acorda" a simulação para aplicar a mudança
            simulation.alpha(0.5).restart();
        }
    };
}
