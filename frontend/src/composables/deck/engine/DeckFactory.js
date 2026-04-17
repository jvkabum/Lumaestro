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
            
            getTooltip: ({ object }) => {
                if (!object || !object.name) return null;
                const limitStr = (str, len) => str.length > len ? str.substring(0, len) + '...' : str;
                
                const summaryText = object.summary || object['what-it-does'] || '';
                const summaryHtml = summaryText ? 
                    `<div style="margin-top: 8px; padding-top: 8px; border-top: 1px dashed rgba(0,242,255,0.2); font-size: 11.5px; opacity: 0.85; max-width: 250px; line-height: 1.4; white-space: normal;">
                       ${limitStr(summaryText, 220)}
                     </div>` : '';

                return {
                    html: `
                        <div style="font-weight: 700; font-size: 14px; letter-spacing: 0.5px; margin-bottom: 2px;">${object.name.toUpperCase()}</div>
                        <div style="font-size: 11px; color: #00f2ff; text-transform: uppercase;">▶ ${object['document-type'] || 'Conceito'}</div>
                        ${summaryHtml}
                    `,
                    style: {
                        backgroundColor: 'rgba(10, 15, 30, 0.95)', color: '#fff', borderRadius: '6px',
                        padding: '12px 14px', border: '1px solid rgba(0, 242, 255, 0.25)',
                        fontFamily: 'Inter, sans-serif',
                        boxShadow: '0 8px 32px rgba(0, 242, 255, 0.15)', backdropFilter: 'blur(12px)',
                        zIndex: 9999, pointerEvents: 'none'
                    }
                };
            },
            
            layers: [],
            parameters: { antialias: true, depthTest: true, blend: true }
        });
    };

    return { createDeck };
}
