/**
 * 📜 StoreContract — O Contrato de Compatibilidade
 * 
 * Responsável por expor as funções do motor gráfico para o resto 
 * da aplicação através da store global, mantendo compatibilidade 
 * com o contrato v9.0 original.
 */
export function useStoreContract({ 
    store, 
    deckInstanceRef, 
    currentViewState, 
    currentNodesRef, 
    currentLinksRef,
    pilotFocus, 
    pilotZoom, 
    pilotPan, 
    updateGraphFn 
}) {
    
    const bind = () => {
        const zoomToFit = () => pilotZoom(deckInstanceRef.value, currentViewState);
        const cameraPosition = (pos, node) => node && pilotFocus(deckInstanceRef.value, currentViewState, node);
        const panTarget = (x, y, z) => pilotPan(deckInstanceRef.value, currentViewState, x, y, z);
        
        const focusNode = (id) => {
            if (!id) return;
            const targetId = String(id).toLowerCase();
            
            // 1. Tenta match exato primeiro
            let node = currentNodesRef.value.find(n => String(n.id).toLowerCase() === targetId);
            
            // 2. Se não achar, tenta match parcial (útil para "sqlite" vs "sqlite.md")
            if (!node) {
                node = currentNodesRef.value.find(n => {
                    const nid = String(n.id).toLowerCase();
                    return nid.includes(targetId) || targetId.includes(nid);
                });
            }

            if (node) {
                console.log("[Contract] ✅ Nó encontrado para zoom:", node.id);
                pilotFocus(deckInstanceRef.value, currentViewState, node);
            } else {
                console.warn("[Contract] ❌ Nó não encontrado no grafo para o ID:", id);
            }
        };

        // Registro do contrato do grafo na Store
        store.graphInstance = {
            zoomToFit,
            cameraPosition,
            panTarget,
            focusNode,
            graphData: (newData) => {
                // Getter: Retorna dados atuais
                if (newData === undefined) {
                    return { 
                        nodes: currentNodesRef.value, 
                        links: currentLinksRef.value 
                    };
                }
                // Setter: Dispara sincronização
                updateGraphFn(newData.nodes, newData.links);
            }
        };
    };

    const unbind = () => {
        store.graphInstance = null;
    };

    return { bind, unbind };
}
