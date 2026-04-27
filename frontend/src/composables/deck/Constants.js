/**
 * 🎨 useDeckConstants — Paleta de Cores e Escalas do Cosmos
 * 
 * Centraliza a identidade visual do Lumaestro em formato RGB WebGL.
 */
export const colors = {
    source: [244, 114, 182],   // Rosa Saturado
    page: [34, 211, 238],      // Cyan Vibrante
    chunk: [100, 160, 255],    // Azul Celeste
    system: [255, 255, 255],   // Branco Puro
    memory: [244, 114, 182],   // Rosa Quente (igual original)
    active: [252, 211, 77],     // Dourado
    celestial: [192, 132, 252], // Violeta Cósmico
    semantic: [34, 211, 238],   // Cyan (Sinapse)
    galaxyCore: [255, 200, 50], // Ouro (Núcleo)
    solarCore: [255, 100, 50],  // Laranja (Sistema)
    planet: [100, 200, 255],    // Azul claro
    asteroid: [180, 180, 200]   // Cinza (Memória)
};

export const communityColors = [
    [34, 211, 238],   // Cyan
    [167, 139, 250],  // Violeta
    [251, 146, 60],   // Laranja
    [74, 222, 128],   // Esmeralda
    [244, 114, 182],  // Rosa
    [250, 204, 21],   // Amarelo
    [56, 189, 248],   // Sky Blue
    [232, 121, 249]   // Fúcsia
];

export const getCommunityColor = (cid) => {
    if (cid === undefined || cid === null || cid < 0) return null;
    return communityColors[cid % communityColors.length];
};
