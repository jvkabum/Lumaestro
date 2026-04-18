---
tags: [ai, fine-tuning, rlhf, lightning, training]
type: guide
status: active
---

# 🧠 Manual de Elite: Fine-Tuning & RLHF (Lightning Engine)

O sistema de **Fine-Tuning** do Lumaestro não é apenas uma exportação de dados; é um processo de **Curadoria de Inteligência** baseado em **RLHF (Reinforcement Learning from Human Feedback)**. Este documento detalha o fluxo técnico desde a interação do usuário até a geração do dataset de treino.

## 🏗️ 1. O Ciclo de Vida do Dado de Treino

No Lumaestro, os dados para ajuste fino são gerados organicamente através do uso do sistema. Cada interação passa por um filtro de qualidade antes de ser considerada uma "Amostra de Ouro" (Gold Sample).

`mermaid
sequenceDiagram
    participant U as Usuário
    participant A as Agente
    participant ACP as Portão ACP
    participant DB as DuckDB (Lightning)
    participant EXP as Exportador Dataset

    U->>A: Solicita Tarefa Complexa
    A->>ACP: Propõe Ação (Tool Call)
    ACP->>U: Solicita Aprovação
    U->>ACP: Aprova Ação
    ACP->>DB: Registra Reward (+1.0) & Gold Sample
    Note over DB: O dado é marcado como 'Curado'
    EXP->>DB: Filtra Amostras com Reward > 0.8
    EXP->>U: Gera dataset_lumaestro_rlhf.jsonl
`

---

## 🛠️ 2. Arquitetura Interna (Lightning Core)

O motor que sustenta o Fine-Tuning reside em internal/lightning/.

### Recompensas de Treinamento (eward_engine.go)
Diferente de logs comuns, o sistema utiliza o RewardEngine para atribuir "Dopamina Digital". Somente interações com alta recompensa são elegíveis para o Fine-Tuning.

> [!TIP]
> O threshold padrão para elegibilidade de treino é **0.8**. Interações abaixo disso são consideradas "em aprendizado" e não devem poluir o dataset de ajuste fino.

### Otimização de Prompts (optimization.go)
Antes de um treino de pesos (SFT), o sistema realiza um **"Fine-Tuning de Instruções"**. O otimizador busca falhas no DuckDB e refina os prompts dos agentes para evitar reincidência de erros.

---

## 📦 3. Geração e Exportação do Dataset

Ao utilizar o comando de exportação no Dashboard, o Lumaestro executa uma query OLAP no DuckDB para consolidar as conversas aprovadas.

### Formato Conversacional (SFT):
O arquivo dataset_lumaestro_rlhf.jsonl segue o padrão messages da OpenAI/Google:

`json
{
  "messages": [
    {"role": "system", "content": "Você é o Maestro Coder..."},
    {"role": "user", "content": "Corrija o bug no main.go"},
    {"role": "assistant", "content": "Entendido. Analisando o código...", "weight": 1},
    {"role": "assistant", "content": "<thought>...</thought>Arquivo corrigido.", "weight": 1}
  ]
}
`

---

## 🚀 4. Treinando Modelos Externos

Com o dataset em mãos, você pode herdar a inteligência do Lumaestro em modelos menores (SLMs) ou modelos de fronteira:

1.  **Unsloth/Axolotl**: Ideal para modelos como **Llama-3-8B** ou **Mistral**.
2.  **Google Vertex AI**: Utilize para fazer o fine-tuning do **Gemini 1.5 Flash**, tornando-o um especialista no seu codebase.
3.  **OpenAI Fine-Tuning**: Compatível com o formato .jsonl gerado.

`mermaid
graph LR
    L[Lumaestro Data] -->|Export| DS[Dataset .jsonl]
    DS -->|SFT Training| M1[Llama-3 Fine-tuned]
    DS -->|Distillation| M2[Gemini 1.5 Flash FT]
    
    style L fill:#2d333b,stroke:#6d5dfc,color:#e6edf3
    style DS fill:#1e1e2e,stroke:#313244,color:#cdd6f4
`

---

## 🔗 Veja Também
- [[LIGHTNING_REINFORCEMENT_LEARNING]]: Detalhes sobre o motor de recompensas.
- [[ACP_MODE]]: Como o feedback humano é coletado.
- [[DATABASE_SCHEMA]]: Estrutura das tabelas de telemetria no DuckDB.

> [!IMPORTANT]
> O Fine-tuning é a fase final da evolução do seu sistema. Use-o para reduzir custos e latência, transferindo o conhecimento dos modelos maiores para modelos locais especializados.
