import { Deck, OrbitView } from '@deck.gl/core';

/**
 * 🏭 DeckFactory — Fábrica de Visualização Premium (Vanilla / Limpa)
 * 
 * Configurações de ViewState, Tooltips CSS animados e OrbitView bounds.
 */
export function useDeckFactory() {

    const NAV_CONFIG = {
        minZoom: -15, maxZoom: 50,
        minPitch: -Infinity, maxPitch: Infinity, // Total liberdade espacial (looping 360 infinito)
        zoomSpeed: 1.2, dragSpeed: 1.0, rotateSpeed: 0.8,
        inertia: 0.15
    };

    const createDeck = ({ 
        containerRef, 
        currentViewState, 
        onViewStateChange
    }) => {
        // 🛑 Bloqueia o menu de botão direito nativo do Windows/Mac 
        // para permitir exclusividade funcional física no Lumaestro.
        if (containerRef) {
            containerRef.addEventListener('contextmenu', e => e.preventDefault());
        }

        return new Deck({
            parent: containerRef,
            initialViewState: {
                ...currentViewState,
                minZoom: NAV_CONFIG.minZoom, maxZoom: NAV_CONFIG.maxZoom,
                minPitch: NAV_CONFIG.minPitch, maxPitch: NAV_CONFIG.maxPitch,
                minRotationX: NAV_CONFIG.minPitch, maxRotationX: NAV_CONFIG.maxPitch // Nome correto para liberar a trava do OrbitView
            },
            
            onViewStateChange: (view) => {
                if (onViewStateChange) onViewStateChange(view);
            },
            
            controller: {
                dragPan: true, dragRotate: false,
                scrollZoom: true, touchZoom: true, touchRotate: true,
                keyboard: false, 
                zoomSpeed: NAV_CONFIG.zoomSpeed, dragSpeed: NAV_CONFIG.dragSpeed, rotateSpeed: NAV_CONFIG.rotateSpeed,
                inertia: NAV_CONFIG.inertia,
                minZoom: NAV_CONFIG.minZoom, maxZoom: NAV_CONFIG.maxZoom, 
                minPitch: NAV_CONFIG.minPitch, maxPitch: NAV_CONFIG.maxPitch,
                minRotationX: NAV_CONFIG.minPitch, maxRotationX: NAV_CONFIG.maxPitch
            },
            
            views: new OrbitView({
                orbitAxis: 'Y',
                orbitTarget: [0, 0, 0],
                near: 0.00001, far: 1000000, fovy: 50
            }),
            
            getTooltip: ({ object }) => object && object.name ? {
                text: `${object.name}\nTipo: ${object['document-type'] || 'Conceito'}`,
                style: {
                    backgroundColor: 'rgba(15, 23, 42, 0.95)', color: '#fff', borderRadius: '8px',
                    padding: '10px 14px', border: '1px solid rgba(0, 242, 255, 0.3)',
                    fontFamily: 'Inter, sans-serif', fontSize: '13px',
                    boxShadow: '0 4px 20px rgba(0, 0, 0, 0.4)', backdropFilter: 'blur(8px)',
                    zIndex: 9999, animation: 'tooltipFadeIn 0.2s ease-out'
                }
            } : null,
            
            layers: [],
            parameters: { antialias: true, depthTest: true, blend: true }
        });
    };

    return { createDeck };
}
