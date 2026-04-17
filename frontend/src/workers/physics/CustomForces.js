/**
 * 🌌 CustomForces — As Leis do Cosmos
 * 
 * Define forças não-padrão para o grafo: 
 * - Cluster Gravity (Atração por comunidades)
 * - Hierarchy Gravity (Atração por pais)
 * - Z-Push (Repulsão no eixo Z)
 */

export function forceAll(communityCenters, parentMap) {
    let nodes;

    function force(alpha) {
        for (let i = 0; i < nodes.length; i++) {
            const node = nodes[i];

            // 1. GRAVIDADE DE CLUSTER (Pull para o centro da galáxia)
            const center = communityCenters.get(node.community);
            if (center) {
                const s = 0.005 * alpha;
                node.vx += (center.x - node.x) * s;
                node.vy += (center.y - node.y) * s;
                node.vz += (center.z - node.z) * s;
            }

            // 2. GRAVIDADE HIERÁRQUICA (Soft Pull para o Pai)
            const parent = parentMap.get(node.id);
            if (parent) {
                const s = 0.005 * alpha;
                node.vx += (parent.x - node.x) * s;
                node.vy += (parent.y - node.y) * s;
                node.vz += (parent.z - node.z) * s;
            }

            // 3. REPULSÃO Z CUSTOMIZADA (Evita o achatamento 3D)
            const pushZ = 25 * alpha;
            for (let j = i + 1; j < nodes.length; j++) {
                const other = nodes[j];
                const dz = (node.z || 0) - (other.z || 0);
                const absZ = Math.abs(dz);
                if (absZ < 400) {
                    const sign = dz >= 0 ? 1 : -1;
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
