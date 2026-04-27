/**
 * 🌉 EventBridge — A Ponte de Interação
 * 
 * Centraliza os tratadores de eventos de Clique, Hover e Drag.
 * Gerencia a comunicação com a API do sistema (Wails) para recuperar 
 * o contexto dos nós neurais.
 */
export function useEventBridge({ store, pilotFocus, activateNetwork, startPhysicsDrag, handlePhysicsDrag, endPhysicsDrag, updateLayers }) {

    const onHover = (info) => {
        if (info.object) {
            store.hoveredNodeId = info.object.id;
            document.body.style.cursor = 'pointer';
        } else {
            store.hoveredNodeId = null;
            document.body.style.cursor = 'default';
        }
    };

    const onClick = (info, deckInstance, currentViewState, activeNodeIdRef) => {
        if (!info.object) {
            activeNodeIdRef.value = null;
            store.resetHighlights();
            updateLayers();
            return;
        }

        const nodeId = info.object.id;
        store.activeNodeId = nodeId;
        activeNodeIdRef.value = nodeId;

        // ✨ ATIVAÇÃO DE REDE NEURAL (Local Integration)
        if (activateNetwork) activateNetwork(nodeId);

        // [Mixer] Sincroniza links destacados (Limpa o rastro anterior)
        store.highlightedLinks.clear();
        store.clickedNodeLinks.clear();

        store.selectedNode = info.object;
        store.nodeDetails = { loading: true, path: '', content: '', isVirtual: false };

        // Integração Dinâmica com o Backend (Wails)
        const bridge = (window.go?.core?.App) || (window.go?.main?.App);

        if (bridge && bridge.GetNeuralNodeContext) {
            bridge.GetNeuralNodeContext(nodeId).then(res => {
                if (res && res.success !== false) {
                    store.nodeDetails = {
                        loading: false,
                        path: res.path || 'Memória Virtual',
                        content: res.content || res.summary || 'Sem metadados',
                        isVirtual: res.document_type === 'memory'
                    };

                    // Enriquecimento: Se o backend devolver arestas específicas, as adicionamos!
                    if (res.related_edges) {
                        res.related_edges.forEach(edgeId => store.clickedNodeLinks.add(edgeId));
                    }
                    updateLayers();
                } else {
                    store.nodeDetails = { loading: false, path: 'Erro', content: 'Metadados não encontrados no Vector Store.' };
                }
            }).catch(err => {
                console.error("[EventBridge] Erro ao buscar contexto do nó:", err);
                store.nodeDetails = { loading: false, path: 'Erro de Conexão', content: 'Falha letal ao contatar o Bridge do backend.' };
            });
        } else {
            // Backend inativo, simula falha
            console.warn("[EventBridge] Bridge do Wails inativo ou Módulo não encontrado.");
            store.nodeDetails = { loading: false, path: 'Offline', content: 'Comunicação RPC indisponível no momento.' };
        }

        // Voo de câmera para o nó
        pilotFocus(deckInstance, currentViewState, info.object);
        updateLayers();
    };

    const onDragStart = (info) => {
        if (info.object) {
            startPhysicsDrag(info.object.id);
            return true;
        }
        return false;
    };

    const onDrag = (info) => {
        if (info.object && info.coordinate) {
            handlePhysicsDrag(info.object.id, info.coordinate[0], info.coordinate[1], info.coordinate[2]);
            return true;
        }
    };

    const onDragEnd = (info) => {
        if (info.object) {
            endPhysicsDrag(info.object.id);
            return true;
        }
    };

    return { onHover, onClick, onDragStart, onDrag, onDragEnd };
}
