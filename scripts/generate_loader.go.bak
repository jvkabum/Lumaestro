package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	destRoot := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`

	// 1. Coletar categorias
	entries, _ := ioutil.ReadDir(destRoot)
	
	var mainImports []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		category := e.Name()
		catPath := filepath.Join(destRoot, category)
		
		skills, _ := ioutil.ReadDir(catPath)
		var catImports []string

		for _, s := range skills {
			if !s.IsDir() {
				continue
			}
			catImports = append(catImports, fmt.Sprintf("\t_ \"Lumaestro/internal/agents/skills/%s/%s\"", category, s.Name()))
		}

		// Criar all.go na categoria
		if len(catImports) > 0 {
			allContent := fmt.Sprintf("package %s\n\nimport (\n%s\n)\n", strings.ReplaceAll(category, "-", "_"), strings.Join(catImports, "\n"))
			ioutil.WriteFile(filepath.Join(catPath, "all.go"), []byte(allContent), 0644)
			fmt.Printf("Gerado all.go para: %s (%d skills)\n", category, len(catImports))
			
			mainImports = append(mainImports, fmt.Sprintf("\t_ \"Lumaestro/internal/agents/skills/%s\"", category))
		}
	}

	// Criar o importador mestre no Lumaestro em internal/agents/skills/loader.go
	loaderContent := fmt.Sprintf("package skills\n\nimport (\n%s\n)\n", strings.Join(mainImports, "\n"))
	ioutil.WriteFile(filepath.Join(destRoot, "loader.go"), []byte(loaderContent), 0644)

	fmt.Println("🌟 Sistema de auto-carregamento (loader.go) gerado com sucesso!")
}
