import * as d3 from 'd3-force-3d';

let simulation;
let nodesData = [];
let linksData = [];

self.onmessage = function (event) {
    const { type, payload } = event.data;

    if (type === 'INIT' || type === 'UPDATE_DATA') {
        const { nodes, links } = payload;

        const oldMap = new Map();
        nodesData.forEach(n => oldMap.set(n.id, n));

        nodesData = nodes.map(n => {
            const existing = oldMap.get(n.id);
            if (existing) return Object.assign(existing, n);
            // Nascimento esférico 3D (distribuição volumétrica polar)
            const r = Math.pow(Math.random(), 1 / 3) * 3000;
            const theta = Math.acos(2 * Math.random() - 1);
            const phi = 2 * Math.PI * Math.random();
            return {
                ...n,
                x: r * Math.sin(theta) * Math.cos(phi),
                y: r * Math.sin(theta) * Math.sin(phi),
                z: r * Math.cos(theta),
                vz: 0
            };
        });

        // FORÇA o Z em todos os nós (d3-force-3d v3 não o faz)
        nodesData.forEach(n => {
            if (n.z === null || n.z === undefined) {
                const r = Math.pow(Math.random(), 1 / 3) * 8000;
                const theta = Math.acos(2 * Math.random() - 1);
                n.z = r * Math.cos(theta);
            }
            if (n.vz === null || n.vz === undefined) n.vz = 0;
        });

        // Mapa de IDs O(1)
        const idMap = new Map();
        nodesData.forEach(n => idMap.set(n.id, n));

        // Cálculo de graus para detecção de Hubs (Física)
        const nodeDegrees = new Map();
        links.forEach(l => {
            const sid = typeof l.source === 'object' ? l.source.id : l.source;
            const tid = typeof l.target === 'object' ? l.target.id : l.target;
            nodeDegrees.set(sid, (nodeDegrees.get(sid) || 0) + 1);
            nodeDegrees.set(tid, (nodeDegrees.get(tid) || 0) + 1);
        });

        const HUB_PHYS_LIMIT = 40; // Aumentado para lidar com densidade v7.0de" em vez de "mola rígida"

        // Filtra links órfãos e remove links de HUBs da física rígida
        linksData = links.filter(l => {
            const sid = typeof l.source === 'object' ? l.source.id : l.source;
            const tid = typeof l.target === 'object' ? l.target.id : l.target;
            if (!idMap.has(sid) || !idMap.has(tid)) return false;

            // Se um dos lados for um Hub, removemos do forceLink para evitar o "colapso estrela"
            const sDeg = nodeDegrees.get(sid) || 0;
            const tDeg = nodeDegrees.get(tid) || 0;
            return sDeg <= HUB_PHYS_LIMIT && tDeg <= HUB_PHYS_LIMIT;
        });

        // Mapa de parentesco
        const parentMap = new Map();
        nodesData.forEach(n => {
            if (n.parent_gravity_id && idMap.has(n.parent_gravity_id)) {
                parentMap.set(n.id, idMap.get(n.parent_gravity_id));
            }
        });

        // Centros de comunidade Louvain (Esfera de Fibonacci 3D)
        const communityMap = new Map();
        nodesData.forEach(n => {
            if (n.community !== undefined && n.community !== null) {
                if (!communityMap.has(n.community)) communityMap.set(n.community, []);
                communityMap.get(n.community).push(n);
            }
        });

        const numC = Math.max(communityMap.size, 1);
        const communityCenters = new Map();
        const golden = Math.PI * (Math.sqrt(5) - 1);
        const gR = 45000; // SUPERNOVA: Raio galáctico massivamente expandido para separar ilhas
        Array.from(communityMap.keys()).forEach((cid, i) => {
            const yNorm = 1 - (i / Math.max(numC - 1, 1)) * 2;
            const rAtY = Math.sqrt(1 - yNorm * yNorm);
            const angle = golden * i;
            communityCenters.set(cid, {
                x: Math.cos(angle) * rAtY * gR,
                y: yNorm * gR,
                z: Math.sin(angle) * rAtY * gR
            });
        });

        console.log(`[Physics] ${nodesData.length} nós, ${parentMap.size} com pai, ${communityMap.size} comunidades, ${linksData.length} links válidos`);

        if (simulation) simulation.stop();

        // ═══════════════════════════════════════════════════════════
        // MOTOR CELESTIAL v5.0 — Z MANUAL (d3-force-3d v3 bug fix)
        // ═══════════════════════════════════════════════════════════

        function forceAll() {
            let nodes;
            function force(alpha) {
                for (let i = 0; i < nodes.length; i++) {
                    const node = nodes[i];

                    // === GRAVIDADE DE CLUSTER (Puxa para a comunidade) ===
                    const center = communityCenters.get(node.community);
                    if (center) {
                        const s = 0.005 * alpha; // GALACTIC EXPANSION: Pull reduzido para permitir dispersão
                        node.vx += (center.x - node.x) * s;
                        node.vy += (center.y - node.y) * s;
                        node.vz += (center.z - node.z) * s;
                    }

                    // === GRAVIDADE HIERÁRQUICA (Soft Pull para o Pai ou Hub) ===
                    const parent = parentMap.get(node.id);
                    if (parent) {
                        const s = 0.005 * alpha; // Pull reduzido para v8.0
                        node.vx += (parent.x - node.x) * s;
                        node.vy += (parent.y - node.y) * s;
                        node.vz += (parent.z - node.z) * s;
                    }

                    // === REPULSÃO Z CUSTOMIZADA (Evita achatamento) ===
                    const pushZ = 25 * alpha;
                    for (let j = i + 1; j < nodes.length; j++) {
                        const other = nodes[j];
                        const dz = (node.z || 0) - (other.z || 0);
                        const absZ = Math.abs(dz);
                        if (absZ < 400) {
                            const sign = dz >= 0 ? 1 : -1;
                            node.vz += sign * pushZ;
                            other.vz -= sign * pushZ;
                        }
                    }
                }
            }
            force.initialize = function (_n) { nodes = _n; };
            return force;
        }

        simulation = d3.forceSimulation(nodesData, 3) 
            .alphaDecay(0.04) // Resfriamento mais lento para dar tempo de expansão
            .velocityDecay(0.35) // Menos fricção para permitir viagens longas às galáxias
            
            .force('link', d3.forceLink(linksData).id(d => d.id)
                .distance(link => {
                    const sC = link.source?.community;
                    const tC = link.target?.community;
                    const sType = link.source?.['document-type'] || 'chunk';
                    const tType = link.target?.['document-type'] || 'chunk';
                    
                    if (sType === 'memory' || tType === 'memory') return 120; // Mais espaço para memórias
                    return (sC === tC) ? 850 : 15000; // 850px interno, 15000px entre Galáxias!
                })
                .strength(link => {
                    const sC = link.source?.community;
                    const tC = link.target?.community;
                    return (sC === tC) ? 0.3 : 0.02; // Links mais elásticos e suaves
                })
            )

            .force('cosmos', forceAll())

            .force('charge', d3.forceManyBody().strength(d => {
                const deg = nodeDegrees.get(d.id) || 0;
                const pr = (d.pagerank && d.pagerank > 0) ? (d.pagerank * 20) : deg;
                const baseRepulsion = -25000; // BIG BANG v8.0: Força bruta de separação
                
                if (deg > HUB_PHYS_LIMIT) return -185000; // Hubs agora são supernovas de repulsão
                return baseRepulsion - (pr * 500); 
            }).distanceMax(80000))
            
            .force('collide', d3.forceCollide().radius(d => {
                const deg = nodeDegrees.get(d.id) || 0;
                const pr = (d.pagerank && d.pagerank > 0) ? (d.pagerank * 20) : deg;
                if (deg > HUB_PHYS_LIMIT) return 2500; // Raio de influência massiva
                return (1 + Math.pow(pr, 0.5) * 15) + 70; // Buffer de colisão v8.0
            }));

        const velDecay = 0.3;
        let tickCount = 0;

        simulation.on('tick', () => {
            // ═══ INTEGRAÇÃO Z MANUAL ═══
            // d3-force-3d v3.0.6 não integra vz → z, fazemos manualmente
            for (let i = 0; i < nodesData.length; i++) {
                const n = nodesData[i];
                if (n.fz !== undefined && n.fz !== null) {
                    n.z = n.fz; // Fixed position (drag)
                    n.vz = 0;
                } else {
                    n.vz *= (1 - velDecay);
                    n.z = (n.z || 0) + n.vz;
                }
            }

            tickCount++;
            if (tickCount % 2 !== 0) return;
            const positions = nodesData.map(n => ({ id: n.id, x: n.x, y: n.y, z: n.z || 0 }));
            self.postMessage({ type: 'TICK', payload: { positions } });
        });

        simulation.tick(100); // Warmup
        simulation.alpha(0.6).restart();
    }

    else if (type === 'DRAG_START') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node && simulation) {
            simulation.alphaTarget(0.3).restart();
            node.fx = node.x; node.fy = node.y; node.fz = node.z;
        }
    }
    else if (type === 'DRAG') {
        const { nodeId, x, y, z } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node) { node.fx = x; node.fy = y; node.fz = z; }
    }
    else if (type === 'DRAG_END') {
        const { nodeId } = payload;
        const node = nodesData.find(n => n.id === nodeId);
        if (node && simulation) {
            simulation.alphaTarget(0);
            node.fx = null; node.fy = null; node.fz = null;
        }
    }
};
