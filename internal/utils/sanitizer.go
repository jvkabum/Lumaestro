package utils

import (
	"regexp"
)

// 🛡️ Sanitizer: O Lacre de Amnésia do Lumaestro
// Este utilitário remove caminhos absolutos do host para evitar vazamentos de contexto.

var (
	// Caça C:\..., /home/..., /Users/... e substitui por referências neutras
	pathRegex = regexp.MustCompile(`(?i)[a-z]:\\[^ \n\r\t]+|/[a-z0-9_-]+/[^ \n\r\t]+`)
)

// SanitizePath remove qualquer rastro de caminhos absolutos de uma string.
func SanitizePath(input string) string {
	if input == "" {
		return ""
	}
	return pathRegex.ReplaceAllString(input, "[CAMINHO_ISOLADO]")
}
