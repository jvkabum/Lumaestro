package main

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

func main() {
    baseDir := `c:\git\projeto sem nome ia\Lumaestro\internal\agents\skills`
    
    // Regex para achar pacotes que começam com número
    pkgRegex := regexp.MustCompile(`(?m)^package (\d.*)$`)

    err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if !info.IsDir() && strings.HasSuffix(path, ".go") && info.Name() != "manager.go" && info.Name() != "loader.go" {
            content, err := os.ReadFile(path)
            if err != nil {
                return err
            }
            
            strContent := string(content)
            
            // Corrige "package 007" para "package p_007"
            if pkgRegex.MatchString(strContent) {
                 strContent = pkgRegex.ReplaceAllString(strContent, "package p_$1")
            }

            if string(content) != strContent {
                os.WriteFile(path, []byte(strContent), 0644)
                fmt.Printf("Pacote inválido corrigido em: %s\n", path)
            }
        }
        return nil
    })

    if err != nil {
        fmt.Println("Erro:", err)
    }
}
