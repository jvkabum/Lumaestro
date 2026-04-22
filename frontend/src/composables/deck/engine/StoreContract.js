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
            
            let node = currentNodesRef.value.find(n => String(n.id).toLowerCase() === targetId);
            
            if (!node) {
                node = currentNodesRef.value.find(n => {
                    const nid = String(n.id).toLowerCase();
                    return nid.includes(targetId) || targetId.includes(nid);
                });
            }

            if (node) {
                console.log("[Contract] ✅ Nó encontrado para zoom + detalhes:", node.id);
                pilotFocus(deckInstanceRef.value, currentViewState, node);
                
                // 🧠 AUTO-DETAIL: Abre a descrição do nó automaticamente
                store.selectedNode = node;
                store.nodeDetails = { loading: true, path: '', content: '', isVirtual: false };

                const bridge = (window.go?.core?.App) || (window.go?.main?.App);
                if (bridge && bridge.GetNeuralNodeContext) {
                    bridge.GetNeuralNodeContext(node.id).then(res => {
                        if (res && res.success !== false) {
                            store.nodeDetails = {
                                loading: false,
                                path: res.path || 'Memória Virtual',
                                content: res.content || res.summary || 'Sem metadados',
                                isVirtual: res.document_type === 'memory'
                            };
                        } else {
                            store.nodeDetails = { loading: false, path: 'Informativo', content: 'Nota identificada, mas conteúdo ainda em processamento.' };
                        }
                    }).catch(err => {
                        console.error("[Contract] Erro ao buscar contexto automático:", err);
                    });
                }
            } else {
                console.warn("[Contract] ❌ Nó não encontrado para ID:", id);
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
