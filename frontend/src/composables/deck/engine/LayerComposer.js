// Importação das micro-layers atômicas
import { createLabelLayer } from '../layers/LabelLayer.js';
import { createLinkLayer } from '../layers/LinkLayer.js';
import { createNodeLayer } from '../layers/NodeLayer.js';
import { createPhotonLayer } from '../layers/PhotonLayer.js';

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
            const sid = l.source?.id || l.source;
            const tid = l.target?.id || l.target;
            degreeCounts.set(sid, (degreeCounts.get(sid) || 0) + 1);
            degreeCounts.set(tid, (degreeCounts.get(tid) || 0) + 1);
        });

        // Montagem do Pipeline de Renderização
        return [
            // 1. Etiquetas (LOD)
            createLabelLayer({ currentNodes, degreeCounts, zoom, store, tickCounter: animationTime }),
            
            // 2. Conexões (Arcos)
            createLinkLayer({ currentLinks, clLinks, hlLinks, tickCounter: animationTime }),
            
            // 3. Nós (Interação)
            createNodeLayer({ 
                currentNodes, degreeCounts, zoom, activeNodeId, store, tickCounter: animationTime,
                ...eventHandlers
            }),

            // 4. Atmosfera (Fótons)
            createPhotonLayer({ currentLinks, clLinks, hlLinks, animationTime, store })
        ];
    };

    return { compose };
}
