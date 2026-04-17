import { TextLayer } from '@deck.gl/layers';
import { colors } from '../Constants';

/**
 * 🏷️ LabelLayer — O Tipógrafo do Cosmos
 * 
 * Responsável por renderizar etiquetas de texto inteligentes com LOD (Level of Detail).
 * As labels aparecem/somem dinamicamente baseadas no zoom e na importância do nó.
 */
export function createLabelLayer({ currentNodes, degreeCounts, zoom, store, tickCounter }) {
    return new TextLayer({
        id: 'graph-labels',
        data: currentNodes,
        visible: store.showLabels !== false,
        getPosition: node => [node.x || 0, node.y || 0, node.z || 0],
        getText: node => {
            const deg = degreeCounts.get(node.id) || node.degree || 0;
            const isElite = (deg > 15) || (node['document-type'] === 'source') || (node['document-type'] === 'system');
            const isImportant = (deg > 5);
            const isMemory = node['document-type'] === 'memory';
            const isHovered = store.hoveredNodeId === node.id;
            const isSelected = store.selectedNodeId === node.id;

            // LOD 1: Elite, Hovered ou Selecionado (Visível de longe)
            if ((isElite || isHovered || isSelected) && zoom > -3.0) {
                const name = node.name || 'Nó';
                return name.length > 20 ? name.substring(0, 18) + '..' : name;
            }

            // LOD 2: Memória ou Importante (Visível de perto médio)
            if ((isMemory || isImportant) && zoom > -1.0) {
                const name = node.name || 'Dado';
                return name.length > 18 ? name.substring(0, 16) + '..' : name;
            }

            // LOD 3: Micro-detalhes (Visível apenas em zoom máximo)
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

            let alpha = 0;
            if (isElite || store.hoveredNodeId === node.id || store.selectedNodeId === node.id) {
                alpha = Math.max(0, Math.min(255, (zoom + 3.2) * 200));
            } else if (isMemory || isImportant) {
                alpha = Math.max(0, Math.min(255, (zoom + 1.2) * 200));
            } else {
                alpha = Math.max(0, Math.min(255, (zoom - 1.0) * 200));
            }

            if (isMemory) return [...colors.memory, alpha];
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
    });
}
