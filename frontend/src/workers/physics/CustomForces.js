/**
 * 🌌 CustomForces — As Leis do Cosmos
 * 
 * Define forças não-padrão para o grafo: 
 * - Cluster Gravity (Atração por comunidades)
 * - Hierarchy Gravity (Atração por pais)
 * - Z-Push (Repulsão no eixo Z)
 */

// Configuração Centralizada de Tunagem (v19.0 - Expansão Ampla)
const CONFIG = {
    clusterRepulsion: 0.10,
    hierarchyMaxDist: 750,
    zRepulsionThreshold: 150,
    zRepulsionStrength: 10
};

export function forceAll(communityCenters, parentMap) {
    let nodes;

    function force(alpha) {
        const sCluster = CONFIG.clusterRepulsion * alpha;
        const sZ = CONFIG.zRepulsionStrength * alpha;

        for (let i = 0; i < nodes.length; i++) {
            const node = nodes[i];

            // 1. EXPANSÃO RADIAL DE CLUSTER (Anti-Gravidade Dinâmica)
            const center = communityCenters.get(node.community);
            if (center) {
                const dx = node.x - center.x; 
                const dy = node.y - center.y;
                const dz = node.z - center.z;
                const distSq = dx * dx + dy * dy + dz * dz || 1;
                const dist = Math.sqrt(distSq);
                
                // Força repulsiva para espalhar os nós da comunidade no seu próprio setor
                const f = sCluster * 220 / (dist / 150 + 1); 
                node.vx += (dx / dist) * f;
                node.vy += (dy / dist) * f;
                node.vz += (dz / dist) * f;
            }

            // 2. BIAS HIERÁRQUICO (Expansão Floral em Árvore)
            // Empurra filhos para longe do pai orbital para formar uma linda copa aberta
            const parent = parentMap.get(node.id);
            if (parent) {
                const s = 0.08 * alpha;
                const dx = node.x - parent.x;
                const dy = node.y - parent.y;
                const dz = node.z - parent.z;
                const dist = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
                
                if (dist < CONFIG.hierarchyMaxDist) { 
                    const push = (CONFIG.hierarchyMaxDist - dist) / CONFIG.hierarchyMaxDist * s * 300;
                    node.vx += (dx / dist) * push;
                    node.vy += (dy / dist) * push;
                    node.vz += (dz / dist) * push;
                }
            }

            // 3. EXPANSÃO Z ESTRUTURADA (Profundidade Volumétrica Real)
            const radialDist = Math.sqrt(node.x * node.x + node.y * node.y) || 1;
            const zBias = (radialDist / 350) * sZ; 
            
            if (Math.abs(node.z || 0) < 120) {
                const direction = (node.z || 0) >= 0 ? 1 : -1;
                node.vz += direction * zBias * 12;
            }
        }
    }


    force.initialize = function (_n) {
        nodes = _n;
    };

    return force;
}
