import { Deck, OrbitView } from '@deck.gl/core';

/**
 * 🏭 DeckFactory — A Fábrica de Visualização
 * 
 * Responsável por configurar e instanciar o objeto Deck.gl principal.
 * Centraliza definições de Tooltips, Views e comportamentos de ViewState.
 */
export function useDeckFactory() {

    const createDeck = ({ containerRef, currentViewState, onViewStateChange }) => {
        return new Deck({
            parent: containerRef,
            initialViewState: currentViewState,
            getTooltip: ({ object }) => object && object.name ? {
                text: `${object.name}\nTipo: ${object['document-type'] || 'Conceito'}`,
                style: {
                    backgroundColor: 'rgba(15, 23, 42, 0.95)',
                    color: '#fff',
                    borderRadius: '6px',
                    padding: '8px 12px',
                    border: '1px solid rgba(255, 255, 255, 0.1)',
                    fontFamily: 'Inter, sans-serif',
                    fontSize: '13px',
                    zIndex: 9999
                }
            } : null,
            onViewStateChange,
            views: new OrbitView({ orbitAxis: 'Y', near: 0.1, far: 50000 }),
            layers: []
        });
    };

    return { createDeck };
}
