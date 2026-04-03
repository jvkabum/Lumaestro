# 📂 Manual de Elite: Fine-Tuning do Enxame (RLHF) 🐹⚙️⚡

Este guia detalha como utilizar os datasets gerados pelo Lumaestro para treinar modelos de IA especializados que herdam a inteligência curada pelo seu enxame.

---

## 🏗️ 1. Geração do Dataset
No Dashboard do Lumaestro, utilize o botão **"📦 Gerar Dataset RLHF"**. Isso criará o arquivo `dataset_lumaestro_rlhf.jsonl` no diretório raiz do projeto.

### Formato Conversacional:
```json
{
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ]
}
```

---

## 🛠️ 2. Ferramentas Recomendadas

Para treinar modelos Open Source (Llama-3, Mistral, Qwen) com os dados do Lumaestro:
*   [Unsloth](https://github.com/unslothai/unsloth): Para treinamento ultra-rápido em GPUs de consumidor.
*   [Axolotl](https://github.com/OpenAccess-AI-Collective/axolotl): Para configurações avançadas de SFT/DPO.

Para treinar na Cloud:
*   **Google Vertex AI**: Faça o upload do `.jsonl` e realize o Fine-tuning do **Gemini 1.5 Flash**.

---

## 🧠 3. Estratégia de Treinamento (SFT)
Os dados do Lumaestro representam o **"Caminho Perfeito"** (Gold Samples + Candidatos Aprovados). Ao realizar o Supervised Fine-Tuning (SFT):
1.  O modelo aprende o **estilo de resposta** aprovado pelo Comandante.
2.  O modelo herda a **ontologia de extração** do Graph-RAG.
3.  A latência é reduzida, pois o modelo menor se torna um "especialista" na tarefa do agente.

---
**Lumaestro: O futuro da inteligência é treinado por quem executa.** 🐹⚙️⚡🤖💰🏁👁️📂🧪
