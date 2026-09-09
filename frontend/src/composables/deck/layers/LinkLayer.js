import { COORDINATE_SYSTEM } from '@deck.gl/core';
import { ArcLayer } from '@deck.gl/layers';
import { colors } from '../Constants';

// Módulo de animação compatível com a arquitetura V9
const animationModule = {
    name: 'animation_v9',
    uniformTypes: {
        u_time_v9_stable: 'f32'
    },
    defaultUniforms: {
        u_time_v9_stable: 0
    },
    // Declaramos o bloco explicitamente no vs e no fs para o assembler emitir em ambos
    vs: `
        uniform animation_v9Uniforms {
            float u_time_v9_stable;
        } animation_v9;
    `,
    fs: `
        uniform animation_v9Uniforms {
            float u_time_v9_stable;
        } animation_v9;
    `
};

/**
 * ⚡ NeuralLinkLayer — Camada de arestas com animação de fótons via GPU
 */
class NeuralLinkLayer extends ArcLayer {
    static defaultProps = {
        ...ArcLayer.defaultProps,
        animationTime: { type: 'number', value: 0, compare: true }
    };

    getShaders() {
        const shaders = super.getShaders();
        return {
            ...shaders,
            modules: [...(shaders.modules || []), animationModule],
            inject: {
                'vs:#decl': `
                    in float instanceOffsets;
                    out float vOffset;
                `,
                'vs:DECKGL_FILTER_GL_POSITION': `
                    vOffset = instanceOffsets;
                `,
                'fs:#decl': `
                    in float vOffset;
                    
                    vec2 get_neural_pulse(float progress, float offset) {
                        float phase = mod(animation_v9.u_time_v9_stable + offset, 1.0);
                        float dist = distance(progress, phase);
                        
                        // Core: Brilho central intenso (a bolinha)
                        float core = exp(-pow(dist * 110.0, 2.0));
                        // Halo: Aura de luz nítida
                        float halo = exp(-pow(dist * 40.0, 2.0));
                        
                        return vec2(core, halo);
                    }
                `,
                'fs:DECKGL_FILTER_COLOR': `
                    vec2 pulse = get_neural_pulse(geometry.uv.x, vOffset);
                    
                    // Brilho Aditivo
                    vec3 glowColor = color.rgb * 4.0; 
                    vec3 bloom = (glowColor * pulse.y) + (vec3(1.0, 1.0, 1.0) * pulse.x * 2.5);
                    
                    color.rgb += bloom;
                    
                    // Aumenta o alpha onde está o fóton, tornando-o opaco mesmo em links transparentes!
                    color.a = clamp(color.a + pulse.x + (pulse.y * 0.5), 0.0, 1.0);
                    
                    color.rgb = clamp(color.rgb, 0.0, 1.0);
                `
            }
        };
    }

    draw(opts) {
        const { model } = this.state;
        if (model && model.shaderInputs) {
            model.shaderInputs.setProps({
                animation_v9: {
                    u_time_v9_stable: this.props.animationTime || 0
                }
            });
        }
        super.draw(opts);
    }

    updateState({ props, oldProps, changeFlags }) {
        super.updateState({ props, oldProps, changeFlags });
        // Força o redesenho constante para a animação
        if (props.animationTime !== oldProps.animationTime) {
            this.setNeedsRedraw();
        }
    }

    initializeState() {
        super.initializeState();
        this.getAttributeManager().addInstanced({
            instanceOffsets: { size: 1, accessor: 'getOffset', defaultValue: 0 }
        });
    }
}

NeuralLinkLayer.layerName = 'NeuralLinkLayer';

/**
 * 🕸️ LinkLayer — A Teia Conectiva
 */
