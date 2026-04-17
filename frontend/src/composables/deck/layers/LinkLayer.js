import { ArcLayer } from '@deck.gl/layers';
import { colors } from '../Constants';

/**
 * 🕸️ LinkLayer — A Teia Conectiva
 * 
 * Responsável por renderizar os arcos neurais que conectam as ideias.
 * Inclui lógica de destaque de trilhas (Trails) e filtragem de objetos celestiais.
 */
export function createLinkLayer({ currentLinks, clLinks, hlLinks, tickCounter }) {
    return new ArcLayer({
        id: 'graph-edges',
        data: [...currentLinks].filter(l => {
            // Filtra links orbitais ou que conectam núcleos de sistema/galáxia (para manter clareza)
            if (l['edge-type'] === 'orbital') return false;
            const sObj = l.sourceObj;
            const tObj = l.targetObj;
            if (sObj && (sObj['celestial-type'] === 'galaxy-core' || sObj['celestial-type'] === 'solar-system-core')) return false;
            if (tObj && (tObj['celestial-type'] === 'galaxy-core' || tObj['celestial-type'] === 'solar-system-core')) return false;
            return true;
        }),
        getSourcePosition: link => link.sourceObj ? [link.sourceObj.x || 0, link.sourceObj.y || 0, link.sourceObj.z || 0] : [0, 0, 0],
        getTargetPosition: link => link.targetObj ? [link.targetObj.x || 0, link.targetObj.y || 0, link.targetObj.z || 0] : [0, 0, 0],
        getSourceColor: link => {
            const s = link.source.id || link.source;
            const t = link.target.id || link.target;
            if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return [255, 255, 255, 220]; 
            if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return [...colors.active, 200];  
            return [...colors.page, 60]; 
        },
        getTargetColor: link => {
            const s = link.source.id || link.source;
            const t = link.target.id || link.target;
            if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return [255, 255, 255, 220];
            if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return [252, 211, 77, 200];
            return [40, 180, 180, 60];
        },
        getWidth: link => {
            const s = link.source.id || link.source;
            const t = link.target.id || link.target;
            if (clLinks.has(`${s}-${t}`) || clLinks.has(`${t}-${s}`)) return 2.5;
            if (hlLinks.has(`${s}-${t}`) || hlLinks.has(`${t}-${s}`)) return 1.8;
            return 0.5;
        },
        getHeight: 0.3, // Curvatura 3D suave
        greatCircle: false,
        updateTriggers: {
            getSourceColor: [clLinks.size, hlLinks.size],
            getTargetColor: [clLinks.size, hlLinks.size],
            getSourcePosition: tickCounter,
            getTargetPosition: tickCounter
        }
    });
}
