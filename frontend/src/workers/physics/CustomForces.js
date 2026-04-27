/**
 * 🌌 CustomForces — As Leis do Cosmos
 * 
 * Define forças não-padrão para o grafo: 
 * - Cluster Gravity (Atração por comunidades)
 * - Hierarchy Gravity (Atração por pais)
 * - Z-Push (Repulsão no eixo Z)
 */

// Configuração Centralizada de Tunagem (v18.11 - A/B Testing Enabled)
const CONFIG = {
    clusterRepulsion: 0.05,
    hierarchyMaxDist: 350,
    zRepulsionThreshold: 100,
    zRepulsionStrength: 5
};

export function forceAll(communityCenters, parentMap) {
    let nodes;

    function force(alpha) {
        const sCluster = CONFIG.clusterRepulsion * alpha;
        const sZ = CONFIG.zRepulsionStrength * alpha;

        for (let i = 0; i < nodes.length; i++) {
            const node = nodes[i];

            // 1. EXPANSÃO RADIAL DE CLUSTER (v18.15 - Anti-Gravidade Dinâmica)
            const center = communityCenters.get(node.community);
            if (center) {
                const dx = node.x - center.x; 
                const dy = node.y - center.y;
                const dz = node.z - center.z;
                const distSq = dx * dx + dy * dy + dz * dz || 1;
                const dist = Math.sqrt(distSq);
                
                // Força repulsiva baseada no inverso da distância (Suavizada)
                const f = sCluster * 150 / (dist / 100 + 1); 
                node.vx += (dx / dist) * f;
                node.vy += (dy / dist) * f;
                node.vz += (dz / dist) * f;
            }

            // 2. BIAS HIERÁRQUICO (Tree Expansion)
            const parent = parentMap.get(node.id);
            if (parent) {
                const s = 0.05 * alpha;
                const dx = node.x - parent.x;
                const dy = node.y - parent.y;
                const dz = node.z - parent.z;
                const dist = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
                
                if (dist < CONFIG.hierarchyMaxDist) { 
                    const push = (CONFIG.hierarchyMaxDist - dist) / CONFIG.hierarchyMaxDist * s * 200;
                    node.vx += (dx / dist) * push;
                    node.vy += (dy / dist) * push;
                    node.vz += (dz / dist) * push;
                }
            }

            // 3. EXPANSÃO Z ESTRUTURADA (v18.16 - Bias Esférico O(N))
            // Aplica um bias de profundidade baseado na distância radial XY para criar uma "esfera de neurônios"
            const radialDist = Math.sqrt(node.x * node.x + node.y * node.y) || 1;
            const zBias = (radialDist / 400) * sZ; 
            
            if (Math.abs(node.z || 0) < 40) {
                // Empurra suavemente para fora do plano XY com base na posição atual
                const direction = node.z >= 0 ? 1 : -1;
                node.vz += direction * zBias * 8;
            }
        }
    }


    force.initialize = function (_n) {
        nodes = _n;
    };

    return force;
}
