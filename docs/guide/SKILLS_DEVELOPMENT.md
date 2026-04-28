---
title: "Guia de Desenvolvimento de Skills (Habilidades)"
type: "guide"
status: "active"
tags: ["skills", "development", "tools", "acp", "agent-capabilities"]
---

# 🛠️ Desenvolvimento de Skills: Expandindo o Arsenal do Enxame

> [!ABSTRACT]
> As **Skills** são os braços e pernas do Lumaestro. Elas permitem que os agentes interajam com o mundo físico — executando comandos, lendo arquivos e integrando-se a APIs externas. Este guia detalha o processo de injeção de novas capacidades no enxame.

## 🏗️ Ciclo de Injeção e Execução de Habilidades

O nascimento de uma Skill segue um fluxo rigoroso desde a codificação até a descoberta semântica pela IA.

```mermaid
flowchart TD
    %% Estilos
    classDef step fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef registry fill:#ffcc00,stroke:#333,stroke-width:2px,color:#000
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff

    subgraph Development [Fase de Criação]
        S1[fa:fa-code Lógica em Go: internal/skills]
        S2[fa:fa-file-signature Definição JSON Schema]
    end

    subgraph Knowledge [Fase de Registro]
        R1[fa:fa-book Skillbook: Qdrant/DuckDB]
        R2{fa:fa-brain LLM Discovery}
    end

    subgraph Execution [Fase de Operação]
        E1[fa:fa-robot Agente detecta necessidade]
        E2[fa:fa-bolt Execução via ACP]
    end

    %% Fluxo
    S1 & S2 --> R1
    R1 --> R2
    R2 --> E1
    E1 --> E2

    %% Estilos
    class S1,S2,E2 step
    class R1,R2 registry
    class E1 ia
```

---

## 🧩 Anatomia de uma Skill de Elite

Uma habilidade no Lumaestro é composta por dois pilares fundamentais:

1.  **Definição Semântica (Skillbook)**: Uma descrição clara (em linguagem natural) do "Porquê" e "Quando" usar a ferramenta. Isso é armazenado vetorialmente para que a IA possa "puxar" a habilidade correta no momento da decisão.
2.  **Protocolo de Execução (ACP)**: A implementação lógica real. O agente se comunica com a Skill enviando um payload JSON estruturado, garantindo previsibilidade e segurança.

---

## 🚀 Categorias de Poder

### 🔋 Native Skills
Embutidas diretamente no núcleo Go. Exemplos: `FileRead`, `FileWrite`, `CrawlerExecute`, `SearchWeb`. São ultra-rápidas e seguras.

### 🔌 External Skills
Scripts ou executáveis (Python, Node, Bash) que o enxame pode invocar via Shell interativo. Permitem uma expansão infinita das capacidades sem inchar o binário central.

### 🧠 Learned Skills (Estratégias)
Estratégias de sucesso que o **Agente Reflector** destila e salva após concluir uma tarefa complexa. Elas funcionam como "memórias procedurais" para tarefas futuras semelhantes.

---

## 🛡️ Diretrizes de Soberania (Best Practices)

- **Atomicidade**: Uma skill deve realizar uma única ação com precisão cirúrgica.
- **Validação de Path**: Nunca permita que uma skill de arquivo opere fora do workspace definido pelo Comandante.
- **Feedback Verboso**: Em caso de falha, retorne o erro técnico detalhado. O LLM usará essa informação para tentar uma correção (Self-Healing).

---

## 🔗 Documentos Relacionados

- [[SKILLS_SYSTEM]] — Visão técnica do orquestrador de habilidades.
- [[AGENTS_GUIDE]] — Como delegar tarefas que exigem habilidades específicas.
- [[DEVELOPER_GUIDE]] — Setup para começar a codar novas Skills.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Onde a inteligência encontra a ação. 🛠️🦾⚡**
