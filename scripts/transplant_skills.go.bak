package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	sourceRoot := `c:\git\projeto sem nome ia\antigravity-awesome-skills\internal\skills`
	destRoot := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`

	// 1. Limpar Legacy no Lumaestro
	entries, _ := os.ReadDir(destRoot)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "category_") {
			os.Remove(filepath.Join(destRoot, e.Name()))
			fmt.Printf("Removido legado: %s\n", e.Name())
		}
	}

	// 2. Transplante Estruturado
	filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Ignorar o registry.go original (pois já temos o manager.go no Lumaestro)
		if d.Name() == "registry.go" {
			return nil
		}

		relPath, _ := filepath.Rel(sourceRoot, path)
		if relPath == "." {
			return nil
		}

		targetPath := filepath.Join(destRoot, relPath)

		if d.IsDir() {
			os.MkdirAll(targetPath, 0755)
			return nil
		}

		// Processar Arquivos .go (Atualizar Imports)
		if strings.HasSuffix(d.Name(), ".go") {
			content, _ := os.ReadFile(path)
			newContent := strings.ReplaceAll(
				string(content),
				"antigravity-awesome-skills/internal/skills",
				"Lumaestro/internal/agents/skills",
			)
			
			os.WriteFile(targetPath, []byte(newContent), 0644)
			// fmt.Printf("Transplantado: %s\n", relPath)
		}

		return nil
	})

	fmt.Println("🚀 Transplante de motor de skills CONCLUÍDO com sucesso!")
}
