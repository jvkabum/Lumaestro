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

    // Sincronização Incremental (Blindagem de Física)
    const syncIncremental = (rawNodes, nodeMap, currentNodes) => {
        const pureNodes = purify(rawNodes);

        pureNodes.forEach(n => {
            const sid = String(n.id);
            if (!nodeMap.has(sid)) {
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
                currentNodes.push(newNode);
            } else {
                // Nó Existente: Atualiza metadados sem tocar nas coordenadas físicas
                const existing = nodeMap.get(sid);
                Object.assign(existing, { 
                    ...n, 
                    x: existing.x, 
                    y: existing.y, 
                    z: existing.z 
                });
            }
        });

        return pureNodes;
    };

    // Mapeamento de Links (Cura de referências)
    const mapLinks = (rawEdges, nodeMap) => {
        const pureEdges = purify(rawEdges);
        return pureEdges.map(link => {
            const sid = String(typeof link.source === 'object' ? link.source.id : link.source);
            const tid = String(typeof link.target === 'object' ? link.target.id : link.target);
            return {
                ...link,
                sourceObj: nodeMap.get(sid),
                targetObj: nodeMap.get(tid)
            };
        });
    };

    return { purify, bootstrapCoordinates, syncIncremental, mapLinks };
}
