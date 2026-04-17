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
        for (let i = 0; i < nodes.length; i++) {
            const node = nodes[i];

            // 1. EXPANSÃO RADIAL DE CLUSTER (v18.9 - Anti-Gravidade)
            const center = communityCenters.get(node.community);
            if (center) {
                const s = CONFIG.clusterRepulsion * alpha;
                const dx = node.x - center.x; // Direção: Centro -> Nó (EMPURA PARA FORA)
                const dy = node.y - center.y;
                const dz = node.z - center.z;
                const dist = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
                
                // Força repulsiva constante para garantir expansão radial
                const force = s / (dist / 1000 + 1); 
                node.vx += dx * force;
                node.vy += dy * force;
                node.vz += dz * force;
            }

            // 2. BIAS HIERÁRQUICO (Tree Expansion)
            const parent = parentMap.get(node.id);
            if (parent) {
                const s = 0.02 * alpha;
                // Empurra o filho para longe do pai
                const dx = node.x - parent.x;
                const dy = node.y - parent.y;
                const dz = node.z - parent.z;
                const dist = Math.sqrt(dx * dx + dy * dy + dz * dz) || 1;
                
                if (dist < CONFIG.hierarchyMaxDist) { 
                    const push = (CONFIG.hierarchyMaxDist - dist) / CONFIG.hierarchyMaxDist * s * 150;
                    node.vx += (dx / dist) * push;
                    node.vy += (dy / dist) * push;
                    node.vz += (dz / dist) * push;
                }
            }

            // 3. REPULSÃO Z CUSTOMIZADA (Evita o achatamento 3D)
            const pushZ = CONFIG.zRepulsionStrength * alpha;
            for (let j = i + 1; j < nodes.length; j++) {
                const other = nodes[j];
                const dz = (node.z || 0) - (other.z || 0);
                const absZ = Math.abs(dz);
                if (absZ < CONFIG.zRepulsionThreshold) {
                    const sign = dz >= 0 ? 1 : -1;
                    node.vx += (Math.random() - 0.5) * pushZ; // Adiciona pequeno offset caótico (Z-fighting prevention)
                    node.vy += (Math.random() - 0.5) * pushZ;
                    node.vz += sign * pushZ;
                    other.vz -= sign * pushZ;
                }
            }
        }
    }


    force.initialize = function (_n) {
        nodes = _n;
    };

    return force;
}
