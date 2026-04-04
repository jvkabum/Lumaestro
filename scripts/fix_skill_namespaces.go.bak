package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	destRoot := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`

	// 1. Corrigir Imports em todos os skill.go
	filepath.WalkDir(destRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		if d.Name() == "manager.go" || d.Name() == "loader.go" {
			return nil
		}

		content, _ := os.ReadFile(path)
		newContent := strings.ReplaceAll(
			string(content),
			"antigravity-awesome-skills/internal/skills",
			"Lumaestro/internal/agents/skills",
		)

		if string(content) != newContent {
			os.WriteFile(path, []byte(newContent), 0644)
			// fmt.Printf("Namespace corrigido: %s\n", path)
		}

		return nil
	})

	fmt.Println("🚀 Namespaces (imports) corrigidos em todos os arquivos de skills no Lumaestro!")
}
