package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	baseDir := `c:\git\projeto sem nome ia\Lumaestro\skills`
	outputDir := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`
	catalogPath := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills\_awesome_backup_catalog.md` // Não importa se falhar

	// 1. Mapear Categorias
	skillToCategory := make(map[string]string)
	currentCategory := "general"

	file, err := os.Open(catalogPath)
	if err == nil {
		scanner := bufio.NewScanner(file)
		catRegex := regexp.MustCompile(`^## (.*) \(\d+\)`)
		skillRegex := regexp.MustCompile("^\\| `([^`]+)` \\|")
		for scanner.Scan() {
			line := scanner.Text()
			if matches := catRegex.FindStringSubmatch(line); len(matches) > 1 {
				currentCategory = strings.ToLower(strings.ReplaceAll(matches[1], " ", "_"))
			} else if matches := skillRegex.FindStringSubmatch(line); len(matches) > 1 {
				skillToCategory[matches[1]] = currentCategory
			}
		}
		file.Close()
	}

	// 2. Coletar Conteúdo das Skills
	categoryData := make(map[string]map[string]string)

	filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}

		skillFile := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			content, _ := os.ReadFile(skillFile)
			skillName := filepath.Base(path)
			
			category := skillToCategory[skillName]
			if category == "" {
				category = "others"
			}

			if categoryData[category] == nil {
				categoryData[category] = make(map[string]string)
			}
			
			// Limpar caracteres NUL que quebram o compilador Go
			cleanContent := strings.ReplaceAll(string(content), "\x00", "")
			
			// Escapar backticks para Go string literals
			safeContent := strings.ReplaceAll(cleanContent, "`", "` + \"`\" + `")
			categoryData[category][skillName] = safeContent
		}
		return nil
	})

	// 3. Gerar Arquivos Go
	for cat, skills := range categoryData {
		filename := fmt.Sprintf("category_%s.go", cat)
		filePath := filepath.Join(outputDir, filename)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("package skills\n\nfunc init() {\n"))
		for name, content := range skills {
			sb.WriteString(fmt.Sprintf("\tRegister(\"%s\", `%s`)\n", name, content))
		}
		sb.WriteString("}\n")

		err := os.WriteFile(filePath, []byte(sb.String()), 0644)
		if err != nil {
			fmt.Printf("Erro ao gravar %s: %v\n", filename, err)
		} else {
			fmt.Printf("Gerado: %s (%d skills)\n", filename, len(skills))
		}
	}
}
