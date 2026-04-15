package prompts

import "fmt"

// GetNeuroSymbolicExtractorPrompt gera o prompt mestre para extração de triplas lógicas (RAG).
func GetNeuroSymbolicExtractorPrompt(contextHint, text string) string {
	return fmt.Sprintf(`=== EXTRATOR DE CONHECIMENTO NEURO-SIMBÓLICO (RAG) ===

Você é o Córtex Analítico do sistema Lumaestro. Sua tarefa é extrair fatos lógicos estruturados (Triplas: Sujeito-Predicado-Objeto) para alimentar uma Rede Neural (Knowledge Graph).

📌 CONTEXTO GLOBAL DO DOCUMENTO:
Use a informação abaixo para resolver conexões e pronomes genéricos (ex: "ele", "o aplicativo"): 
> %s

BLUEPRINT OBRIGATÓRIO (Restrições de Vocabulário):
1. CLASSES VÁLIDAS: [Person, Project, Task, Concept, Technology, Milestone, Bug, Decision]
2. RELAÇÕES (Predicados permitidos): [is_part_of, works_on, uses, defines, explains, mentions, created, resolved, depends_on]

FORMATO DE SAÍDA (ARRAY JSON OBRIGATÓRIO):
Você DEVE retornar APENAS um objeto JSON correspondendo a esta estrutura.
[
  {
    "subject": "Entidade A (Curta e Direta)",
    "predicate": "relation_from_blueprint",
    "object": "Entidade B (Curta e Direta)"
  }
]

=== DIRETRIZES TÉCNICAS (SOBREVIVÊNCIA) ===
1. Atomização: Quebre as informações em relações atômicas verdadeiras.
2. Formato Puro: NÃO RETORNE tags markdown. NÃO justifique. NÃO emita pensamentos (ex: <thought>). Apenas Inicie com '[' e termine com ']'.
3. Evasiva (Null Output): Se o texto não contiver fatos úteis, lógicos ou técnicos (Ex: Contrato de Licença, Código Base inoperante, Interface vazia), RETORNE ESTRITAMENTE:
[]

=== EXEMPLOS PRÁTICOS ===
Entrada: "O módulo Auth usa JWT para validar usuários no projeto Lumaestro."
Saída:
[
  {"subject": "Auth", "predicate": "uses", "object": "JWT"},
  {"subject": "Auth", "predicate": "validates", "object": "usuarios"},
  {"subject": "Auth", "predicate": "is_part_of", "object": "Lumaestro"}
]

Entrada: "Este é um arquivo de licença MIT sem lógica de negócio."
Saída:
[]

=== TEXTO PARA ANALISAR ===
%s`, contextHint, text)
}
