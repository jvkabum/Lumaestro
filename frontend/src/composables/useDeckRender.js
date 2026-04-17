import { Deck, FlyToInterpolator, OrbitView } from '@deck.gl/core';
import { ArcLayer, ScatterplotLayer, TextLayer } from '@deck.gl/layers';
import { shallowRef } from 'vue';
import { useGraphStore } from '../stores/graph';

import PhysicsWorker from '../workers/physicsWorker?worker';

export function useDeckRender() {
    let deckInstance = null;
    let physicsWorker = null;
    const store = useGraphStore();

    // Cache interno
    let currentNodes = [];
    let currentLinks = [];
    const nodeMap = new Map(); // Registro Vitalício de Referências
    let activeNodeId = null;

    // Estado da Câmera REATIVO (Faz o grafo sentir o zoom em tempo real)
    const currentViewState = shallowRef({
        target: [0, 0, 0],
        zoom: -2.8,            // Zoom recuado para ver o novo Multiverso
        rotationX: 30,
        rotationOrbit: -25
    });

    // Cores (RGB formato WebGL) — Saturadas e Vívidas
    const colors = {
        source: [244, 114, 182],   // Rosa Saturado (como na referência)
        page: [34, 211, 238],      // Cyan Vibrante
        chunk: [100, 160, 255],    // Azul Celeste
        system: [255, 255, 255],   // Branco Puro (gigantes na referência)
        memory: [244, 114, 182],   // Rosa Quente
        active: [252, 211, 77]     // Dourado
    };

    // Paleta de Clusters (Comunidades Semânticas) — Cores Distintas e Vibrantes
    const communityColors = [
        [34, 211, 238],   // Cyan
        [167, 139, 250],  // Violeta
        [251, 146, 60],   // Laranja
        [74, 222, 128],   // Esmeralda
        [244, 114, 182],  // Rosa
        [250, 204, 21],   // Amarelo
        [56, 189, 248],   // Sky Blue
        [232, 121, 249]   // Fúcsia
    ];

    const getCommunityColor = (cid) => {
        if (cid === undefined || cid === null || cid < 0) return null;
        return communityColors[cid % communityColors.length];
    };

    const getRadius = (node) => {
        const deg = node.degree || 0;
        const pr = (node.pagerank && node.pagerank > 0) ? node.pagerank : 0;
        const importance = Math.max(deg, pr * 20);
        const type = node['document-type'] || 'chunk';

        // Escala Híbrida Exponencial (Pixels + Zoom Compensado)
        // Resolve o bug de encolhimento consumindo o estado reativo
        const zoom = currentViewState.value.zoom;
        const exponentialScale = Math.pow(2, zoom + 2.8);

        const baseSize = type === 'source' ? 1.8 : (type === 'system' ? 2.2 : (type === 'page' ? 1.4 : 1.0));
        const r = (baseSize + Math.pow(importance, 0.45) * 0.25) * exponentialScale;

        return Math.max(r, 1.5);
    };

    const initGraph = (containerRef, rawNodes, rawEdges, initialActiveNode) => {
        if (!containerRef) return;
        activeNodeId = initialActiveNode;

        // Purifica os dados retirando toda a casca reativa (Proxy) do Vue 3.
        // O Deck.gl e o WebWorker precisam de arrays JSON Literais para rodar pesadamente sem penalidade ou DataCloneError.
        const pureNodes = JSON.parse(JSON.stringify(rawNodes));
        const pureEdges = JSON.parse(JSON.stringify(rawEdges));

        // Espalhamento Térmico Inicial com objetos limpos
        currentNodes = pureNodes.map(n => {
            if (n.x === undefined) {
                // Cálculo de Nascimento Esférico Orgânico (Distribuição Polar)
                const r = Math.pow(Math.random(), 1 / 3) * 1200; // Raio volumétrico
                const theta = Math.acos(2 * Math.random() - 1);
                const phi = 2 * Math.PI * Math.random();

                return {
                    ...n,
                    x: r * Math.sin(theta) * Math.cos(phi),
                    y: r * Math.sin(theta) * Math.sin(phi),
                    z: r * Math.cos(theta)
                };
            }
            return n;
        });

        // Ponteiro O(1) de acesso direto (Limpo) - FORCE STRING ID
        currentNodes.forEach(n => nodeMap.set(String(n.id), n));

        currentLinks = pureEdges.map(link => {
            const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
            const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
            return {
                ...link,
                sourceObj: nodeMap.get(sid),
                targetObj: nodeMap.get(tid)
            };
        });

        // 1. O Cérebro Matemático (Isolado no WebWorker via protocolo Vite)
        physicsWorker = new PhysicsWorker();

        // 2. Motor Gráfico (Deck)
        deckInstance = new Deck({
            parent: containerRef,
            initialViewState: currentViewState,
            getTooltip: ({ object }) => object && object.name ? {
                text: `${object.name}\nTipo: ${object['document-type'] || 'Conceito'}`,
                style: {
                    backgroundColor: 'rgba(15, 23, 42, 0.95)',
                    color: '#fff',
                    borderRadius: '6px',
                    border: '1px solid rgba(255, 255, 255, 0.1)',
                    fontFamily: 'Inter, sans-serif',
                    fontSize: '13px',
                    padding: '8px 12px',
                    zIndex: 9999
                }
            } : null,
            onViewStateChange: ({ viewState }) => {
                currentViewState.value = viewState; // Dispara a reatividade do Vue
                updateLayers();                     // Força o Deck.gl a recalcular o LOD das labels imediatamente
                return viewState;
            },
            views: new OrbitView({
                orbitAxis: 'Y',
                near: 0.1,    // Permite chegar "na cara" do nó sem cortar (clipping)
                far: 50000    // Visão profunda para o multiverso
            }),
            controller: {
                dragRotate: true,
                dragPan: true,
                doubleClickZoom: false
            },
            layers: []
        });

        // 3. Loop Síncrono da Simulação de Física -> Renderização
        let tickCounter = 0;
        physicsWorker.onmessage = (event) => {
            const { type, payload } = event.data;
            if (type === 'TICK') {
                tickCounter++;
                const { positions } = payload;
                if (!positions) return;

                // Sync O(1) rápido via Map persistente
                for (let i = 0; i < positions.length; i++) {
                    const p = positions[i];
                    const node = nodeMap.get(String(p.id));
                    if (node) {
                        node.x = p.x;
                        node.y = p.y;
                        node.z = p.z;
                    }
                }

                // 🩹 AUTO-CURA: Re-ancora links que perderam o objeto (Resolve o bug de 0,0,0)
                currentLinks.forEach(link => {
                    if (!link.sourceObj || !link.targetObj) {
                        const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
                        const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
                        if (!link.sourceObj) link.sourceObj = nodeMap.get(sid);
                        if (!link.targetObj) link.targetObj = nodeMap.get(tid);
                    }
                });

                updateLayers(tickCounter);
            }
        };

        physicsWorker.postMessage({
            type: 'INIT',
            payload: { nodes: pureNodes, links: pureEdges }
        });

        // 4. Ignição do Motor de Fótons (Partículas Óticas 60FPS)
        startPhotonLoop();
    };

    /**
     * 🔥 updateGraph: Sincronização Incremental (Zero Jitter)
     * Mantém a simulação viva no Worker e apenas injeta novos dados.
     */
    const updateGraph = (rawNodes, rawEdges) => {
        if (!physicsWorker || !deckInstance) return;

        const pureNodes = JSON.parse(JSON.stringify(rawNodes));
        const pureEdges = JSON.parse(JSON.stringify(rawEdges));

        // 1. Atualiza o registro de nós (Garante persistência de referência para o Deck.gl)
        pureNodes.forEach(n => {
            const sid = String(n.id);
            if (!nodeMap.has(sid)) {
                // Nascimento Esférico se for um nó novo
                const r = 200 + Math.random() * 500;
                const theta = Math.random() * 2 * Math.PI;
                const phi = Math.acos(2 * Math.random() - 1);
                
                const newNode = {
                    ...n,
                    x: r * Math.sin(phi) * Math.cos(theta),
                    y: r * Math.sin(phi) * Math.sin(theta),
                    z: r * Math.cos(phi)
                };
                nodeMap.set(sid, newNode);
                currentNodes.push(newNode);
            } else {
                // Atualiza metadados PRESERVANDO x, y, z (Blindagem de Física)
                const existing = nodeMap.get(sid);
                // Usamos spread para garantir que novos metadados entrem, mas posições fiquem
                Object.assign(existing, { 
                    ...n, 
                    x: existing.x, 
                    y: existing.y, 
                    z: existing.z 
                });
            }
        });

        // 2. Reconecta os links usando os objetos ATIVOS do nodeMap (Resolve o bug de 0,0,0)
        currentLinks = pureEdges.map(link => {
            const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
            const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
            
            return {
                ...link,
                sourceObj: nodeMap.get(sid),
                targetObj: nodeMap.get(tid)
            };
        });

        physicsWorker.postMessage({
            type: 'UPDATE_DATA',
            payload: { nodes: pureNodes, links: pureEdges }
        });
    };

    let animationRafId = null;
    let animationTime = 0;

    const startPhotonLoop = () => {
        const march = () => {
            // Incrementa o tempo universal da simulação. 
            // 0.007 = Fóton atravessa o link inteiro em cerca de 2 segundos.
            animationTime += 0.007;
            if (animationTime >= 100000) animationTime = 0; // Evita limite de float

            updateLayers(null); // Passa Null para repintar fótons sem estressar física
            animationRafId = requestAnimationFrame(march);
        };
        if (animationRafId) cancelAnimationFrame(animationRafId);
        animationRafId = requestAnimationFrame(march);
    };

    const updateLayers = (tickCounter = null) => {
        if (!deckInstance) return;

        const hlLinks = store.highlightedLinks;
        const clLinks = store.clickedNodeLinks;
        const zoom = currentViewState.value.zoom;

        // Hub detection: nós com >25 links viram gravidade invisível (sem linhas)
        const degreeCounts = new Map();
        currentLinks.forEach(l => {
            const sid = l.source?.id || l.source;
            const tid = l.target?.id || l.target;
            degreeCounts.set(sid, (degreeCounts.get(sid) || 0) + 1);
            degreeCounts.set(tid, (degreeCounts.get(tid) || 0) + 1);
        });
        const HUB_THRESHOLD = 15; // Hubs com >15 conexões viram gravidade invisível

        const layers = [
            // 🏷️ CAMADA DE TEXTO (Labels Inteligentes LOD + Hubs)
            new TextLayer({
                id: 'graph-labels',
                data: currentNodes,
                getPosition: node => [node.x || 0, node.y || 0, node.z || 0],
                getText: node => {
                    const deg = degreeCounts.get(node.id) || node.degree || 0;
                    const isElite = (deg > 15) || (node['document-type'] === 'source') || (node['document-type'] === 'system');
                    const isImportant = (deg > 5);
                    const isMemory = node['document-type'] === 'memory';
                    const isHovered = store.hoveredNodeId === node.id;
                    const isSelected = store.selectedNodeId === node.id;

                    if ((isElite || isHovered || isSelected) && zoom > -3.0) {
                        const name = node.name || 'Nó';
                        return name.length > 20 ? name.substring(0, 18) + '..' : name;
                    }

                    if ((isMemory || isImportant) && zoom > -1.0) {
                        const name = node.name || 'Dado';
                        return name.length > 18 ? name.substring(0, 16) + '..' : name;
                    }

                    if (zoom > 1.2) {
                        const name = node.name || '..';
                        return name.length > 16 ? name.substring(0, 14) + '..' : name;
                    }

                    return '';
                },
                getSize: node => {
                    const deg = degreeCounts.get(node.id) || node.degree || 0;
                    if (deg > 15) return 15;
                    if (node['document-type'] === 'memory') return 11;
                    return 10;
                },
                getColor: node => {
                    const deg = degreeCounts.get(node.id) || node.degree || 0;
                    const isElite = (deg > 15) || (node['document-type'] === 'source') || (node['document-type'] === 'system');
                    const isImportant = (deg > 5);
                    const isMemory = node['document-type'] === 'memory';

                    // Cálculo de Opacidade Progressiva (Fade-in baseado no Zoom)
                    let alpha = 0;
                    if (isElite || store.hoveredNodeId === node.id || store.selectedNodeId === node.id) {
                        alpha = Math.max(0, Math.min(255, (zoom + 3.2) * 200));
                    } else if (isMemory || isImportant) {
                        alpha = Math.max(0, Math.min(255, (zoom + 1.2) * 200));
                    } else {
                        alpha = Math.max(0, Math.min(255, (zoom - 1.0) * 200));
                    }

                    if (isMemory) return [244, 114, 182, alpha];
                    if (isElite) return [255, 255, 255, alpha];
                    return [255, 255, 255, alpha * 0.8];
                },
                getAngle: 0,
                getTextAnchor: 'start',
                getAlignmentBaseline: 'center',
                getPixelOffset: [12, 0],
                fontFamily: 'Inter, sans-serif',
                fontWeight: 600,
                outlineWidth: 1,
                outlineColor: [15, 23, 42, 180],
                updateTriggers: {
                    getPosition: tickCounter,
                    getText: [zoom, store.hoveredNodeId, store.selectedNodeId],
                    getColor: [zoom, store.hoveredNodeId, store.selectedNodeId],
                    getSize: [zoom, store.hoveredNodeId, store.selectedNodeId]
                }
            }),

            new ArcLayer({
                id: 'graph-edges',
                data: [...currentLinks].filter(l => {
                    if (l['edge-type'] === 'orbital') return false;
                    const sObj = l.sourceObj;
                    const tObj = l.targetObj;
                    if (sObj && (sObj['celestial-type'] === 'galaxy-core' || sObj['celestial-type'] === 'solar-system-core')) return false;
                    if (tObj && (tObj['celestial-type'] === 'galaxy-core' || tObj['celestial-type'] === 'solar-system-core')) return false;
                    return true; // Sem filtros de HUB: Restauração Main
                }),
                getSourcePosition: link => link.sourceObj ? [link.sourceObj.x || 0, link.sourceObj.y || 0, link.sourceObj.z || 0] : [0, 0, 0],
                getTargetPosition: link => link.targetObj ? [link.targetObj.x || 0, link.targetObj.y || 0, link.targetObj.z || 0] : [0, 0, 0],
                getSourceColor: link => {
                    const s = link.source.id || link.source;
                    const t = link.target.id || link.target;
                    if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return [255, 255, 255, 220];
                    if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return [252, 211, 77, 200];
                    return [40, 180, 180, 60]; // Cyan uniforme sutil
                },
                getTargetColor: link => {
                    const s = link.source.id || link.source;
                    const t = link.target.id || link.target;
                    if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return [255, 255, 255, 220];
                    if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return [252, 211, 77, 200];
                    return [40, 180, 180, 60]; // Mesma cor = sem gradiente
                },
                getWidth: link => {
                    const s = link.source.id || link.source;
                    const t = link.target.id || link.target;
                    if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return 2.5;
                    if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return 1.8;
                    return 0.5;
                },
                getHeight: 0.3,  // Curvatura sutil dos arcos
                greatCircle: false,
                updateTriggers: {
                    getSourceColor: [store.clickedNodeLinks.size, store.highlightedLinks.size],
                    getTargetColor: [store.clickedNodeLinks.size, store.highlightedLinks.size],
                    getSourcePosition: tickCounter,
                    getTargetPosition: tickCounter
                }
            }),

            new ScatterplotLayer({
                id: 'graph-nodes',
                data: [...currentNodes], // Clonagem superficial para forçar renderização do Deck.gl
                getPosition: node => [node.x || 0, node.y || 0, node.z || 0],
                getFillColor: node => {
                    if (node.id === activeNodeId) return colors.active;
                    if (store.hoveredNodeId === node.id) return [...colors.active];

                    // Cor da Comunidade Louvain (cluster semântico)
                    const cCol = getCommunityColor(node.community);
                    if (cCol) return [...cCol, 230];

                    // Fallback por tipo de documento
                    const type = node['document-type'] || 'chunk';
                    return colors[type] ? [...colors[type], 220] : [155, 155, 155, 220];
                },
                getRadius: node => {
                    // 📏 ESCALONAMENTO POR IMPORTÂNCIA ESTRUTURAL (v9.0)
                    const deg = degreeCounts.get(node.id) || node.degree || 0;
                    const pr = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : deg;
                    
                    const zoom = currentViewState.value.zoom;
                    // Calibração do zoomBoost para suportar nós maiores sem cobrir a tela
                    const zoomBoost = Math.max(0.40, Math.pow(2, zoom + 1.5)); 
                    
                    // Fórmula Agressiva: Foca no número de conexões reais (deg)
                    const baseScale = 5 + Math.pow(deg, 0.7) * 2.5; 
                    const finalSize = (node.id === activeNodeId) ? baseScale * 1.5 : baseScale;
                    
                    return Math.max(finalSize * zoomBoost, 3.5); 
                },
                radiusUnits: 'pixels',
                radiusMinPixels: 3,
                radiusMaxPixels: 1000,
                pickable: true,
                opacity: 1,
                billboard: true,
                antialiasing: true,
                stroked: false,
                getLineColor: [255, 255, 255, 80],
                lineWidthMinPixels: 1,

                updateTriggers: {
                    getFillColor: [activeNodeId, store.hoveredNodeId],
                    getPosition: tickCounter,
                    getRadius: [currentViewState.value.zoom] // Gatilho de recalibragem instantânea
                },

                // 🖱️ Eventos Brutos (Hover / Clicks)
                onHover: (info) => {
                    if (info.object) {
                        store.hoveredNodeId = info.object.id;
                        document.body.style.cursor = 'pointer';
                    } else {
                        store.hoveredNodeId = null;
                        document.body.style.cursor = 'default';
                    }
                },
                onClick: (info) => {
                    if (info.object) {
                        store.activeNodeId = info.object.id;
                        activeNodeId = info.object.id;
                        store.selectedNode = info.object; // Ativa a aba lateral de Proveniência na UI

                        // Inicializa skeleton loading
                        store.nodeDetails = { loading: true, path: '', content: '', isVirtual: false };

                        // Busca o interior semântico do Nó via Go Backend
                        if (window.go && window.go.main && window.go.main.App && window.go.main.App.GetNeuralNodeContext) {
                            window.go.main.App.GetNeuralNodeContext(info.object.id)
                                .then(res => {
                                    if (res && res.success !== false) {
                                        store.nodeDetails = {
                                            loading: false,
                                            path: res.path || 'Virtual Memory',
                                            content: res.content || res.summary || 'Sem metadados',
                                            isVirtual: res.document_type === 'memory'
                                        };

                                        // Aciona os laços dourados para o Trail
                                        store.highlightedLinks.clear();
                                        store.clickedNodeLinks.clear();
                                        if (res.related_edges) {
                                            res.related_edges.forEach(edgeId => store.clickedNodeLinks.add(edgeId));
                                        }
                                        updateLayers();
                                    } else {
                                        store.nodeDetails = { loading: false, path: 'Erro', content: 'Ficheiro Fantasma ou inacessível.' };
                                    }
                                });
                        }

                        focusNode(info.object);
                        updateLayers();
                    }
                },

                // 🎯 Drag Sincronizado do VRAM de volta pro Worker de Física na CPU
                onDragStart: (info, event) => {
                    if (info.object) {
                        physicsWorker.postMessage({ type: 'DRAG_START', payload: { nodeId: info.object.id } });
                        return true; // Rouba o bloqueio do teclado rotacional do Deck
                    }
                    return false;
                },
                onDrag: (info, event) => {
                    if (info.object && info.coordinate) {
                        // info.coordinate já contém a projeção exata em 3D vinda do Raycast do WebGL!
                        physicsWorker.postMessage({
                            type: 'DRAG',
                            payload: { nodeId: info.object.id, x: info.coordinate[0], y: info.coordinate[1], z: info.coordinate[2] }
                        });
                        return true;
                    }
                },
                onDragEnd: (info, event) => {
                    if (info.object) {
                        physicsWorker.postMessage({ type: 'DRAG_END', payload: { nodeId: info.object.id } });
                        return true;
                    }
                }
            }),

            // 💫 CAMADA DE FÓTONS (TRÁFEGO DE DADOS)
            new ScatterplotLayer({
                id: 'graph-photons',
                data: [...currentLinks],
                getPosition: (link, { index }) => {
                    const s = link.sourceObj;
                    const t = link.targetObj;
                    if (!s || !t) return [0, 0, 0];
                    // O index atua como 'semente' para os fótons não andarem todos paralelos.
                    const phase = (animationTime + (index * 0.137)) % 1.0;
                    return [
                        s.x + (t.x - s.x) * phase,
                        s.y + (t.y - s.y) * phase,
                        s.z + (t.z - s.z) * phase
                    ];
                },
                getFillColor: link => {
                    const s = link.source.id || link.source;
                    const t = link.target.id || link.target;
                    // Fótons ficam Dourados Intensos se a rota for clicada/focada.
                    if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`) || hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) {
                        return [252, 211, 77, 255];
                    }
                    // Efeito de "Pulsar" baseado no tempo para os fótons normais
                    const pulse = (Math.sin(animationTime * 10 + (link.source.id?.length || 0)) + 1) / 2;
                    return [34, 211, 238, 40 + (pulse * 60)]; // Cyan Vibrante pulsante
                },
                getRadius: link => {
                    const isActive = store.highlightedLinks.size > 0;
                    return isActive ? 1.8 : 1.2;
                },
                radiusUnits: 'pixels',
                billboard: true,
                updateTriggers: {
                    getPosition: animationTime,
                    getFillColor: [store.clickedNodeLinks.size, store.highlightedLinks.size]
                }
            })
        ];

        deckInstance.setProps({ layers });
    };

    // Voar (Câmera Interpolada Cinematográfica Estilo Universo)
    const focusNode = (node) => {
        if (!deckInstance || !node) return;

        const targetViewState = {
            ...currentViewState.value,
            target: [node.x, node.y, node.z],
            distance: 300,
            transitionDuration: 2500, // Voo cinematográfico mais lento
            transitionInterpolator: new FlyToInterpolator({ speed: 1.2 })
        };

        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    const zoomToFit = () => {
        if (!deckInstance) return;
        const targetViewState = {
            ...currentViewState.value,
            target: [0, 0, 0],   // Retorna ao centro do Universo
            zoom: -3.2,          // Visão Macro (clusters completos e distantes)
            rotationX: 30,
            rotationOrbit: -25,
            transitionDuration: 1500,
            transitionInterpolator: new FlyToInterpolator()
        };
        deckInstance.setProps({ initialViewState: targetViewState });
        currentViewState.value = targetViewState;
    };

    // Restaurar a ponte de controle para o botão "RECENTRAR" do Vue
    // Mockamos a API que o 3d-force-graph tinha para não quebrar a UI
    store.graphInstance = {
        zoomToFit: zoomToFit,
        graphData: (newData) => {
            if (newData === undefined) {
                // Getter: retorna estado atual do motor
                return { nodes: currentNodes, links: currentLinks };
            }
            // Setter: reconstrói referências e sincroniza com o Worker de Física
            currentNodes = newData.nodes;
            const nodeMapInternal = new Map();
            currentNodes.forEach(n => nodeMapInternal.set(String(n.id), n));
            currentLinks = newData.links.map(link => {
                const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
                const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
                return {
                    ...link,
                    sourceObj: nodeMapInternal.get(sid),
                    targetObj: nodeMapInternal.get(tid)
                };
            }).filter(link => link.sourceObj && link.targetObj);
            if (physicsWorker) {
                physicsWorker.postMessage({
                    type: 'UPDATE_DATA',
                    payload: {
                        nodes: JSON.parse(JSON.stringify(currentNodes)),
                        links: JSON.parse(JSON.stringify(newData.links))
                    }
                });
            }
        },
        cameraPosition: (pos, node) => {
            if (node) focusNode(node);
        },
        panTarget: (moveRawX, moveRawY, moveRawZ) => {
            if (!deckInstance) return;
            // Translata o movimento baseado pro Yaw atual (Camera Angle)
            const yaw = currentViewState.value.rotationOrbit * (Math.PI / 180);

            const dx = moveRawX * Math.cos(yaw) + moveRawZ * Math.sin(yaw);
            const dz = -moveRawX * Math.sin(yaw) + moveRawZ * Math.cos(yaw);

            // Sensibilidade do WASD baseada no zoom
            const speedFactor = Math.max(0.5, 1 / (currentViewState.value.zoom || 0.1)) * 0.1;

            const tX = currentViewState.value.target[0] + dx * speedFactor;
            const tY = currentViewState.value.target[1] + moveRawY * speedFactor;
            const tZ = currentViewState.value.target[2] + dz * speedFactor;

            const targetViewState = {
                ...currentViewState.value,
                target: [tX, tY, tZ],
                transitionDuration: 0 // Sem animação para FPS ser cravado a frame
            };
            deckInstance.setProps({ initialViewState: targetViewState });
            currentViewState.value = targetViewState;
        }
    };

    const destroyGraph = () => {
        if (physicsWorker) {
            physicsWorker.terminate();
            physicsWorker = null;
        }
        if (deckInstance) {
            deckInstance.finalize();
            deckInstance = null;
        }
        store.graphInstance = null;
        document.body.style.cursor = 'default';
    };

    return {
        initGraph,
        updateGraph,
        destroyGraph,
        focusNode,
        currentViewState
    };
}
