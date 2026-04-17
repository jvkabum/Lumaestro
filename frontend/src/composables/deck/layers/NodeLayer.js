
import { ScatterplotLayer } from '@deck.gl/layers';
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
            // 📏 ESCALONAMENTO POR IMPORTÂNCIA ESTRUTURAL (Paridade v14.1)
            const deg = degreeCounts.get(node.id) || node.degree || 0;
            const pr = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : deg;

            // Fator de Tipo (Fontes e Sistemas são naturalmente maiores)
            const type = node['document-type'] || 'chunk';
            const typeFactor = type === 'source' ? 1.8 : (type === 'system' ? 2.2 : (type === 'page' ? 1.4 : 1.0));

            const zoomBoost = Math.max(0.40, Math.pow(2, zoom + 1.5));

            // Fórmula original preservada
            const baseScale = (5 + Math.pow(Math.max(deg, pr), 0.7) * 2.5) * typeFactor;
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
        updateTriggers: {
            getFillColor: [activeNodeId, store.hoveredNodeId],
            getPosition: tickCounter,
            getRadius: [zoom]
        },
        onHover,
        onClick,
        onDragStart,
        onDrag,
        onDragEnd
    });
}
