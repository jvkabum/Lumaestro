/**
 * 📡 PhysicsReceiver — O Receptor de Telemetria
 * 
 * Responsável por processar as mensagens vindas do WebWorker de Física.
 * Sincroniza posições e realiza a "auto-cura" de links (referências perdidas).
 */
export function usePhysicsReceiver({ nodeMap, currentLinksRef, syncPositions, resolveNode, onUpdate, onStabilized }) {
    
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
                        if (!link.sourceObj) link.sourceObj = (resolveNode ? resolveNode(sid, nodeMap) : null) || nodeMap.get(sid) || nodeMap.get(sid.toLowerCase());
                        if (!link.targetObj) link.targetObj = (resolveNode ? resolveNode(tid, nodeMap) : null) || nodeMap.get(tid) || nodeMap.get(tid.toLowerCase());
                    }
                });

                // 3. Notifica o Maestro para redesenhar
                if (onUpdate) onUpdate();
            }
            else if (type === 'PRUNED_LINKS') {
                // Recebendo a Árvore Limpa: cura referências de nós para os objetos vivos da memória
                const healedLinks = (payload.links || []).map(link => {
                    const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
                    const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
                    const sObj = (resolveNode ? resolveNode(sid, nodeMap) : null) || nodeMap.get(sid) || nodeMap.get(sid.toLowerCase());
                    const tObj = (resolveNode ? resolveNode(tid, nodeMap) : null) || nodeMap.get(tid) || nodeMap.get(tid.toLowerCase());
                    return {
                        ...link,
                        source: sid,
                        target: tid,
                        sourceObj: sObj,
                        targetObj: tObj
                    };
                }).filter(l => l.sourceObj && l.targetObj);

                currentLinksRef.value = healedLinks;
                console.log(`[PhysicsReceiver] Visão Limpa! Desenhando ${healedLinks.length} arestas isoladas.`);
                if (onUpdate) onUpdate();
            }
            else if (type === 'STABILIZED') {
                console.log(`[PhysicsReceiver] ✨ Física Estabilizada! Preparando para persistir ${payload.positions.length} nós.`);
                if (onStabilized) onStabilized(payload.positions);
            }
        };
    };

    return { setupReceiver };
}
