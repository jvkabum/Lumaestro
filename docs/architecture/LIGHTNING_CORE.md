# ⚡ Lumaestro Architecture: Lightning Core (APO & Regression) 🐹⚙️⚡

Este documento detalha o motor de inteligência e telemetria industrial que transforma o Lumaestro em um sistema de evolução autônoma.

---

## 🏗️ 1. O Loop de Inteligência Assistida (APO)

O Lumaestro utiliza um motor de **Automatic Prompt Optimization (APO)** inspirado em arquiteturas de 18k linhas, permitindo que o enxame aprenda com cada falha.

`mermaid
graph TD
    %% Estilo Dark Mode
    classDef default fill:#2d333b,stroke:#6d5dfc,color:#e6edf3;
    
    A[Execução do Agente] -->|Log de Telemetria| B[DuckDB Spans]
    B -->|Avaliação de Recompensa| C{Recompensa < 0.3?}
    C -->|Falha Crítica| D[Motor APO Beam Search]
    C -->|Sucesso| E[Operação Normal]
    D -->|Reflexão Metacognitiva| F[Gerador de 3 Variantes]
    F -->|Teste de Regressão| G[Validação Gold Samples]
    G -->|Precisão Estimada| H[Córtex de Decisão dashboard]
    H -->|Aprovação do Comandante| I[Evolução do System Prompt]
    I --> A
`

---

## 🗄️ 2. Camada Analítica Colunar (DuckDB)

Diferente de logs JSON comuns, o Lumaestro usa o **DuckDB (OLAP)** para processar "Consciência Colunar". Isso permite análises de alta performance em tempo real:

*   **Tabela spans**: Rastreabilidade total (OTEL compatible) de cada rastro de pensamento.
*   **Tabela ewards**: "Dopamina Digital" mapeando o sucesso do enxame.
*   **Tabela gold_samples**: O repositório de "Verdade Absoluta" usado para evitar regressões intelectuais.

---

## 🧪 3. Motor de Regressão Gold

Toda nova inteligência proposta pelo enxame deve provar seu valor antes de ser apresentada. O motor de regressão executa cada variante contra todas as referências históricas de sucesso.

**Métrica de Estabilidade**:
*   Accuracy = (Hits / Total Gold Samples) * 100

---
**Lumaestro Architecture: Performance industrial, Autonomia absoluta.** 🐹⚙️⚡🤖💰🏁👁️📂🧪
