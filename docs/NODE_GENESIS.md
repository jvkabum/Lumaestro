---
title: "Gênese de Nós e Neurônios"
type: "architecture"
status: "active"
tags: ["core", "graph", "crawler", "weaver"]
---

# 🧬 Gênese de Nós e Neurônios 🪐

> [!ABSTRACT]
> Este documento detalha os processos bio-digitais de nascimento e propagação de matéria no Grafo do Lumaestro. Aqui definimos como arquivos mortos se tornam neurônios vivos e como conversas efêmeras se cristalizam em sinapses permanentes.

## 🧠 Visão Geral da Matéria

No ecossistema Lumaestro, um **Nó** não é apenas um registro de banco de dados; ele é uma entidade celestial com gravidade, massa e propósito. O sistema opera em três planos de existência para a criação de matéria:

### 1. 📂 O Plano Estrutural (O Crawler)
Esta é a materialização do seu trabalho físico no disco rígido.
- **Gatilho:** Ciclo de `Scan` automático ou manual do Workspace.
- **Processo:** O motor percorre o sistema de arquivos, gera um hash SHA-256 único por caminho absoluto e extrai metadados locais (headers, funções, resumos).
- **Identidade Visual:** **Luas (Moons)** ou **Planetas**. Possuem "Órbita Física", ficando presos magneticamente às suas pastas de origem.

### 🧠 2. O Plano Cognitivo (O Knowledge Weaver)
Criação de conhecimento a partir da inteligência pura e extração semântica.
- **Gatilho:** Interações via Chat. A IA identifica afirmações de alto valor.
- **Processo:** O `Weaver` extrai triplas semânticas e as vetoriza (Embeddings). Não dependem de arquivos físicos pré-existentes.
- **Identidade Visual:** **Asteroides** ou **Neurônios Flutuantes**. Possuem "Órbita Semântica", aproximando-se de outros nós por afinidade de sentido, não por localização de pasta.

### 🌞 3. O Plano Primordial (O Galaxy Core)
O ponto de ancoragem de toda a galáxia do projeto.
- **Gatilho:** Inicialização do Workspace Ativo.
- **Processo:** Cálculo da raiz do projeto e atribuição de massa crítica (`100.0`). Serve como o ponto 0,0,0 da bússola gravitacional.
- **Identidade Visual:** **Sol Central (Sun)**. O maior nó do sistema, responsável por impedir a dispersão da matéria no vácuo 3D.

---

### 📊 Tabela Comparativa de Gênese

| Característica | **Crawler (Arquivo)** | **Weaver (Memória)** | **Core (Âncora)** |
| :--- | :--- | :--- | :--- |
| **Representação** | Lua / Planeta | Asteroide / Neurônio | Sol / Núcleo |
| **Origem** | Disco Rígido (Realidade) | Conversa (Conhecimento) | Configuração (Estrutura) |
| **Atração** | Por Pasta (Hierárquica) | Por Sentido (Semântica) | Global (Centro) |
| **Propósito** | Localizar Código/Notas | Raciocínio da IA | Organização da Galáxia |

---

## 🕸️ Fluxo de Dados: Ciclo de Vida da Informação

Para compreender a gênese de nós, dividimos a visualização em duas camadas: a **Lógica** (o que acontece na mente do usuário) e a **Técnica** (o que acontece na infraestrutura).

### A. Visão Conceitual (A Trindade da Matéria)
Este fluxo demonstra como as três fontes de matéria alimentam o Universo 3D através de suas respectivas esteiras de processamento.

