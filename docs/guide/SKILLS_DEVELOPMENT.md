# 🛠️ Guia de Desenvolvimento de Skills (Habilidades) 🐹🦾

As habilidades (Skills) são as ferramentas que permitem aos agentes do Lumaestro interagir com o mundo real (rodar comandos, ler arquivos, acessar APIs).

## 🧩 Anatomia de uma Skill

Uma skill no Lumaestro é composta por:
1.  **Definição no Skillbook**: Descrição semântica para que o LLM saiba QUANDO usar.
2.  **Implementação em Go**: A lógica real que será executada.

## 🚀 Passo a Passo: Criando uma Nova Skill

### 1. Registrar no Skillbook
As habilidades são armazenadas vetorialmente no Qdrant na coleção ce_skills.
`go
// Exemplo de como o sistema salva uma nova estratégia
skillbook.SaveSkill(ctx, "Usar esta ferramenta para analisar logs de erro do sistema.")
`

### 2. Definir o Schema JSON (Tool Use)
O Lumaestro usa o protocolo ACP (Agent Communication Protocol). Sua skill deve aceitar parâmetros via JSON:
`json
{
  "name": "exec_command",
  "arguments": {
    "command": "npm test",
    "dir": "./frontend"
  }
}
`

## 📚 Categorias de Skills
- **Native Skills**: Embutidas no executável Go (Ex: Leitura de arquivos, Crawling).
- **External Skills**: Scripts ou executáveis externos chamados via ACP.
- **Learned Skills**: Estratégias que o Agente Reflector aprende e salva no Skillbook após uma tarefa bem-sucedida.

## 📝 Dicas de Ouro
- **Atomicidade**: Uma skill deve fazer apenas uma coisa bem feita.
- **Segurança**: Sempre valide os caminhos de diretório para evitar que a IA acesse pastas sensíveis (como .git ou .env).
- **Feedback**: A skill deve retornar um output claro; se falhar, o erro deve ser descritivo para que o LLM possa tentar corrigir o comando.

---
[[INDEX|⬅️ Voltar ao Índice]] | [[DEVELOPER_GUIDE|Próximo: Guia do Desenvolvedor ➡️]]
