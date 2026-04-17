/**
 * 🧪 DataProcessor — O Alquimista de Dados
 * 
 * Responsável por preparar os dados brutos para o motor de física.
 * Gerencia nascimento esférico, detecção de hubs e mapeamento celestial.
 */

const HUB_PHYS_LIMIT = 40;

export function bootstrapNodes(nodes, oldMap) {
    return nodes.map(n => {
        const existing = oldMap.get(n.id);
        if (existing) return Object.assign(existing, n);

        // Nascimento esférico 3D (distribuição volumétrica polar)
        const r = Math.pow(Math.random(), 1 / 3) * 3000;
        const theta = Math.acos(2 * Math.random() - 1);
        const phi = 2 * Math.PI * Math.random();
        
        return {
            ...n,
            x: r * Math.sin(theta) * Math.cos(phi),
            y: r * Math.sin(theta) * Math.sin(phi),
            z: r * Math.cos(theta),
            vz: 0
        };
    });
}

/**
 * Garante que todos os nós possuam coordenadas Z válidas (Reparo Celestial)
 */
export function repairCoordinates(nodesData) {
    nodesData.forEach(n => {
        if (n.z === null || n.z === undefined) {
            const r = Math.pow(Math.random(), 1 / 3) * 8000;
            const theta = Math.acos(2 * Math.random() - 1);
            n.z = r * Math.cos(theta);
        }
        if (n.vz === null || n.vz === undefined) n.vz = 0;
    });
}

export function processDegrees(links, idMap) {
    const nodeDegrees = new Map();
    links.forEach(l => {
        const sid = typeof l.source === 'object' ? l.source.id : l.source;
        const tid = typeof l.target === 'object' ? l.target.id : l.target;
        nodeDegrees.set(sid, (nodeDegrees.get(sid) || 0) + 1);
        nodeDegrees.set(tid, (nodeDegrees.get(tid) || 0) + 1);
    });

    const validLinks = links.filter(l => {
        const sid = typeof l.source === 'object' ? l.source.id : l.source;
        const tid = typeof l.target === 'object' ? l.target.id : l.target;
        if (!idMap.has(sid) || !idMap.has(tid)) return false;

        const sDeg = nodeDegrees.get(sid) || 0;
        const tDeg = nodeDegrees.get(tid) || 0;
        return sDeg <= HUB_PHYS_LIMIT && tDeg <= HUB_PHYS_LIMIT;
    });

    return { nodeDegrees, validLinks };
}

export function computeCommunityCenters(nodesData) {
    const communityMap = new Map();
    nodesData.forEach(n => {
        if (n.community !== undefined && n.community !== null) {
            if (!communityMap.has(n.community)) communityMap.set(n.community, []);
            communityMap.get(n.community).push(n);
        }
    });

    const communityCenters = new Map();
    const numC = Math.max(communityMap.size, 1);
    const golden = Math.PI * (Math.sqrt(5) - 1);
    const gR = 45000; // SUPERNOVA expansion

    Array.from(communityMap.keys()).forEach((cid, i) => {
        const yNorm = 1 - (i / Math.max(numC - 1, 1)) * 2;
        const rAtY = Math.sqrt(1 - yNorm * yNorm);
        const angle = golden * i;
        communityCenters.set(cid, {
            x: Math.cos(angle) * rAtY * gR,
            y: yNorm * gR,
            z: Math.sin(angle) * rAtY * gR
        });
    });

    return communityCenters;
}

export function mapHierarchy(nodesData, idMap) {
    const parentMap = new Map();
    nodesData.forEach(n => {
        if (n.parent_gravity_id && idMap.has(n.parent_gravity_id)) {
            parentMap.set(n.id, idMap.get(n.parent_gravity_id));
        }
    });
    return parentMap;
}
