import { shallowRef } from 'vue';
import { useGraphStore } from '../../stores/graph';
import { useOrchestratorStore } from '../../stores/orchestrator';

// Motores de Especialistas (Engine Drivers)
import { useDataEngineer } from './engine/DataEngineer.js';
import { useInteractionPilot } from './engine/InteractionPilot.js';
import { usePhysicsDriver } from './engine/PhysicsDriver.js';

// Micro-Módulos Atômicos (v13.0 & v14.0)
import { useAnimationClock } from './engine/AnimationClock.js';
import { useDeckFactory } from './engine/DeckFactory.js';
import { useEventBridge } from './engine/EventBridge.js';
import { useLayerComposer } from './engine/LayerComposer.js';
import { usePhysicsReceiver } from './engine/PhysicsReceiver.js';
import { useStoreContract } from './engine/StoreContract.js';

/**
 * 🎼 useDeckRender — O Maestro Atômico Definitivo (v14.0)
 * 
 * Fachada pura e declarativa que orquestra um ecossistema de micro-especialistas.
 * Responsável apenas por conectar os sinais entre os drivers atômicos.
 */
export function useDeckRender() {
    const store = useGraphStore();
    const orchestrator = useOrchestratorStore();
    const deckInstance = shallowRef(null);
    let physicsWorker = null;

    // Estado Vitalício (Wrappers para reatividade e compartilhamento)
    const currentNodes = shallowRef([]);
    const currentLinks = shallowRef([]);
    const nodeMap = new Map();
    const activeNodeId = shallowRef(null);
    const currentViewState = shallowRef({ target: [0, 0, 0], zoom: -2.8, rotationX: 30, rotationOrbit: -25 });
    let cleanupKeyboard = null; // ← Segura a lixeira dos listeners de teclado

    // ── Instalação dos Especialistas (Atomic Drivers) ──
    const {
        initPhysics, updatePhysicsData, syncPositions,
        startDrag, handleDrag, endDrag,
        updateForce, terminatePhysics // ← updateForce restaurado
    } = usePhysicsDriver();

    const { purify, bootstrapCoordinates, syncIncremental, mapLinks } = useDataEngineer();
    const { 
        focusNode: pilotFocus, 
        focusNodeById: pilotFocusNodeById, // ← Importando a lógica de busca base
        zoomToFit: pilotZoom, 
        panTarget: pilotPan, 
        setupKeyboardNav 
    } = useInteractionPilot();
    const { startClock, stopClock, getTick, getFPS } = useAnimationClock();
    const { compose } = useLayerComposer();
    const { createDeck } = useDeckFactory();

    /**
     * 💾 Persistência de Layout (Ponte Wails - [Mixer Main])
     */
    const savePositions = async (positions) => {
        if (!positions || positions.length === 0) return;
        try {
            // Tenta acessar o backend para salvar as coordenadas
            const bridge = (window.go?.core?.App) || (window.go?.main?.App);
            if (bridge && bridge.UpdateNodePositions) {
                const res = await bridge.UpdateNodePositions(positions);
                console.log(`[Maestro] 🪐 Layout persistido: ${res}`);
            }
        } catch (err) {
            console.warn('[Maestro] ❌ Falha ao persistir layout:', err);
        }
    };

    const { setupReceiver } = usePhysicsReceiver({
        nodeMap,
        currentLinksRef: currentLinks,
        syncPositions,
        onUpdate: () => updateLayers(),
        onStabilized: (positions) => savePositions(positions) // ← Auto-save ativado
    });

    const eventHandlers = useEventBridge({
        store, pilotFocus, updateLayers: () => updateLayers(),
        startPhysicsDrag: startDrag, handlePhysicsDrag: handleDrag, endPhysicsDrag: endDrag
    });



    /**
     * ⚡ activateNetwork — Identifica e ilumina a vizinhança imadiata do nó.
     */
    const activateNetwork = (nodeId) => {
        if (!nodeId) return;
        const s = String(nodeId);

        store.resetHighlights();

        currentLinks.value.forEach(l => {
            const sid = String(l.source?.id || l.source);
            const tid = String(l.target?.id || l.target);

            if (sid === s || tid === s) {
                // Acende o link
                store.clickedNodeLinks.add(`${sid}-${tid}`);
                store.clickedNodeLinks.add(`${tid}-${sid}`); // Bidirecional para segurança visual

                // Acende o vizinho
                const neighborId = sid === s ? tid : sid;
                store.highlightedNeighbors.add(neighborId);
            }
        });

        console.log(`[useDeckRender] ⚡ Rede ativada para ${nodeId}: ${store.highlightedNeighbors.size} vizinhos iluminados.`);
    };

    /**
     * 🎯 focusNodeById — Resolve um ID ÚNICO e voa a câmera até ele.
     * Prioriza ID Absoluto (Caminho) para evitar colisões.
     */
    const focusNodeById = (nodeId) => {
        if (!nodeId || !deckInstance.value) return null;

        console.log(`[Maestro] 🔎 Buscando Neurônio: "${nodeId}"`);

        const node = pilotFocusNodeById(nodeId, { 
            nodeMap, 
            deckInstance: deckInstance.value, 
            currentViewState 
        });

        if (node) {
            console.log(`[Maestro] 🚀 Alinhando propulsão cinematográfica para: ${node.name}`);
            orchestrator.pushStatus(`✨ Sinapse Visual estabelecida: ${node.name}`, 'memory');
            activeNodeId.value = node.id;

            // ✨ ATIVAÇÃO DE REDE NEURAL (Neon Effect)
            activateNetwork(node.id);
            updateLayers();
            return node;
        } else {
            console.error(`[Maestro] ❌ Neurônio "${nodeId}" não localizado no Grafo 3D.`);
            orchestrator.pushStatus(`❌ Falha: Neurônio não localizado.`, 'error');
        }
        return null;
    };

    // ── Bind de Contratos (Sincronização com Pinia/RAG) ──
    const { bind, unbind } = useStoreContract({
        store, deckInstanceRef: deckInstance, currentViewState,
        currentNodesRef: currentNodes, currentLinksRef: currentLinks,
        nodeMap,
        pilotFocus, pilotZoom, pilotPan,
        focusNodeById, // ⚡ Versão robusta agora definida antes do uso
        updateGraphFn: (n, e) => updateGraph(n, e)
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
            containerRef,
            currentViewState: currentViewState.value,
            onViewStateChange: ({ viewState }) => {
                currentViewState.value = viewState;
                updateLayers();
                return viewState;
            }
        });

        // 2. Física e Telemetria
        physicsWorker = initPhysics(currentNodes.value, purify(rawEdges));
        setupReceiver(physicsWorker);

        // 3. Ativação dos Contratos e Relógio
        bind();

        // ⌨️ Configuração de Teclado (WASD/QE + Presets v18.13 - via InteractionPilot)
        cleanupKeyboard = setupKeyboardNav(deckInstance.value, currentViewState, containerRef, store, 25);

        startClock(() => {
            store.currentFps = getFPS();
            updateLayers();
        });

        // ⌨️ Escuta Global para Debug (F1)
        const handleDebugKey = (e) => {
            if (e.key === 'F1') {
                e.preventDefault();
                store.showFps = !store.showFps;
            }
        };
        window.addEventListener('keydown', handleDebugKey);
        const originalCleanup = cleanupKeyboard;
        cleanupKeyboard = () => {
            if (originalCleanup) originalCleanup();
            window.removeEventListener('keydown', handleDebugKey);
        };

        // 🚀 [RENDER INICIAL] Força um frame imediato para evitar tela preta 
        // caso o Worker demore a disparar o primeiro Tick
        updateLayers();
    };

    const updateGraph = (rawNodes, rawEdges) => {
        if (!physicsWorker || !deckInstance.value) return;
        const pureNodes = syncIncremental(rawNodes, nodeMap, currentNodes);
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

        // 🚀 Sincronização de Estado (View + Layers)
        // Passar viewState aqui evita o "lock" do zoom manual quando o sistema entra em modo controlado.
        deckInstance.value.setProps({
            layers,
            viewState: currentViewState.value
        });
    };

    const destroyGraph = () => {
        if (cleanupKeyboard) cleanupKeyboard(); // Limpa listeners de teclado
        stopClock(); terminatePhysics(); unbind();

        if (deckInstance.value) { deckInstance.value.finalize(); deckInstance.value = null; }
        document.body.style.cursor = 'default';
    };

    return {
        initGraph, updateGraph, destroyGraph, updateForce,
        currentViewState, savePositions, currentNodes,
        focusNodeById, nodeMap
    };
}
