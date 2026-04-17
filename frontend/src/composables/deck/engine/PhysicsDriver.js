import PhysicsWorker from '../../../workers/physicsWorker?worker';

/**
 * 🏎️ PhysicsDriver — O Motor de Física
 * 
 * Responsável pelo ciclo de vida do WebWorker, recepção de TICKS 
 * e sincronização das coordenadas físicas de volta para a VRAM via nodeMap.
 */
export function usePhysicsDriver() {
    let physicsWorker = null;

    const initPhysics = (nodes, links) => {
        physicsWorker = new PhysicsWorker();
        physicsWorker.postMessage({ 
            type: 'INIT', 
            payload: { nodes, links } 
        });
        return physicsWorker;
    };

    const updatePhysicsData = (nodes, links) => {
        if (!physicsWorker) return;
        physicsWorker.postMessage({
            type: 'UPDATE_DATA',
            payload: { nodes, links }
        });
    };

    const syncPositions = (positions, nodeMap) => {
        if (!positions || !nodeMap) return;
        for (let i = 0; i < positions.length; i++) {
            const p = positions[i];
            const node = nodeMap.get(String(p.id));
            if (node) {
                node.x = p.x;
                node.y = p.y;
                node.z = p.z;
            }
        }
    };

    // 🎯 Drag Sincronizado
    const startDrag = (nodeId) => {
        physicsWorker?.postMessage({ type: 'DRAG_START', payload: { nodeId } });
    };

    const handleDrag = (nodeId, x, y, z) => {
        physicsWorker?.postMessage({ type: 'DRAG', payload: { nodeId, x, y, z } });
    };

    const endDrag = (nodeId) => {
        physicsWorker?.postMessage({ type: 'DRAG_END', payload: { nodeId } });
    };

    const terminatePhysics = () => {
        physicsWorker?.terminate();
        physicsWorker = null;
    };

    return { 
        initPhysics, 
        updatePhysicsData, 
        syncPositions, 
        startDrag, 
        handleDrag, 
        endDrag, 
        terminatePhysics 
    };
}
