package prompts

import "fmt"

// GetSwarmNewTaskPrompt retorna o prompt de onboarding quando um agente corporativo assume uma nova tarefa.
func GetSwarmNewTaskPrompt(agentName, role, taskTitle, taskDescription string) string {
	return fmt.Sprintf("Você é o agente corporativo %s (Cargo: %s). Você acaba de assumir a tarefa: %s\nDescrição: %s\nInicie o trabalho imediatamente. Use as ferramentas de 'Lumaestro/' para Handoff ou Conclusão se necessário.", agentName, role, taskTitle, taskDescription)
}

// GetSwarmContinuePrompt retorna o prompt de continuidade quando um agente retoma uma tarefa em andamento.
func GetSwarmContinuePrompt(agentName, role, taskTitle, historyStr string) string {
	return fmt.Sprintf("Você é o agente corporativo %s (Cargo: %s). Continuando tarefa: %s\nHistórico recente:\n%s\nPor favor, prossiga com os próximos passos.", agentName, role, taskTitle, historyStr)
}
