import { bootstrapNodes, computeCommunityCenters, convertToBFSTree, mapHierarchy, processDegrees, repairCoordinates } from './physics/DataProcessor';
import { createSimulation } from './physics/SimulationEngine';

/**
 * 🎼 physicsWorker — O Maestro da Física Atômica (v15.0)
 * 
 * Fachada ultra-leve que orquestra os especialistas matemáticos:
 * - DataProcessor (Engenharia de Dados)
 * - SimulationEngine (Motor D3-Force-3D)
 */

let engine = null; // Agora guarda o objeto { simulation, updateForce }
let nodesData = [];
let linksData = [];

self.onmessage = function (event) {
    const { type, payload } = event.data;

    if (type === 'INIT' || type === 'UPDATE_DATA') {
        const { nodes, links } = payload;

        // 1. Preparação e Nascimento de Dados
        const oldMap = new Map();
        nodesData.forEach(n => oldMap.set(n.id, n));
        nodesData = bootstrapNodes(nodes, oldMap);
        repairCoordinates(nodesData);

        const idMap = new Map();
        nodesData.forEach(n => idMap.set(n.id, n));

        // 2. Extração de Metadados Matemáticos (Graus, Centros, Hierarquia)
        const { nodeDegrees, validLinks } = processDegrees(links, idMap);

        // 🌟 3. A PODAGEM MÁGICA (BFS MST)
        linksData = convertToBFSTree(nodesData, validLinks, nodeDegrees);

        self.postMessage({ type: 'PRUNED_LINKS', payload: { links: linksData } });

        // 3. Reinicialização do Motor com Registry
        if (engine && engine.simulation) engine.simulation.stop();

        engine = createSimulation({
            nodesData,
            linksData,
            nodeDegrees,
            onTick: (nodes) => {
                const positions = nodes.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z || 0 }));
                self.postMessage({ type: 'TICK', payload: { positions } });
            },
            onEnd: (nodes) => {
                const positions = nodes.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z || 0 }));
                self.postMessage({ type: 'STABILIZED', payload: { positions } });
            }
        });

        engine.simulation.tick(100); // Warmup
        engine.simulation.alpha(0.6).restart();
    }

    else if (type === 'UPDATE_FORCE') {
        const { name, params } = payload;
        if (engine && engine.updateForce) {
            console.log(`[Physics] ⚡ Atualizando força: ${name}`, params);
            engine.updateForce(name, params);
        }
    }

    else if (type === 'DRAG_START') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node && engine) {
            engine.simulation.alphaTarget(0.3).restart();
            node.fx = node.x; node.fy = node.y; node.fz = node.z;
        }
    }
    else if (type === 'DRAG') {
        const { nodeId, x, y, z } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node) { node.fx = x; node.fy = y; node.fz = z; }
    }
    else if (type === 'DRAG_END') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node && engine) {
            engine.simulation.alphaTarget(0);
            node.fx = null; node.fy = null; node.fz = null;
        }
    }
};
