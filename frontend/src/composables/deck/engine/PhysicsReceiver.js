/**
 * 📡 PhysicsReceiver — O Receptor de Telemetria
 * 
 * Responsável por processar as mensagens vindas do WebWorker de Física.
 * Sincroniza posições e realiza a "auto-cura" de links (referências perdidas).
 */
export function usePhysicsReceiver({ nodeMap, currentLinksRef, syncPositions, onUpdate }) {
    
    const setupReceiver = (worker) => {
        if (!worker) return;

        worker.onmessage = (event) => {
            const { type, payload } = event.data;
            
            if (type === 'TICK') {
                // 1. Sincroniza coordenadas X, Y, Z O(1)
                syncPositions(payload.positions, nodeMap);
                
                // 2. Auto-cura de links (Verifica referências source/target)
                currentLinksRef.value.forEach(link => {
                    if (!link.sourceObj || !link.targetObj) {
                        const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
                        const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
                        if (!link.sourceObj) link.sourceObj = nodeMap.get(sid);
                        if (!link.targetObj) link.targetObj = nodeMap.get(tid);
                    }
                });

                // 3. Notifica o Maestro para redesenhar
                if (onUpdate) onUpdate();
            }
        };
    };

    return { setupReceiver };
}
