/**
 * 🌉 EventBridge — A Ponte de Interação
 * 
 * Centraliza os tratadores de eventos de Clique, Hover e Drag.
 * Gerencia a comunicação com a API do sistema (Wails) para recuperar 
 * o contexto dos nós neurais.
 */
export function useEventBridge({ store, pilotFocus, startPhysicsDrag, handlePhysicsDrag, endPhysicsDrag, updateLayers }) {

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
        if (!info.object) return;

        store.activeNodeId = info.object.id;
        activeNodeIdRef.value = info.object.id;
        store.selectedNode = info.object;
        store.nodeDetails = { loading: true, path: '', content: '', isVirtual: false };

        // Integração com o Backend (Wails)
        if (window.go?.main?.App?.GetNeuralNodeContext) {
            window.go.main.App.GetNeuralNodeContext(info.object.id).then(res => {
                if (res && res.success !== false) {
                    store.nodeDetails = {
                        loading: false,
                        path: res.path || 'Memória Virtual',
                        content: res.content || res.summary || 'Sem metadados',
                        isVirtual: res.document_type === 'memory'
                    };
                    
                    // Sincroniza links destacados
                    store.highlightedLinks.clear();
                    store.clickedNodeLinks.clear();
                    if (res.related_edges) {
                        res.related_edges.forEach(edgeId => store.clickedNodeLinks.add(edgeId));
                    }
                    updateLayers();
                }
            });
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
