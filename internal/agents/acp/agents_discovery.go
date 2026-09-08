package acp

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// CustomAgent representa uma definicao de agente personalizado conforme a documentacao do Antigravity CLI.
type CustomAgent struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	Scope       string `json:"scope"` // "workspace" ou "global"
	Path        string `json:"path"`
}

// DiscoverCustomAgents escaneia agentes definidos no workspace ({workspace}/.agents/agents/) e globalmente (~/.gemini/config/agents/).
func DiscoverCustomAgents(workspace string) ([]CustomAgent, error) {
	var agents []CustomAgent
	seen := make(map[string]bool)

	// 1. Escopo de Workspace ({workspace}/.agents/agents/{name}/agent.md)
	if workspace != "" && workspace != "." {
		wsAgentsDir := filepath.Join(workspace, ".agents", "agents")
		if list, err := scanAgentsDir(wsAgentsDir, "workspace"); err == nil {
			for _, a := range list {
				if !seen[a.Name] {
					seen[a.Name] = true
					agents = append(agents, a)
				}
			}
		}
	}

	// 2. Escopo Global (~/.gemini/config/agents/{name}/agent.md)
	userHome, errHome := os.UserHomeDir()
	if errHome == nil {
		globalAgentsDir := filepath.Join(userHome, ".gemini", "config", "agents")
		if list, err := scanAgentsDir(globalAgentsDir, "global"); err == nil {
			for _, a := range list {
				if !seen[a.Name] {
					seen[a.Name] = true
					agents = append(agents, a)
				}
			}
		}
	}

	return agents, nil
}

func scanAgentsDir(baseDir string, scope string) ([]CustomAgent, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var result []CustomAgent
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentFile := filepath.Join(baseDir, entry.Name(), "agent.md")
		if info, errStat := os.Stat(agentFile); errStat == nil && !info.IsDir() {
			if agent, errParse := parseAgentMarkdown(agentFile, entry.Name(), scope); errParse == nil {
				result = append(result, agent)
			}
		}
	}
	return result, nil
}

func parseAgentMarkdown(filePath string, folderName string, scope string) (CustomAgent, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return CustomAgent{}, err
	}

	content := string(data)
	agent := CustomAgent{
		Name:        folderName,
		Description: "",
		Prompt:      "",
		Scope:       scope,
		Path:        filePath,
	}

	// Parse simples e robusto de Frontmatter YAML (delimitado por ---)
	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			agent.Prompt = strings.TrimSpace(parts[2])

			scanner := bufio.NewScanner(strings.NewReader(frontmatter))
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "name:") {
					val := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					if val != "" {
						agent.Name = val
					}
				} else if strings.HasPrefix(line, "description:") {
					agent.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				}
			}
		} else {
			agent.Prompt = content
		}
	} else {
		agent.Prompt = content
	}

	return agent, nil
}
