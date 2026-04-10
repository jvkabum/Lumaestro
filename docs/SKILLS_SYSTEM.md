---
tags:
  - architecture
  - agents
  - skills
  - development
---

# 🛠️ Sistema de Skills do Lumaestro

> [!ABSTRACT] Visão Geral
> No Lumaestro, uma **Skill** (Habilidade) não é apenas um comando, mas um **Injetor de Especialização**. Elas fornecem o "Know-how" técnico, comportamental e arquitetural para os agentes, transformando uma IA genérica em um especialista (ex: Go Expert, Vue Architect, Doc-Master).

---

## 🏗️ Arquitetura das Skills

O sistema é dividido em duas camadas: **Skills Estáticas** (Arsenal Nativo) e **Skills Dinâmicas** (Skillbook/Qdrant).

### 1. Arsenal Nativo (Static Skills)
São centenas de habilidades pré-definidas em Go, organizadas por categorias no diretório `internal/agents/skills/`.

*   **Registro Automático:** Cada skill reside em seu próprio pacote e usa a função `init()` para se registrar no `manager.go`.
*   **Estrutura de Código (`internal/agents/skills/manager.go`):**
    `go
    type Skill struct {
        Name        string
        Category    string
        Content     string // O "Prompt" da habilidade em Markdown
        Description string
    }
    `

### 2. Skillbook (Dynamic Strategies)
Localizado em `internal/agents/skillbook.go`, este componente usa o **Qdrant** para armazenar e recuperar estratégias de aprendizado baseadas em similaridade vetorial.

---

## 🔄 Fluxo de Vida de uma Skill

O ciclo de vida vai desde o registro no backend até a injeção no System Prompt do agente.

`mermaid
graph TD
    %% Estilo Dark Mode
    classDef default fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;

    A[Inicialização do App] --> B{init() em cada Skill}
    B --> C[skills.Register]
    C --> D[Registry Global em manager.go]
    
    User[👤 Usuário] -->|Solicita Agente @golang-pro| E[Executor]
    E --> F[skills.GetSkill]
    F --> G[Injeção no System Prompt]
    G --> H[🤖 Agente Especialista]

    subgraph "Camada de Persistência"
        D
    end
`

---

## 🧩 Anatomia de uma Skill (`skill.go`)

Cada skill segue um padrão rigoroso de documentação interna. Veja o exemplo da `golang-pro` em `internal/agents/skills/development/golang_pro/skill.go`:

| Seção | Propósito |
| :--- | :--- |
| **Frontmatter (YAML)** | Metadados como `name`, `risk` e `source`. |
| **Use this skill when** | Gatilhos de contexto para o agente. |
| **Instructions** | Passo a passo técnico para a execução da tarefa. |
| **Capabilities** | Lista exaustiva de conhecimentos técnicos (ex: Concurrency, GORM). |
| **Behavioral Traits** | Como o agente deve se comportar (ex: "Prefere simplicidade sobre esperteza"). |

---

## 🧠 Integração com o Skillbook (RAG)

O `Skillbook` permite que o Lumaestro "aprenda" novas formas de resolver problemas e as recupere via busca semântica.

`mermaid
sequenceDiagram
    participant A as Agente
    participant SB as Skillbook (Go)
    participant Q as Qdrant (Vector DB)

    A->>SB: SaveSkill(description)
    SB->>SB: GenerateEmbedding()
    SB->>Q: UpsertPoint("ace_skills")
    
    Note over A, Q: Futuramente...
    
    A->>SB: RetrieveRelevantSkills(query)
    SB->>Q: Search Similarity
    Q-->>A: Retorna Estratégias Relevantes
`

---

## 🛠️ Como Criar uma Nova Skill

Para adicionar um novo especialista ao enxame, siga estes passos:

1.  **Crie a Pasta:** `internal/agents/skills/[categoria]/[nome_da_skill]/`.
2.  **Crie o Arquivo:** `skill.go`.
3.  **Implemente o `init()`:**
    `go
    func init() {
        skills.Register(skills.Skill{
            Name: "minha-skill",
            Category: "general",
            Content: "--- prompt aqui ---",
        })
    }
    `
4.  **Importe o pacote:** Certifique-se de que o pacote da skill seja importado (direta ou indiretamente) para que o `init()` seja executado.

---

## 🔗 Documentos Relacionados
- [[AGENTS_GUIDE]]: Como o Executor utiliza as skills.
- [[NEURAL_BRAIN]]: Como o Qdrant gerencia a memória rosa e as skills dinâmicas.
- [[LIGHTNING_ENGINE]]: O motor que avalia a eficácia das skills aplicadas.

---
**Lumaestro: Da Habilidade à Maestria. 🐹⚙️⚡🕸️🧠🏎️🤖💰🏁🛡️🧪**
