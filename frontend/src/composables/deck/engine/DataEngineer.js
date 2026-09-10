/**
 * 🛠️ DataEngineer — O Arquiteto de Dados
 * 
 * Responsável por purificar os dados (JSON), gerenciar o mapeamento 
 * incremental de nós e garantir o espalhamento térmico inicial.
 */
export function useDataEngineer() {
    
    // Purifica dados do Vue (Proxies) para JSON puro (Deck.gl/Worker performance)
    const purify = (data) => JSON.parse(JSON.stringify(data));

    // Espalhamento Térmico Inicial (Bootstrap de Coordenadas)
    const bootstrapCoordinates = (nodes) => {
        return nodes.map(n => {
            if (n.x === undefined) {
                const r = Math.pow(Math.random(), 1 / 3) * 1200;
                const theta = Math.acos(2 * Math.random() - 1);
                const phi = 2 * Math.PI * Math.random();
                return {
                    ...n,
                    x: r * Math.sin(theta) * Math.cos(phi),
                    y: r * Math.sin(theta) * Math.sin(phi),
                    z: r * Math.cos(theta)
                };
            }
            return n;
        });
    };

    // Sincronização Incremental (Blindagem de Física e Metadados v18.5)
    const syncIncremental = (rawNodes, nodeMap, currentNodesRef) => {
        const pureNodes = purify(rawNodes);
        
        // 1. Reconciliação de Metadados (Mantendo referências físicas)
        const updatedList = pureNodes.map(n => {
            const sid = String(n.id);
            const sidLower = sid.toLowerCase();
            const existing = nodeMap.get(sid) || nodeMap.get(sidLower);

            if (!existing) {
                // Novo Nó: Nascimento Esférico
                const r = 200 + Math.random() * 500;
                const theta = Math.random() * 2 * Math.PI;
                const phi = Math.acos(2 * Math.random() - 1);
                
                const newNode = {
                    ...n,
                    x: r * Math.sin(phi) * Math.cos(theta),
                    y: r * Math.sin(phi) * Math.sin(theta),
                    z: r * Math.cos(phi)
                };
                nodeMap.set(sid, newNode);
                nodeMap.set(sidLower, newNode);
                return newNode;
            } else {
                // Nó Existente: Mescla metadados novos preservando coordenadas de física
                Object.assign(existing, { 
                    ...n, 
                    x: existing.x, 
                    y: existing.y, 
                    z: existing.z 
                });
                nodeMap.set(sid, existing);
                nodeMap.set(sidLower, existing);
                return existing;
            }
        });

        // 2. [CRÍTICO] Sincronização Reativa: Atualiza o array que o Deck.gl consome
        currentNodesRef.value = updatedList;

        return updatedList;
    };

    // Índice auxiliar para resolução rápida de nomes, caminhos e entidades
    const buildLookupIndex = (nodes) => {
        const index = new Map();
        if (!nodes || !Array.isArray(nodes)) return index;

        nodes.forEach(n => {
            if (!n || !n.id) return;
            const id = String(n.id);
            const idLow = id.toLowerCase();
            index.set(idLow, n);

            if (n.name) {
                const nameLow = String(n.name).toLowerCase().trim();
                index.set(nameLow, n);
            }

            const parts = id.split(':');
            if (parts.length >= 3) {
                const sub = parts.slice(2).join(':').toLowerCase();
                index.set(sub, n);
                index.set(sub.split('\\').join('/'), n);
                index.set(sub.split('/').join('\\'), n);
                
                const baseName = sub.split(/[\\/]/).pop();
                if (baseName && !index.has(baseName)) {
                    index.set(baseName, n);
                }
            }

            if (idLow.startsWith('asteroid:')) {
                const stripped = idLow.substring(9).trim();
                if (!index.has(stripped)) {
                    index.set(stripped, n);
                }
            }
        });

        return index;
    };

    // Resolução tolerante a falhas de IDs, nomes e caminhos
    const resolveNode = (key, nodeMap, lookupIndex) => {
        if (!key) return null;
        if (typeof key === 'object' && key !== null) {
            if (key.x !== undefined && key.y !== undefined) return key;
            if (key.id) key = key.id;
        }
        const k = String(key).trim();
        const kLow = k.toLowerCase();

        // 1. Busca direta no nodeMap
        if (nodeMap) {
            const direct = nodeMap.get(k) || nodeMap.get(kLow);
            if (direct) return direct;
        }

        // 2. Busca no índice de nomes/caminhos
        if (lookupIndex) {
            const found = lookupIndex.get(kLow) ||
                          lookupIndex.get(kLow.split('\\').join('/')) ||
                          lookupIndex.get(kLow.split('/').join('\\'));
            if (found) return found;

            if (!kLow.startsWith('asteroid:')) {
                const ast = lookupIndex.get('asteroid:' + kLow);
                if (ast) return ast;
            }
        }

        // 3. Fallback asteroid no nodeMap
        if (nodeMap && !kLow.startsWith('asteroid:')) {
            const ast = nodeMap.get('asteroid:' + kLow);
            if (ast) return ast;
        }

        return null;
    };

    // Mapeamento de Links com cura de referências e canonicalização de IDs
    const mapLinks = (rawEdges, nodeMap, lookupIndex) => {
        if (!rawEdges) return [];
        const pureEdges = purify(rawEdges);
        const resolvedLinks = [];

        pureEdges.forEach(link => {
            const sid = link.source;
            const tid = link.target;
            const sObj = resolveNode(sid, nodeMap, lookupIndex);
            const tObj = resolveNode(tid, nodeMap, lookupIndex);

            if (sObj && tObj && sObj.id !== tObj.id) {
                resolvedLinks.push({
                    ...link,
                    source: sObj.id,
                    target: tObj.id,
                    sourceObj: sObj,
                    targetObj: tObj
                });
            }
        });

        return resolvedLinks;
    };

    return { purify, bootstrapCoordinates, syncIncremental, mapLinks, buildLookupIndex, resolveNode };
}
