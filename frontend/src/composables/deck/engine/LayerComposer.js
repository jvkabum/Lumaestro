import { createLabelLayer } from '../layers/LabelLayer.js';
import { createLinkLayer } from '../layers/LinkLayer.js';
import { createNodeLayer } from '../layers/NodeLayer.js';

/**
 * 🏗️ LayerComposer — O Arquiteto de Camadas
 * 
 * Responsável por calcular a densidade de conexões (degreeCounts) 
 * e montar o array final de camadas para o Deck.gl.
 */
export function useLayerComposer() {
    
    const compose = ({
        currentNodes,
        currentLinks,
        currentViewState,
        activeNodeId,
        animationTime,
        store,
        eventHandlers
    }) => {
        const zoom = currentViewState.zoom;
        const clLinks = store.clickedNodeLinks;
        const hlLinks = store.highlightedLinks;

        // Cálculo O(L) de densidade para uso compartilhado pelas camadas
        const degreeCounts = new Map();
        currentLinks.forEach(l => {
            // 🛡️ [ELITE FIX v18.8] Garante que IDs sejam Strings para compatibilidade com o NodeMap
            const sid = String(l.source?.id || l.source);
            const tid = String(l.target?.id || l.target);
            degreeCounts.set(sid, (degreeCounts.get(sid) || 0) + 1);
            degreeCounts.set(tid, (degreeCounts.get(tid) || 0) + 1);
        });

        // Montagem do Pipeline de Renderização
        return [
            // 1. Etiquetas (LOD) - CARTESIAN Sync
            createLabelLayer({ currentNodes, degreeCounts, zoom, store, tickCounter: animationTime }),
            
            // 2. Conexões + Fótons (GPU Sync V9)
            createLinkLayer({ currentLinks, clLinks, hlLinks, animationTime }),
            
            // 3. Nós (Interação) - CARTESIAN Sync
            createNodeLayer({ 
                currentNodes, degreeCounts, zoom, activeNodeId, store, tickCounter: animationTime,
                ...eventHandlers
            })
        ];
    };

    return { compose };
}
