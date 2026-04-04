package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

func main() {
    baseDir := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`
    
    err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && strings.HasSuffix(path, ".go") && info.Name() != "manager.go" {
            content, err := os.ReadFile(path)
            if err != nil {
                return err
            }
            newContent := strings.ReplaceAll(string(content), "antigravity-awesome-skills/internal/skills", "Lumaestro/internal/agents/skills")
            if string(content) != newContent {
                err = os.WriteFile(path, []byte(newContent), 0644)
                if err != nil {
                    return err
                }
                // fmt.Printf("Namespace corrigido em %s\n", path)
            }
        }
        return nil
    })
    
    if err != nil {
        fmt.Printf("Erro ao corrigir namespaces: %v\n", err)
        os.Exit(1)
    }
    fmt.Println("🚀 Namespaces e imports sincronizados com o motor do Lumaestro!")
}
