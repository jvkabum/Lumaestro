import { bootstrapNodes, computeCommunityCenters, convertToBFSTree, mapHierarchy, processDegrees, repairCoordinates } from './physics/DataProcessor';
import { createSimulation } from './physics/SimulationEngine';

/**
 * 🎼 physicsWorker — O Maestro da Física Atômica (v15.0)
 * 
 * Fachada ultra-leve que orquestra os especialistas matemáticos:
 * - DataProcessor (Engenharia de Dados)
 * - SimulationEngine (Motor D3-Force-3D)
 */

let simulation = null;
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
        // Corta todas as linhas de teia redundantes e força o design 'Dente-de-Leão' do D3
        linksData = convertToBFSTree(nodesData, validLinks, nodeDegrees);

        // Envia a teia limpa de volta para o Visualizador Deck.gl parar de desenhar "Miojo" na GPU!
        self.postMessage({ type: 'PRUNED_LINKS', payload: { links: linksData } });

        const communityCenters = computeCommunityCenters(nodesData);
        const parentMap = mapHierarchy(nodesData, idMap);

        console.log(`[Physics] ${nodesData.length} nós, ${parentMap.size} com pai, ${communityCenters.size} comunidades, ${linksData.length} links válidos`);

        // 3. Reinicialização do Motor D3
        if (simulation) simulation.stop();

        simulation = createSimulation({
            nodesData,
            linksData,
            nodeDegrees,
            communityCenters,
            parentMap,
            onTick: (nodes) => {
                const positions = nodes.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z || 0 }));
                self.postMessage({ type: 'TICK', payload: { positions } });
            }
        });

        simulation.tick(100); // Warmup
        simulation.alpha(0.6).restart();
    }

    else if (type === 'DRAG_START') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node && simulation) {
            simulation.alphaTarget(0.3).restart();
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
        if (node && simulation) {
            simulation.alphaTarget(0);
            node.fx = null; node.fy = null; node.fz = null;
        }
    }
};
