import * as d3 from 'd3-force-3d';

let simulation;
let nodesData = [];
let linksData = [];

// Função determinística para espalhamento estável sem comunidades
function seedRandom(str) {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
        hash = ((hash << 5) - hash) + str.charCodeAt(i);
        hash |= 0;
    }
    return (Math.abs(hash) % 1000) / 1000;
}

self.onmessage = function (event) {
    const { type, payload } = event.data;

    if (type === 'INIT') {
        const { nodes, links } = payload;
        nodesData = nodes;
        linksData = links;

        // Limpa a simulação anterior se existir
        if (simulation) {
            simulation.stop();
        }

        // ═══════════════════════════════════════════════════════════
        // FÍSICA DE NEBULOSAS v4 — SEGREGAÇÃO POR CLUSTER
        // ═══════════════════════════════════════════════════════════
        
        const communityMap = new Map();
        nodesData.forEach(n => {
            if (n.community !== undefined) {
                if (!communityMap.has(n.community)) communityMap.set(n.community, []);
                communityMap.get(n.community).push(n);
            }
        });

        // 🎯 Calcular centros ideais (Esfera de Fibonacci 3D - Espiral de Ouro)
        const numCommunities = communityMap.size || 1;
        const communityCenters = new Map();
        const phi_golden = Math.PI * (Math.sqrt(5) - 1); // Ângulo de Ouro

        Array.from(communityMap.keys()).forEach((cid, i) => {
            const y = 1 - (i / (numCommunities - 1)) * 2; // y vai de 1 a -1
            const radiusAtY = Math.sqrt(1 - y * y); // Raio no círculo horizontal
            const theta = phi_golden * i; // Ângulo de giro
            
            const dist = 3500; // Raio da galáxia de nebulosas aumentado drasticamente
            communityCenters.set(cid, {
                x: Math.cos(theta) * radiusAtY * dist,
                y: y * dist,
                z: Math.sin(theta) * radiusAtY * dist
            });
        });

        simulation = d3.forceSimulation(nodesData)
            .numDimensions(3)
            
            // LINKS: Curtos dentro do cluster, LONGOS entre clusters
            .force('link', d3.forceLink(linksData).id(d => d.id).distance(link => {
                const sComm = link.source.community;
                const tComm = link.target.community;
                
                if (sComm !== undefined && tComm !== undefined && sComm === tComm) {
                    return 100; // Unidos na mesma nebulosa
                }
                return 2500; // Pontes longas entre galáxias distantes
            }).strength(link => {
                return link.source.community === link.target.community ? 0.6 : 0.05;
            }))
            
            // REPULSÃO: Expandida para empurrar galáxias vizinhas
            .force('charge', d3.forceManyBody().strength(-2500).distanceMax(3000))
            
            // FORÇA DE CLUSTER: Puxa cada nó para o centro da sua nebulosa (Reforçada)
            .force('communityX', d3.forceX(d => {
                if (communityCenters.has(d.community)) return communityCenters.get(d.community).x;
                return (seedRandom(d.id) - 0.5) * 500;
            }).strength(0.4))
            .force('communityY', d3.forceY(d => {
                if (communityCenters.has(d.community)) return communityCenters.get(d.community).y;
                return (seedRandom(d.id) - 0.5) * 500;
            }).strength(0.4))
            .force('communityZ', d3.forceZ(d => {
                if (communityCenters.has(d.community)) return communityCenters.get(d.community).z;
                return (seedRandom(d.id) - 0.5) * 500;
            }).strength(0.4))
            
            .force('collide', d3.forceCollide(d => {
                const isElite = (d.degree > 35) || (d['document-type'] === 'source');
                const isImportant = (d.degree > 12) || (d['document-type'] === 'memory');
                if (isElite) return 100;    // Bolha massiva para nomes grandes
                if (isImportant) return 60; // Espaço para memórias
                return 30;                 // Base para o resto
            }).iterations(3))
            
            .alphaDecay(0.01)    
            .velocityDecay(0.3); // Permite mais movimento de expansão

        let tickCount = 0;
        simulation.on('tick', () => {
            tickCount++;
            // Throttle: Envia a cada 2 ticks para poupar o barramento Worker↔Main
            if (tickCount % 2 !== 0) return;

            // NUNCA envie o objeto 'link' ou 'node' inteiro → D3 injeta refs cíclicas
            const positions = nodesData.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z }));
            
            self.postMessage({
                type: 'TICK',
                payload: { positions }
            });
        });

        // Warmup: 200 ticks silenciosos → Layout já nasce distribuído
        simulation.tick(300);
    } 
    
    else if (type === 'UPDATE_DATA') {
        const { nodes, links } = payload;
        
        const nodeMap = new Map();
        nodesData.forEach(n => nodeMap.set(n.id, n));

        // Mescla nós preservando o estado físico (x, y, z, vx, vy, vz)
        nodesData = nodes.map(n => {
            const existing = nodeMap.get(n.id);
            if (existing) return Object.assign(existing, n);
            // Novos nós iniciam próximos ao centro para evitar explosões
            return {
                ...n,
                x: n.x || (Math.random() - 0.5) * 100,
                y: n.y || (Math.random() - 0.5) * 100,
                z: n.z || (Math.random() - 0.5) * 100
            };
        });

        linksData = links;

        if (simulation) {
            // Recalcular as comunidades presentes nos novos dados
            const communityMap = new Map();
            nodesData.forEach(n => {
                if (n.community !== undefined) {
                    if (!communityMap.has(n.community)) communityMap.set(n.community, []);
                    communityMap.get(n.community).push(n);
                }
            });

            const numCommunities = communityMap.size || 1;
            const updatedCenters = new Map();
            const phi_golden = Math.PI * (Math.sqrt(5) - 1);

            Array.from(communityMap.keys()).forEach((cid, i) => {
                const y = 1 - (i / (numCommunities - 1 || 1)) * 2;
                const radiusAtY = Math.sqrt(1 - y * y);
                const theta = phi_golden * i;
                
                const dist = 3500;
                updatedCenters.set(cid, {
                    x: Math.cos(theta) * radiusAtY * dist,
                    y: y * dist,
                    z: Math.sin(theta) * radiusAtY * dist
                });
            });

            simulation.nodes(nodesData);
            
            // Atualizar as forças para os novos centros (Reforçadas)
            simulation.force('communityX', d3.forceX(d => updatedCenters.has(d.community) ? updatedCenters.get(d.community).x : 0).strength(0.4));
            simulation.force('communityY', d3.forceY(d => updatedCenters.has(d.community) ? updatedCenters.get(d.community).y : 0).strength(0.4));
            simulation.force('communityZ', d3.forceZ(d => updatedCenters.has(d.community) ? updatedCenters.get(d.community).z : 0).strength(0.4));
            
            simulation.force('link').links(linksData);
            simulation.alpha(0.5).restart();
        }
    }
    
    else if (type === 'DRAG_START') {

        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node) {
            simulation.alphaTarget(0.3).restart();
            node.fx = node.x;
            node.fy = node.y;
            node.fz = node.z;
        }
    } 
    
    else if (type === 'DRAG') {
        const { nodeId, x, y, z } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node) {
            node.fx = x;
            node.fy = y;
            node.fz = z;
        }
    } 
    
    else if (type === 'DRAG_END') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node) {
            simulation.alphaTarget(0);
            node.fx = null;
            node.fy = null;
            node.fz = null;
        }
    }
};
