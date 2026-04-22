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
            const node = currentNodesRef.value.find(n => String(n.id) === String(id));
            if (node) pilotFocus(deckInstanceRef.value, currentViewState, node);
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