export function createLinkLayer({ currentLinks, clLinks, hlLinks, animationTime, store }) {
    const showConnections = store ? store.showConnections !== false : true;
    const connectionFilter = (store && store.connectionFilter) || 'all'; // 'all' | 'semantic' | 'focus'

    const isLinkInSet = (s, t, set) => {
        if (!set || set.size === 0) return false;
        const sLow = s.toLowerCase();
        const tLow = t.toLowerCase();
        return set.has(`${s}-${t}`) || set.has(`${t}-${s}`) ||
               set.has(`${sLow}-${tLow}`) || set.has(`${tLow}-${sLow}`);
    };

    return new NeuralLinkLayer({
        id: 'graph-edges-v9-surgical',
        coordinateSystem: COORDINATE_SYSTEM.CARTESIAN,
        data: [...currentLinks].filter(l => {
            const sObj = l.sourceObj;
            const tObj = l.targetObj;

            // 🛡️ Previne pontas nulas, nós inexistentes ou laços em si mesmo
            if (!sObj || !tObj) return false;
            if (sObj.id === tObj.id) return false;

            const s = String(l.source?.id || l.source);
            const t = String(l.target?.id || l.target);
            const isClicked = isLinkInSet(s, t, clLinks);
            const isHighlighted = isLinkInSet(s, t, hlLinks);

            // 🌟 Conexões ativas do nó selecionado SEMPRE aparecem
            if (isClicked || isHighlighted) return true;

            // Se o usuário desligou as conexões:
            if (!showConnections) return false;

            // Modo 'focus': exibe apenas as conexões do nó selecionado
            if (connectionFilter === 'focus') return false;

            const edgeType = l['edge-type'] || l['relation_type'] || l.relation_type || 'link';

            // Modo 'semantic': exibe conexões semânticas e links diretos, omitindo hierarquia orbital pura
            if (connectionFilter === 'semantic' && edgeType === 'orbital') return false;

            return true;
        }),
        getSourcePosition: link => link.sourceObj ? [link.sourceObj.x || 0, link.sourceObj.y || 0, link.sourceObj.z || 0] : [0, 0, 0],
        getTargetPosition: link => link.targetObj ? [link.targetObj.x || 0, link.targetObj.y || 0, link.targetObj.z || 0] : [0, 0, 0],
        getSourceColor: link => {
            const s = String(link.source?.id || link.source);
            const t = String(link.target?.id || link.target);
            const isClicked = isLinkInSet(s, t, clLinks);
            const isHighlighted = isLinkInSet(s, t, hlLinks);

            if (isClicked) return [0, 242, 255, 240]; // Azul ciano neon vibrante
            if (isHighlighted) return [252, 211, 77, 220]; // Dourado solar

            const type = link['edge-type'] || link['relation_type'] || link.relation_type || 'link';
            if (type === 'semantic' || type === 'recon_auto') return [167, 139, 250, 95]; // Violeta neural
            if (type === 'orbital') return [70, 150, 240, 50]; // Linha hierárquica cósmica sutil
            if (type === 'memory') return [244, 114, 182, 110]; // Rosa memória
            return [...colors.page, 75]; // Cyan vibrante padrão
        },
        getTargetColor: link => {
            const s = String(link.source?.id || link.source);
            const t = String(link.target?.id || link.target);
            const isClicked = isLinkInSet(s, t, clLinks);
            const isHighlighted = isLinkInSet(s, t, hlLinks);

            if (isClicked) return [0, 242, 255, 240];
            if (isHighlighted) return [252, 211, 77, 220];

            const type = link['edge-type'] || link['relation_type'] || link.relation_type || 'link';
            if (type === 'semantic' || type === 'recon_auto') return [34, 211, 238, 95];
            if (type === 'orbital') return [100, 180, 255, 50];
            if (type === 'memory') return [244, 114, 182, 110];
            return [40, 180, 180, 75];
        },
        getWidth: link => {
            const s = String(link.source?.id || link.source);
            const t = String(link.target?.id || link.target);
            const isClicked = isLinkInSet(s, t, clLinks);
            const isHighlighted = isLinkInSet(s, t, hlLinks);

            if (isClicked) return 2.0; // Destaque intenso
            if (isHighlighted) return 1.5;

            const type = link['edge-type'] || link['relation_type'] || link.relation_type || 'link';
            if (type === 'orbital') return 0.6; // Linha fina para hierarquia estrutural
            return 1.0;
        },
        getHeight: 0.25, // Curva quase imperceptível para manter charme orgânico sem entortar a entrada no nó!
        animationTime,
        getOffset: (link, { index }) => index * 1.618,
        updateTriggers: {
            getSourceColor: [clLinks?.size || 0, hlLinks?.size || 0, store?.connectionFilter, store?.showConnections],
            getTargetColor: [clLinks?.size || 0, hlLinks?.size || 0, store?.connectionFilter, store?.showConnections],
            getWidth: [clLinks?.size || 0, hlLinks?.size || 0],
            getSourcePosition: animationTime,
            getTargetPosition: animationTime,
            data: [store?.connectionFilter, store?.showConnections, clLinks?.size || 0]
        }
    });
}
