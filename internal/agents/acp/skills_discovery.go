package acp

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// SkillItem representa uma habilidade descoberta segundo o padrão do Antigravity CLI.
type SkillItem struct {
	Name        string `json:"name"`
	Command     string `json:"command"` // e.g. "/accidental-data-loss-prevention"
	Description string `json:"description"`
	Scope       string `json:"scope"` // "workspace" ou "global"
	Path        string `json:"path"`
	Content     string `json:"content"`
}

// DiscoverSkills escaneia diretórios de skills no workspace e globalmente.
func DiscoverSkills(workspace string) ([]SkillItem, error) {
	var skills []SkillItem
	seen := make(map[string]bool)

	// 1. Escopo de Workspace:
	// - {workspace}/.agents/skills/
	// - {workspace}/.gemini/skills/
	if workspace != "" && workspace != "." {
		wsDirs := []string{
			filepath.Join(workspace, ".agents", "skills"),
			filepath.Join(workspace, ".gemini", "skills"),
		}
		for _, dir := range wsDirs {
			if list, err := scanSkillsDir(dir, "workspace"); err == nil {
				for _, s := range list {
					if !seen[s.Name] {
						seen[s.Name] = true
						skills = append(skills, s)
					}
				}
			}
		}
	}

	// 2. Escopo Global:
	// - ~/.gemini/config/skills/
	// - ~/.gemini/antigravity/builtin/skills/
	// - ~/.gemini/antigravity-cli/skills/
	userHome, errHome := os.UserHomeDir()
	if errHome == nil {
		globalDirs := []string{
			filepath.Join(userHome, ".gemini", "config", "skills"),
			filepath.Join(userHome, ".gemini", "antigravity", "builtin", "skills"),
			filepath.Join(userHome, ".gemini", "antigravity-cli", "skills"),
		}
		for _, dir := range globalDirs {
			if list, err := scanSkillsDir(dir, "global"); err == nil {
				for _, s := range list {
					if !seen[s.Name] {
						seen[s.Name] = true
						skills = append(skills, s)
					}
				}
			}
		}
	}

	return skills, nil
}

func scanSkillsDir(baseDir string, scope string) ([]SkillItem, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var result []SkillItem
	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(baseDir, name)

		if entry.IsDir() {
			// Procura por SKILL.md ou skill.md dentro do subdiretório
			candidates := []string{
				filepath.Join(fullPath, "SKILL.md"),
				filepath.Join(fullPath, "skill.md"),
			}
			for _, candidate := range candidates {
				if info, errStat := os.Stat(candidate); errStat == nil && !info.IsDir() {
					if item, errParse := parseSkillMarkdown(candidate, name, scope); errParse == nil {
						result = append(result, item)
						break
					}
				}
			}
		} else if strings.HasSuffix(strings.ToLower(name), ".md") {
			baseName := strings.TrimSuffix(name, filepath.Ext(name))
			if item, errParse := parseSkillMarkdown(fullPath, baseName, scope); errParse == nil {
				result = append(result, item)
			}
		}
	}

	return result, nil
}

func parseSkillMarkdown(filePath string, defaultName string, scope string) (SkillItem, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return SkillItem{}, err
	}

	content := string(data)
	item := SkillItem{
		Name:        defaultName,
		Command:     "/" + defaultName,
		Description: "",
		Scope:       scope,
		Path:        filePath,
		Content:     "",
	}

	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "---") {
		parts := strings.SplitN(trimmed, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			item.Content = strings.TrimSpace(parts[2])

			scanner := bufio.NewScanner(strings.NewReader(frontmatter))
			inDescBlock := false
			var descBuilder strings.Builder

			for scanner.Scan() {
				line := scanner.Text()
				trimmedLine := strings.TrimSpace(line)

				if strings.HasPrefix(trimmedLine, "name:") {
					inDescBlock = false
					val := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "name:"))
					val = strings.Trim(val, `"'`)
					if val != "" {
						item.Name = val
						item.Command = "/" + val
					}
				} else if strings.HasPrefix(trimmedLine, "description:") {
					val := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "description:"))
					if val == "|" || val == ">" || val == ">-" || val == "|-" {
						inDescBlock = true
					} else {
						inDescBlock = false
						item.Description = strings.Trim(val, `"'`)
					}
				} else if inDescBlock {
					// Linhas indentadas pertencem ao bloco de descrição
					if strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t") {
						if descBuilder.Len() > 0 {
							descBuilder.WriteString(" ")
						}
						descBuilder.WriteString(strings.TrimSpace(line))
					} else {
						inDescBlock = false
					}
				}
			}

			if descBuilder.Len() > 0 && item.Description == "" {
				item.Description = descBuilder.String()
			}
		} else {
			item.Content = trimmed
		}
	} else {
		item.Content = trimmed
	}

	// Limita a descrição para exibição concisa no typeahead se for muito longa
	if len(item.Description) > 160 {
		item.Description = item.Description[:157] + "..."
	}

	return item, nil
}
