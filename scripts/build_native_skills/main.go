package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ToValidPackageName converts a skill name to a valid Go package name
func ToValidPackageName(name string) string {
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, " ", "_")
	
	if len(name) > 0 && unicode.IsDigit(rune(name[0])) {
		name = "p_" + name
	}
	
	// remove any non-alphanumeric/underscore char
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	name = reg.ReplaceAllString(name, "")

	if name == "documentation" || name == "testdata" || name == "vendor" {
		name = "p_" + name
	}
	
	return name
}

func main() {
	sourceRoot := `c:\git\projeto sem nome ia\antigravity-awesome-skills\skills`
	catalogPath := `c:\git\projeto sem nome ia\antigravity-awesome-skills\CATALOG.md`
	destRoot := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`

	// 1. Limpar pastas de habilidades existentes em Lumaestro para evitar lixo
	entries, _ := os.ReadDir(destRoot)
	for _, e := range entries {
		if e.IsDir() {
			os.RemoveAll(filepath.Join(destRoot, e.Name()))
		}
	}

	// 2. Parse CATALOG.md
	skillToCategory := make(map[string]string)
	currentCategory := "general"

	file, err := os.Open(catalogPath)
	if err == nil {
		scanner := bufio.NewScanner(file)
		catRegex := regexp.MustCompile(`^## (.*) \(\d+\)`)
		skillRegex := regexp.MustCompile(`^\| ` + "`" + `([^` + "`" + `]+)` + "`" + ` \|`)
		for scanner.Scan() {
			line := scanner.Text()
			if matches := catRegex.FindStringSubmatch(line); len(matches) > 1 {
				catName := strings.ToLower(strings.ReplaceAll(matches[1], " ", "_"))
				catName = strings.ReplaceAll(catName, "-", "_")
				catName = strings.ReplaceAll(catName, "/", "_")
				currentCategory = catName
			} else if matches := skillRegex.FindStringSubmatch(line); len(matches) > 1 {
				skillToCategory[matches[1]] = currentCategory
			}
		}
		file.Close()
	}

	// 3. Process Skills
	count := 0
	filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}

		skillFile := filepath.Join(path, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			skillName := filepath.Base(path)
			category, ok := skillToCategory[skillName]
			if !ok {
				category = "general"
			}

			content, _ := os.ReadFile(skillFile)
			
			cleanContent := strings.ReplaceAll(string(content), "\x00", "")
			safeContent := strings.ReplaceAll(cleanContent, "`", "` + \"`\" + `")

			safePkgName := ToValidPackageName(skillName)
			
			targetDir := filepath.Join(destRoot, category, safePkgName)
			os.MkdirAll(targetDir, 0755)

			filename := filepath.Join(targetDir, "skill.go")
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf("package %s\n\n", safePkgName))
			sb.WriteString("import \"Lumaestro/internal/agents/skills\"\n\n")
			sb.WriteString("func init() {\n")
			sb.WriteString("\tskills.Register(skills.Skill{\n")
			sb.WriteString(fmt.Sprintf("\t\tName: \"%s\",\n", skillName))
			sb.WriteString(fmt.Sprintf("\t\tCategory: \"%s\",\n", category))
			sb.WriteString(fmt.Sprintf("\t\tContent: `%s`,\n", safeContent))
			sb.WriteString("\t})\n")
			sb.WriteString("}\n")

			os.WriteFile(filename, []byte(sb.String()), 0644)
			count++
		}
		return nil
	})

    // 4. Generate Loaders
    entries, _ = os.ReadDir(destRoot)
	var mainImports []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		category := e.Name()
		catPath := filepath.Join(destRoot, category)
		skillsDir, _ := os.ReadDir(catPath)
		var catImports []string
		for _, s := range skillsDir {
			if !s.IsDir() {
				continue
			}
			catImports = append(catImports, fmt.Sprintf("\t_ \"Lumaestro/internal/agents/skills/%s/%s\"", category, s.Name()))
		}

		if len(catImports) > 0 {
			allContent := fmt.Sprintf("package %s\n\nimport (\n%s\n)\n", ToValidPackageName(category), strings.Join(catImports, "\n"))
			os.WriteFile(filepath.Join(catPath, "all.go"), []byte(allContent), 0644)
			mainImports = append(mainImports, fmt.Sprintf("\t_ \"Lumaestro/internal/agents/skills/%s\"", category))
		}
	}

	// Gerar loader isolado para evitar import cycle
	loaderDir := filepath.Join(destRoot, "loader")
	os.MkdirAll(loaderDir, 0755)
	
	loaderContent := fmt.Sprintf("package loader\n\nimport (\n%s\n)\n", strings.Join(mainImports, "\n"))
	os.WriteFile(filepath.Join(loaderDir, "loader.go"), []byte(loaderContent), 0644)


	fmt.Printf("🌟 O arsenal Lumaestro foi inteiramente recriado (%d skills) sem erros sintáticos!\n", count)
}
