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
export function createLinkLayer({ currentLinks, clLinks, hlLinks, animationTime }) {
    return new NeuralLinkLayer({
        id: 'graph-edges-v9-surgical',
        coordinateSystem: COORDINATE_SYSTEM.CARTESIAN,
        data: [...currentLinks].filter(l => {
            if (l['edge-type'] === 'orbital') return false;
            const sObj = l.sourceObj;
            const tObj = l.targetObj;

            // 🛡️ Previne o "Nó Central Invisível" (Buraco Negro)
            // Se a aresta não tem um nó de origem ou destino válido, a ignoramos.
            if (!sObj || !tObj) return false;

            if (sObj['celestial-type'] === 'galaxy-core' || sObj['celestial-type'] === 'solar-system-core') return false;
            if (tObj['celestial-type'] === 'galaxy-core' || tObj['celestial-type'] === 'solar-system-core') return false;
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
        getHeight: 0.3, // Curva quase imperceptível para manter charme orgânico sem entortar a entrada no nó!
        animationTime,
        getOffset: (link, { index }) => index * 1.618,
        updateTriggers: {
            getSourceColor: [clLinks.size, hlLinks.size],
            getTargetColor: [clLinks.size, hlLinks.size],
            getSourcePosition: animationTime,
            getTargetPosition: animationTime
        }
    });
}
