import { ScatterplotLayer } from '@deck.gl/layers';
import { COORDINATE_SYSTEM } from '@deck.gl/core';
import { colors, getCommunityColor } from '../Constants';

/**
 * 🪐 NeuralNodeLayer — Esferas 3D Falsas (Impostors)
 * 
 * Subclasse que injeta um shader avançado no ScatterplotLayer para 
 * simular geometria tridimensional com reflexão, luz direcional 
 * e brilho especular, mantendo o custo de performance a base de 2 triângulos (2D).
 */
class NeuralNodeLayer extends ScatterplotLayer {
    getShaders() {
        const shaders = super.getShaders();
        return {
            ...shaders,
            inject: {
                'fs:DECKGL_FILTER_COLOR': `
                    // No ScatterplotLayer, geometry.uv já varia de -1.0 a 1.0 partindo do centro
                    vec2 coord = geometry.uv;
                    float radiusSq = dot(coord, coord);
                    
                    // Suaviza a borda como uma esfera (anti-aliasing)
                    if (radiusSq > 1.0) discard;
                    
                    // Reconstrói a Normal da Esfera (eixo Z é calculado via Pitágoras: X² + Y² + Z² = R²)
                    float z = sqrt(1.0 - radiusSq);
                    vec3 normal = normalize(vec3(coord.x, coord.y, z));
                    
                    // Posicionamento da Luz no Cenário (Luz vindo do alto e um pouco pela frente e esquerda)
                    vec3 lightDir = normalize(vec3(-0.6, -0.8, 1.2)); 
                    
                    // 1. Luz Ambiente (Luz base para área de sombra)
                    float ambient = 0.35;
                    
                    // 2. Luz Difusa (O relevo batendo o sol)
                    float diff = max(dot(normal, lightDir), 0.0);
                    
                    // 3. Reflexo Especular (Aquele brilho molhado de bilhar)
                    vec3 viewDir = vec3(0.0, 0.0, 1.0); // Câmera olhando de frente
                    vec3 halfVector = normalize(lightDir + viewDir);
                    // Brilho intenso e focado (fator 64.0 é a estreiteza do brilho)
                    float spec = pow(max(dot(normal, halfVector), 0.0), 32.0);
                    
                    // Misturando a Luz com a Cor original do Node
                    vec3 finalColor = color.rgb * (ambient + diff * 0.75);
                    
                    // O brilho especular sempre puxa pro branco
                    finalColor += vec3(1.0, 1.0, 1.0) * spec * 0.5;
                    
                    color.rgb = clamp(finalColor, 0.0, 1.0);
                `
            }
        };
    }
}

NeuralNodeLayer.layerName = 'NeuralNodeLayer';

/**
 * 🟣 NodeLayer — As Esferas de Conhecimento
 * 
 * Responsável por renderizar os nós (documentos, memórias, sistemas).
 * Inclui os algoritmos de escalonamento v9.0 e os eventos de interação (Hover, Click, Drag).
 */
export function createNodeLayer({
    currentNodes,
    degreeCounts,
    zoom,
    activeNodeId,
    store,
    tickCounter,
    onHover,
    onClick,
    onDragStart,
    onDrag,
    onDragEnd
}) {
    return new NeuralNodeLayer({
        id: 'graph-nodes',
        coordinateSystem: COORDINATE_SYSTEM.CARTESIAN,
        data: [...currentNodes], // Clone para garantir atualização no Deck.gl
        getPosition: node => [node.x || 0, node.y || 0, node.z || 0],
        getFillColor: node => {
            if (node.id === activeNodeId) return colors.active;
            if (store.hoveredNodeId === node.id) return [...colors.active];

            // Cor da Comunidade (Cluster Semântico)
            const cCol = getCommunityColor(node.community);
            if (cCol) return [...cCol, 230];

            // Fallback por tipo de documento
            const type = node['document-type'] || 'chunk';
            return colors[type] ? [...colors[type], 220] : [155, 155, 155, 220];
        },
        getRadius: node => {
            // 📏 HIERARQUIA VISUAL (Volumétrica de Volume = Raio^3)
            const deg = degreeCounts.get(String(node.id)) || node.degree || 0;
            const pr = (node.pagerank && node.pagerank > 0) ? (node.pagerank * 15) : deg;

            const isActive = node.id === activeNodeId;
            const importance = Math.max(deg, pr);
            
            // Scaled Down para parâmetros de densidade tradicionais (Snippet Referência)
            const baseScale = 1.0 + Math.pow(importance, 0.5) * 0.4;
            const finalScale = isActive ? baseScale * 1.5 : baseScale;
            
            return Math.pow(finalScale, 3); // ← Volume = raio³ (para GPU escalar áreas perfeitamente)
        },
        radiusScale: 1, // Desativado o multiplicador de galáxia, agora usamos valores exatos volumétricos
        radiusUnits: 'common', // 🌍 Mudança para unidades globais para perspectiva natural
        radiusMinPixels: 2.0,  // 🔍 Garante legibilidade de longe
        radiusMaxPixels: 1500,
        pickable: true,
        opacity: 1,
        billboard: true,
        antialiasing: true,
        stroked: false,
        updateTriggers: {
            getFillColor: [activeNodeId, store.hoveredNodeId],
            getPosition: tickCounter,
            getRadius: [zoom, degreeCounts.size]
        },
        onHover,
        onClick,
        onDragStart,
        onDrag,
        onDragEnd
    });
}
