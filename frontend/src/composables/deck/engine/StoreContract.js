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
    focusNodeById, // ⚡ Recebendo a ferramenta de busca robusta
    updateGraphFn 
}) {
    
    let unsubscribeActive;
    
    const bind = () => {
        const zoomToFit = () => pilotZoom(deckInstanceRef.value, currentViewState);
        const cameraPosition = (pos, node) => node && pilotFocus(deckInstanceRef.value, currentViewState, node);
        const panTarget = (x, y, z) => pilotPan(deckInstanceRef.value, currentViewState, x, y, z);
 
        // 🚀 [Mixer v2] Ponte de Descoberta: Encapsula a busca robusta com feedback visual
        const focusNode = (id) => {
            if (!id) {
                zoomToFit();
                return;
            }

            const node = focusNodeById(id);
            if (node) {
                console.log("[Contract] ✅ Nó identificado via Contrato:", node.id);
                store.setSelectedNode(node);
                store.discoveryStatus = 'found';

                const bridge = (window.go?.core?.App) || (window.go?.main?.App);
                if (bridge && bridge.GetNeuralNodeContext) {
                    bridge.GetNeuralNodeContext(node.id).then(res => {
                        if (res && res.success !== false) {
                            store.setNodeDetails({
                                loading: false,
                                path: res.path || 'Memória Virtual',
                                content: res.content || res.summary || 'Sem metadados',
                                isVirtual: res.type === 'memory'
                            });
                        }
                    }).catch(err => {
                        console.error("[Contract] Erro ao buscar contexto:", err);
                    });
                }
            } else {
                console.warn("[Contract] ⚠️ Nó não localizado diretamente para ID:", id);
            }
        };
 
        // Registro do contrato do grafo na Store
        store.graphInstance = {
            zoomToFit,
            cameraPosition,
            panTarget,
            focusNode, // Agora usa a versão robusta com feedback
            focusNodeById, // ⚡ Exposto para BridgeDriver, GraphProvenance e Chat
            // Busca exposta sem side-effects
            search: (id) => focusNodeById(id),
            graphData: (newData) => {
                if (newData === undefined) {
                    return { nodes: currentNodesRef.value, links: currentLinksRef.value };
                }
                updateGraphFn(newData.nodes, newData.links);
            }
        };
 
        // 📡 Escuta o chat para zoom automático (Discovery Effect)
        if (window.runtime && window.runtime.EventsOn) {
            unsubscribeActive = window.runtime.EventsOn("node:active", (id) => {
                console.log("[Contract] 📡 Sinal de Ativação Neural recebido:", id);
                focusNode(id);
            });
        }
    };
 
    const unbind = () => {
        if (unsubscribeActive) {
            unsubscribeActive();
            unsubscribeActive = null;
        }
        store.graphInstance = null;
    };

    return { bind, unbind };
}
