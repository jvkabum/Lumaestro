
import { ScatterplotLayer } from '@deck.gl/layers';
import { COORDINATE_SYSTEM } from '@deck.gl/core';
import { colors, getCommunityColor } from '../Constants';

/**
 * 🟣 NodeLayer — As Esferas de Conhecimento
 * 
 * Responsável por renderizar os nós (documentos, memórias, sistemas).
 * Inclui os algoritmos de escalonamento v9.0 e os eventos de interação (Hover, Click, Drag).
 */
export function createNodeLayer({
    currentNodes,
    degreeCounts,
    zoom,
    activeNodeId,
    store,
    tickCounter,
    onHover,
    onClick,
    onDragStart,
    onDrag,
    onDragEnd
}) {
    return new ScatterplotLayer({
        id: 'graph-nodes',
        coordinateSystem: COORDINATE_SYSTEM.CARTESIAN,
        data: [...currentNodes], // Clone para garantir atualização no Deck.gl
        getPosition: node => [node.x || 0, node.y || 0, node.z || 0],
        getFillColor: node => {
            if (node.id === activeNodeId) return colors.active;
            if (store.hoveredNodeId === node.id) return [...colors.active];

            // Cor da Comunidade (Cluster Semântico)
            const cCol = getCommunityColor(node.community);
            if (cCol) return [...cCol, 230];

            // Fallback por tipo de documento
            const type = node['document-type'] || 'chunk';
            return colors[type] ? [...colors[type], 220] : [155, 155, 155, 220];
        },
        getRadius: node => {
            // 📏 HIERARQUIA VISUAL (Volumétrica de Volume = Raio^3)
            const deg = degreeCounts.get(String(node.id)) || node.degree || 0;
            const pr = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : deg;

            const isActive = node.id === activeNodeId;
            const importance = Math.max(deg, pr);
            
            // Scaled Down para parâmetros de densidade tradicionais (Snippet Referência)
            const baseScale = 1.0 + Math.pow(importance, 0.5) * 0.4;
            const finalScale = isActive ? baseScale * 1.5 : baseScale;
            
            return Math.pow(finalScale, 3); // ← Volume = raio³ (para GPU escalar áreas perfeitamente)
        },
        radiusScale: 1, // Desativado o multiplicador de galáxia, agora usamos valores exatos volumétricos
        radiusUnits: 'common', // 🌍 Mudança para unidades globais para perspectiva natural
        radiusMinPixels: 2.0,  // 🔍 Garante legibilidade de longe
        radiusMaxPixels: 1500,
        pickable: true,
        opacity: 1,
        billboard: true,
        antialiasing: true,
        stroked: false,
        updateTriggers: {
            getFillColor: [activeNodeId, store.hoveredNodeId],
            getPosition: tickCounter,
            getRadius: [zoom, degreeCounts.size]
        },
        onHover,
        onClick,
        onDragStart,
        onDrag,
        onDragEnd
    });
}
