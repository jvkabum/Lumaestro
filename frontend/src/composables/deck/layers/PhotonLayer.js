import { ScatterplotLayer } from '@deck.gl/layers';
import { colors } from '../Constants';

/**
 * 💫 PhotonLayer — O Fluxo de Dados Atmosférico
 * 
 * Responsável por renderizar os fótons (partículas) que viajam pelos links.
 * Cria a sensação de um sistema vivo e em constante troca de informação.
 */
export function createPhotonLayer({ currentLinks, clLinks, hlLinks, animationTime, store }) {
    return new ScatterplotLayer({
        id: 'graph-photons',
        data: [...currentLinks],
        getPosition: (link, { index }) => {
            const s = link.sourceObj;
            const t = link.targetObj;
            if (!s || !t) return [0, 0, 0];
            
            // Phase offset para cada link (baseado no index) para evitar sincronia robótica
            const phase = (animationTime + (index * 0.137)) % 1.0;

            // 1. Interpolação Linear de Base (X, Y, Z)
            const lx = s.x + (t.x - s.x) * phase;
            const ly = s.y + (t.y - s.y) * phase;
            const lz = s.z + (t.z - s.z) * phase;

            // 2. Cálculo da Curvatura Parabólica (Sync com LinkLayer getHeight: 0.3)
            const dx = t.x - s.x;
            const dy = t.y - s.y;
            const dz = t.z - s.z;
            const distance = Math.sqrt(dx * dx + dy * dy + dz * dz);
            
            // h = 4 * H * t * (1 - t) -> Parábola perfeita que atinge o pico em t=0.5
            const arcHeight = distance * 0.3; // 0.3 é o multiplicador do LinkLayer
            const curveOffset = arcHeight * 4 * phase * (1 - phase);

            return [lx, ly, lz + curveOffset];
        },
        getFillColor: link => {
            const s = link.source.id || link.source;
            const t = link.target.id || link.target;

            // Fótons Brilhantes em trilhas ativas
            if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`) || hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) {
                return [...colors.active, 255]; 
            }

            // Efeito de pulso orgânico para fótons de fundo
            const pulse = (Math.sin(animationTime * 10 + (link.source.id?.length || 0)) + 1) / 2;
            return [...colors.page, 40 + (pulse * 60)]; 
        },
        getRadius: link => {
            const isActive = store.highlightedLinks.size > 0 || store.clickedNodeLinks.size > 0;
            return isActive ? 1.8 : 1.2;
        },
        radiusUnits: 'pixels',
        billboard: true,
        updateTriggers: {
            getPosition: animationTime,
            getFillColor: [clLinks.size, hlLinks.size]
        }
    });
}
