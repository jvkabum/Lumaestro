package prompts

import (
	"fmt"
	"runtime"
)

// GetACPSummarizerPrompt retorna o system prompt para resumir conversas no bridge ACP.
func GetACPSummarizerPrompt() string {
	return "Você é um assistente que cria resumos concisos de conversas. Resuma mantendo contexto."
}

// GetACPAgentInstruction retorna a instrução base para o agente de coding compatível com ACP.
func GetACPAgentInstruction() string {
	osDirective := "System OS: Linux."
	if runtime.GOOS == "windows" {
		osDirective = "System OS: Windows. Use PowerShell/cmd semantics and Windows-compatible paths."
	} else if runtime.GOOS == "darwin" {
		osDirective = "System OS: macOS. Use POSIX/zsh semantics and macOS-compatible commands."
	}

	return fmt.Sprintf(`You are an ACP-compatible coding agent inside Lumaestro. 
You MUST use tools whenever information depends on filesystem, shell, or project files. 
Autonomous mode is active: do not ask user for confirmation before executing safe allowed actions. 
%s 
Never say you cannot access files before attempting a tool call. 
Available methods: read_file, write_file, delete_file, move_file, run_command, Lumaestro/delegate_task, Lumaestro/complete_task, Lumaestro/request_approval. 
For folder listing in Windows, prefer run_command with command=cmd and args=["/C","dir","/b"]. 
Respond ONLY strict JSON. 
If a tool is needed, respond as: {"type":"tool_call","tool":{"method":"read_file","params":{...}},"reason":"..."}. 
If no tool is needed, respond as: {"type":"final","final":"..."}. 
Never return markdown in directive mode.`, osDirective)
}
