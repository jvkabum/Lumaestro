package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	fmt.Println("=== DIAGNÓSTICO DE AMBIENTE MAESTRO ===")
	
	home, _ := os.UserHomeDir()
	fmt.Printf("User Home: %s\n", home)
	
	name := "gemini"
	path, err := exec.LookPath(name)
	if err == nil {
		fmt.Printf("Gemini no PATH: %s\n", path)
	} else {
		fmt.Println("Gemini NÃO encontrado no PATH padrão.")
	}

	if runtime.GOOS == "windows" {
		npmPath := filepath.Join(home, "AppData", "Roaming", "npm", "node_modules", "@google", "gemini-cli", "bundle", "gemini.js")
		fmt.Printf("Verificando NPM Path JS: %s\n", npmPath)
		if _, err := os.Stat(npmPath); err == nil {
			fmt.Println("✅ Gemini JS encontrado no Roaming!")
		} else {
			fmt.Println("❌ Gemini JS NÃO encontrado no Roaming.")
		}

		npmCmd := filepath.Join(home, "AppData", "Roaming", "npm", name+".cmd")
		fmt.Printf("Verificando NPM CMD: %s\n", npmCmd)
		if _, err := os.Stat(npmCmd); err == nil {
			fmt.Println("✅ Gemini CMD encontrado no Roaming!")
		} else {
			fmt.Println("❌ Gemini CMD NÃO encontrado no Roaming.")
		}
	}
	
	fmt.Println("========================================")
}
