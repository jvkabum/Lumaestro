---
title: "Protocolo de Identidade Linguística (Gemini)"
type: "guide"
status: "active"
tags: ["gemini", "language", "portuguese", "system-prompt"]
---

# ♊ Protocolo de Identidade Linguística (Gemini)

> [!ABSTRACT]
> O Lumaestro possui uma identidade brasileira inegociável. Este documento detalha o protocolo de comunicação que garante que todas as interações, raciocínios e saídas de dados ocorram em **Português do Brasil**, preservando a naturalidade técnica e a precisão cultural.

## 🛡️ Filtro de Soberania Linguística

O sistema intercepta qualquer tentativa de comunicação em outros idiomas e normaliza o fluxo para o padrão nativo brasileiro.

```mermaid
flowchart TD
    %% Estilos
    classDef trigger fill:#1e1e1e,stroke:#888,stroke-width:2px,stroke-dasharray: 5 5,color:#fff
    classDef core fill:#2d333b,stroke:#6d5dfc,stroke-width:2px,color:#fff
    classDef ia fill:#6d5dfc,stroke:#fff,stroke-width:2px,color:#fff

    subgraph Input [Entrada de Dados]
        U[fa:fa-comment Prompt do Usuário]
    end

    subgraph Filter [Filtro de Identidade]
        direction TB
        D{fa:fa-language Detecção de Idioma}
        T[fa:fa-exchange-alt Conversão PT-BR]
    end

    subgraph Response [Córtex de Resposta]
        AI[fa:fa-robot Gemini Engine]
        OUT[fa:fa-check-circle Output Nativo BR]
    end

    %% Fluxo
    U --> D
    D -- "Idiomas Estrangeiros" --> T
    D -- "Português" --> AI
    T --> AI
    AI --> OUT

    %% Estilos
    class U trigger
    class D,T core
    class AI,OUT ia
```

---

## 📜 Regras de Comunicação Imutáveis

1.  **Soberania PT-BR**: O agente deve falar, pensar e formular raciocínios exclusivamente em português do Brasil.
2.  **Conversão Automática**: Se o usuário escrever em outro idioma, o sistema deve converter para PT-BR sem aviso prévio.
3.  **Naturalidade Técnica**: Termos universais (ex: *build*, *deploy*, *API*, *endpoint*) são preservados, mas explicados com a fluidez de um engenheiro brasileiro nativo.
4.  **Clareza e Objetividade**: Estilo comunicativo direto, claro e educativo, evitando formalidades excessivas que prejudiquem a agilidade técnica.

---

## 🛠️ Implementação no Core

- **System Instruction**: Estas regras são injetadas no parâmetro `SystemInstruction` durante a inicialização do `GeminiClient` no `internal/provider/gemini.go`.
- **Enforcement**: O validador de saída descarta qualquer stream de token que não atenda aos critérios de idioma configurados.

---

## 🔗 Documentos Relacionados

- [[MODEL_PROVIDER_MATRIX]] — Como este protocolo se aplica a outros provedores (Claude/LM Studio).
- [[AGENTS_GUIDE]] — O tom de voz dos agentes durante a execução de tarefas.
- [[DOCS_INDEX]] — Índice central de documentação.

---
**Lumaestro: Inteligência Global. Identidade Nacional. ♊🤖🇧🇷**