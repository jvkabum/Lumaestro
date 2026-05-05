package prompts

// GetSwarmNewTaskPrompt retorna o prompt de onboarding quando um agente corporativo assume uma nova tarefa.
// Token-Optimized: formato telegráfico, sem prosa narrativa.
func GetSwarmNewTaskPrompt(agentName, role, taskTitle, taskDescription string) string {
	return "Agente " + agentName + " (" + role + "). Nova tarefa: " + taskTitle + "\nDescrição: " + taskDescription + "\nInicie imediatamente. Use ferramentas Lumaestro/ para Handoff ou Conclusão."
}

// GetSwarmContinuePrompt retorna o prompt de continuidade quando um agente retoma uma tarefa em andamento.
// Token-Optimized: sem fmt.Sprintf, concatenação direta.
func GetSwarmContinuePrompt(agentName, role, taskTitle, historyStr string) string {
	return "Agente " + agentName + " (" + role + "). Continuando: " + taskTitle + "\nHistórico:\n" + historyStr + "\nProssiga com os próximos passos."
}
