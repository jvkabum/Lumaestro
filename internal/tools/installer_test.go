package tools

import (
	"testing"
)

func TestCheckGeminiAuthWithAntigravity(t *testing.T) {
	installer := NewInstaller()
	hasAuth := installer.CheckGeminiAuth()
	if !hasAuth {
		t.Errorf("Esperava CheckGeminiAuth=true quando agy.exe está instalado no sistema")
	}
}
