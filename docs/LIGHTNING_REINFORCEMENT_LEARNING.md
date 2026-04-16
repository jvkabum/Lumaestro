# Lumaestro Lightning: Reinforcement Learning & Machine Learning

Documentação técnica sobre a integração de modelos de aprendizado por reforço para otimização da topologia do Grafo Neural.

## 🧠 Arquitetura de Feedback Operacional

O Lumaestro utiliza um motor de ML baseado em Transformers e Q-Learning para ajustar dinamicamente as forças físicas do grafo (Gravity, Bounce, Straighten) baseando-se na interação do usuário.

### Modelos Ativos:
1. **Nexus-Orchestrator (Gemma 4 8B):** Decide a hierarquia semântica inicial.
2. **Q-Graph Opt (Custom):** Ajusta pesos de arestas baseando-se na frequência de travessia e relevância RAG.

## 🛰️ Sincronização de Pesos Dinâmicos

As arestas do grafo não são estáticas. Cada `graph:edge` recebido via Wails contribui para o "calor" sináptico do nó, resultando em:
- **Nós de Alta Janela:** Se tornam clusters gravitacionais.
- **Micro-Nós:** Orbitam documentos complexos para fornecer granularidade.

---
*Gerado via Lumaestro Agent Engine.*
