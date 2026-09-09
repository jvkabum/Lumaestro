package config

import (
	"testing"
)

func TestPermissionsEngine(t *testing.T) {
	settings := &AntigravitySettings{
		AgentMode:      "accept-edits",
		ToolPermission: "ask",
		Permissions: AntigravityPermissions{
			Allow: []string{
				"command(git status)",
				"command(git diff)",
				"read(*)",
			},
			Ask: []string{
				"command(rm -rf *)",
				"write(prod.env)",
			},
			Deny: []string{
				"command(format c:)",
				"write(/etc/passwd)",
			},
		},
	}

	// 1. Teste Allow
	if lvl := EvaluatePermission(settings, "command", "git status"); lvl != "allow" {
		t.Errorf("esperava 'allow' para git status, obteve '%s'", lvl)
	}

	// 2. Teste Deny
	if lvl := EvaluatePermission(settings, "command", "format c:"); lvl != "deny" {
		t.Errorf("esperava 'deny' para format c:, obteve '%s'", lvl)
	}

	// 3. Teste Prioridade Deny > Ask > Allow
	// Se adicionarmos a mesma regra em deny e allow, deve prevalecer DENY
	conflictSettings := &AntigravitySettings{
		Permissions: AntigravityPermissions{
			Allow: []string{"command(git push --force)"},
			Deny:  []string{"command(git push --force)"},
		},
	}
	if lvl := EvaluatePermission(conflictSettings, "command", "git push --force"); lvl != "deny" {
		t.Errorf("esperava prioridade DENY sobre ALLOW, obteve '%s'", lvl)
	}

	// 4. Teste ParseRule e FormatRule
	rule := FormatRule("command", "git checkout -b feature")
	if rule != "command(git checkout -b feature)" {
		t.Errorf("formatação incorreta: %s", rule)
	}

	act, tgt := ParseRule(rule)
	if act != "command" || tgt != "git checkout -b feature" {
		t.Errorf("parse incorreto: act=%s, tgt=%s", act, tgt)
	}
}
