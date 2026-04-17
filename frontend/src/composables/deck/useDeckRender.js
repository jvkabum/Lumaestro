import { shallowRef } from 'vue';
import { useGraphStore } from '../../stores/graph';

// Motores de Especialistas (Engine Drivers)
import { usePhysicsDriver } from './engine/PhysicsDriver.js';
import { useDataEngineer } from './engine/DataEngineer.js';
import { useInteractionPilot } from './engine/InteractionPilot.js';

// Micro-Módulos Atômicos (v13.0 & v14.0)
import { useAnimationClock } from './engine/AnimationClock.js';
import { useEventBridge } from './engine/EventBridge.js';
import { useLayerComposer } from './engine/LayerComposer.js';
import { useStoreContract } from './engine/StoreContract.js';
import { useDeckFactory } from './engine/DeckFactory.js';
import { usePhysicsReceiver } from './engine/PhysicsReceiver.js';

/**
 * 🎼 useDeckRender — O Maestro Atômico Definitivo (v14.0)
 * 
 * Fachada pura e declarativa que orquestra um ecossistema de micro-especialistas.
 * Responsável apenas por conectar os sinais entre os drivers atômicos.
 */
export function useDeckRender() {
    const store = useGraphStore();
    const deckInstance = shallowRef(null);
    let physicsWorker = null;

    // Estado Vitalício (Wrappers para reatividade e compartilhamento)
    const currentNodes = shallowRef([]);
    const currentLinks = shallowRef([]);
    const nodeMap = new Map();
    const activeNodeId = shallowRef(null);
    const currentViewState = shallowRef({ target: [0, 0, 0], zoom: -2.8, rotationX: 30, rotationOrbit: -25 });

    // ── Instalação dos Especialistas (Atomic Drivers) ──
    const { initPhysics, updatePhysicsData, syncPositions, startDrag, handleDrag, endDrag, terminatePhysics } = usePhysicsDriver();
    const { purify, bootstrapCoordinates, syncIncremental, mapLinks } = useDataEngineer();
    const { focusNode: pilotFocus, zoomToFit: pilotZoom, panTarget: pilotPan } = useInteractionPilot();
    const { startClock, stopClock, getTick } = useAnimationClock();
    const { compose } = useLayerComposer();
    const { createDeck } = useDeckFactory();
    
    const { setupReceiver } = usePhysicsReceiver({ 
        nodeMap, currentLinksRef: currentLinks, syncPositions, onUpdate: () => updateLayers() 
    });

    const eventHandlers = useEventBridge({ 
        store, pilotFocus, updateLayers: () => updateLayers(),
        startPhysicsDrag: startDrag, handlePhysicsDrag: handleDrag, endPhysicsDrag: endDrag 
    });

    const { bind, unbind } = useStoreContract({
        store, deckInstanceRef: deckInstance, currentViewState, 
        currentNodesRef: currentNodes, currentLinksRef: currentLinks,
        pilotFocus, pilotZoom, pilotPan, updateGraphFn: (n, e) => updateGraph(n, e)
    });

    /**
     * Inicializa o Ecossistema Gráfico de forma declarativa
     */
    const initGraph = (containerRef, rawNodes, rawEdges, initialActiveNode) => {
        if (!containerRef) return;
        activeNodeId.value = initialActiveNode;

        // 1. Dados e Deck
        currentNodes.value = bootstrapCoordinates(purify(rawNodes));
        currentNodes.value.forEach(n => nodeMap.set(String(n.id), n));
        currentLinks.value = mapLinks(rawEdges, nodeMap);

        deckInstance.value = createDeck({ 
            containerRef, currentViewState, 
            onViewStateChange: ({ viewState }) => { currentViewState.value = viewState; updateLayers(); return viewState; } 
        });

        // 2. Física e Telemetria
        physicsWorker = initPhysics(currentNodes.value, purify(rawEdges));
        setupReceiver(physicsWorker);

        // 3. Ativação dos Contratos e Relógio
        bind();
        startClock(() => updateLayers());
    };

    const updateGraph = (rawNodes, rawEdges) => {
        if (!physicsWorker || !deckInstance.value) return;
        const pureNodes = syncIncremental(rawNodes, nodeMap, currentNodes.value);
        currentLinks.value = mapLinks(rawEdges, nodeMap);
        updatePhysicsData(pureNodes, purify(rawEdges));
    };

    /**
     * Orquestração das Camadas
     */
    const updateLayers = () => {
        if (!deckInstance.value) return;
        const layers = compose({
            currentNodes: currentNodes.value, currentLinks: currentLinks.value,
            currentViewState: currentViewState.value, activeNodeId: activeNodeId.value,
            animationTime: getTick(), store,
            eventHandlers: { ...eventHandlers, onClick: (i) => eventHandlers.onClick(i, deckInstance.value, currentViewState, activeNodeId) }
        });
        deckInstance.value.setProps({ layers });
    };

    const destroyGraph = () => {
        stopClock(); terminatePhysics(); unbind();
        if (deckInstance.value) { deckInstance.value.finalize(); deckInstance.value = null; }
        document.body.style.cursor = 'default';
    };

    return { initGraph, updateGraph, destroyGraph, currentViewState };
}