```mermaid
flowchart TD
    %% ==========================================
    %% 1. DEFINIÇÃO DE CLASSES (Lumaestro Dark Mode)
    %% ==========================================
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef action fill:#2d333b,stroke:#455a64,stroke-width:1px,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef db fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    classDef deck fill:#0a0a0a,stroke:#9c27b0,stroke-width:3px,color:#fff

    classDef sol fill:#ffcc00,stroke:#ff9900,stroke-width:4px,color:#000
    classDef lua fill:#455a64,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ast fill:#9c27b0,stroke:#fff,stroke-width:2px,color:#fff

    %% ==========================================
    %% 2. CAMADAS ARQUITETURAIS
    %% ==========================================
    
    subgraph L1 [1. Gatilhos de Origem]
        direction LR
        T1([fa:fa-folder-open Abre Workspace])
        T2([fa:fa-search Scan de Disco])
        T3([fa:fa-comment-dots Prompt de Usuário])
    end

    subgraph L2 [2. Motores de Processamento]
        %% Esteira Primordial (Ancoragem)
        B1{fa:fa-server Go Core}
        H1[Gera Hash de Raiz]

        %% Esteira Física (Estrutural)
        C2[fa:fa-spider Crawler]
        D2[Deduplicação SHA-256]
        R2[Geração de Resumo]
        B2{fa:fa-server Go Core}

        %% Esteira Cognitiva (Semântica)
        W3[fa:fa-brain Weaver]
        TR3[Extração de Triplas]
        V3[Vetorização Embeddings]
        Q3[(fa:fa-database Qdrant)]
    end

    subgraph L3 [3. Entidades Celestiais]
        direction LR
        SOL((🌞 SOL CENTRAL))
        LUA(🌙 LUAS / PLANETAS)
        AST(☄️ ASTEROIDES)
    end

    subgraph L4 [O UNIVERSO 3D]
        DECK[fa:fa-cubes Deck.gl Visualizer]
    end

    %% ==========================================
    %% 3. FLUXOS E CONEXÕES
    %% ==========================================
    
    %% Conexões do Fluxo Primordial
    T1 --> B1 --> H1 --> SOL

    %% Conexões do Fluxo Físico
    T2 --> C2 --> D2 --> R2 --> B2 --> LUA

    %% Conexões do Fluxo Cognitivo
    T3 --> W3 --> TR3 --> V3 --> Q3 --> AST

    %% Efeito Gravitacional (Renderização)
    SOL -.->|Atração Global| DECK
    LUA -.->|Órbita por Pasta| DECK
    AST -.->|Órbita por Sentido| DECK

    %% ==========================================
    %% 4. APLICAÇÃO SEGURA DE ESTILOS
    %% ==========================================
    class T1,T2,T3 trigger
    class B1,C2,B2 core
    class H1,D2,R2,TR3,V3 action
    class W3 ia
    class Q3 db
    class DECK deck
    class SOL sol
    class LUA lua
    class AST ast
```

---

### B. Visão de Engenharia (Detalhado)
Este fluxo detalha a comunicação entre subpastas, o protocolo de eventos e as barreiras de proteção.

```mermaid
flowchart TD
    %% Estilos (Lumaestro Dark Mode)
    style U fill:#ff9900,stroke:#333,stroke-width:2px,color:#000
    style B fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    style Q fill:#2e7d32,stroke:#6d5dfc,stroke-width:2px,color:#fff
    style F fill:#9c27b0,stroke:#6d5dfc,stroke-width:2px,color:#fff
    style C fill:#455a64,stroke:#6d5dfc,stroke-width:2px,color:#fff
    style W fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff
    style DGL fill:#1e1e1e,stroke:#9c27b0,stroke-dasharray: 5 5,color:#fff

    subgraph Client [Interface Vue 3]
        U((fa:fa-user Usuário))
        F[fa:fa-desktop Gerenciador de Estado]
        DGL[fa:fa-cubes Renderizador Deck.gl]
    end

    subgraph Engine [Lumaestro Core - Go]
        B{fa:fa-server Lumaestro Backend}
        C[fa:fa-spider Crawler de Workspace]
    end

    subgraph Storage [Camada de Dados]
        Q[(fa:fa-database Qdrant / DuckDB)]
        AR{Auto-Reparo}
    end

    %% Fluxo de Infra (Proteção 409)
    B -->|0. EnsureCollections| AR
    AR -->|Check Exists| Q
    Q -->|409 Already Exists| AR
    AR -->|Ignora e Segue| B

    %% Fluxo do Crawler (Estrutural)
    U -->|1. Abre Projeto| B
    B -->|2. Inicia Scan| C
    C -->|3. Identifica .md / Código e Gera Resumo| B
    
    %% Evento Assíncrono (WebSocket/SSE)
    B -.->|4. SafeEmit 'graph:node'| F

    %% Fluxo do Weaver (Cognitivo)
    U -->|5. Envia Prompt no Chat| W[fa:fa-brain Weaver]
    W -->|6. Extrai Triplas Semânticas| W
    W -->|7. Vetorização de Fatos| Q
    Q -->|8. Confirma Persistência| W
    
    %% Evento Assíncrono (WebSocket/SSE)
    W -.->|9. SafeEmit 'graph:node' tipo Asteroid| F

    %% Efeito Visual
    F -->|10. Injeta nós e arestas| DGL
    DGL -->|11. Aplica Física e Gravidade| DGL
```

---

## 🛡️ Componentes Técnicos

### Proteção de Infraestrutura (Auto-Reparo)

Para evitar falhas fatais durante o boot (especialmente em ambientes de alta concorrência ou reloads de HMR), o Lumaestro utiliza uma política de **Idempotência de Coleção**.

> [!IMPORTANT]
> Se o motor tentar criar a coleção `obsidian_knowledge` e o Qdrant retornar um erro `409 (Conflict)`, o sistema identifica isso como um sinal de que a infraestrutura já está pronta e ignora o erro, permitindo que o scan continue sem interrupções.

```go
if err := c.Qdrant.CreateCollection(name, dimension); err != nil {
    // 🛡️ Idempotência: Se outra goroutine já criou a coleção (409), ignorar
    if strings.Contains(err.Error(), "already exists") {
        fmt.Printf("[Crawler] ✅ Coleção '%s' pronta (via Auto-Reparo).\n", name)
        continue
    }
    return fmt.Errorf("falha ao criar coleção: %w", err)
}
```
### Interface de Emissão (Sinapse Backend → Frontend)
Todos os nós, independente da origem, devem respeitar o contrato de emissão `SafeEmit` para garantir que o Frontend não sofra com race conditions.

```go
// Exemplo de nascimento de um nó via Crawler
utils.SafeEmit(c.ctx, "graph:node", map[string]interface{}{
    "id":             nodeID,
    "name":           nodeName,
    "document-type":  "chunk",
    "celestial-type": "moon", // Arquivos são luas orbitando pastas
    "mass":           5.0,
    "summary":        fileSummary,
})
```

### Vetorização de Memórias (Weaver)
Quando o `KnowledgeWeaver` identifica um novo fato, ele o integra ao Cérebro Vetorial (Qdrant).

```js
// Lógica de Tecelagem (Simulação)
const fact = "Lumaestro usa Go no Backend";
const vector = await embedder.generate(fact);
qdrant.upsert("knowledge_graph", { id: hash(fact), vector, payload: { subject: "Lumaestro", ... } });
```

---

## 🐹 Dicas para o Comandante

> [!TIP]
> **Massa Gravitacional:** Pastas raiz têm massa `50.0` (Sistemas Solares), enquanto arquivos individuais têm massa `5.0`. Se o seu grafo estiver muito disperso, aumente a força de repulsão (F6) para o Modo Supernova.

> [!IMPORTANT]
> **Regra de Ouro:** O sistema nunca cria o mesmo nó duas vezes. O ID é sempre derivado de um hash SHA-256 do caminho absoluto do arquivo ou do conteúdo do fato semântico. Isso evita a "Esquizofrenia de Dados".

---

## 🔗 Documentos Relacionados

- [[APP_BOOT]]: O despertar dos serviços.
- [[RAG_ARCHITECTURE]]: Como o cérebro processa a matéria criada.
- [[DOCS_INDEX]]: Índice estelar da documentação.
