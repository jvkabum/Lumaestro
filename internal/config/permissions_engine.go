package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AntigravityPermissions encapsula as listas de regras granulares action(target)
type AntigravityPermissions struct {
	Allow []string `json:"allow"`
	Ask   []string `json:"ask"`
	Deny  []string `json:"deny"`
}

// AntigravitySettings reflete o schema do ~/.gemini/antigravity-cli/settings.json
type AntigravitySettings struct {
	AgentMode               string                 `json:"agentMode"`
	AllowNonWorkspaceAccess bool                   `json:"allowNonWorkspaceAccess"`
	ColorScheme             string                 `json:"colorScheme"`
	Model                   string                 `json:"model"`
	Permissions             AntigravityPermissions `json:"permissions"`
	ToolPermission          string                 `json:"toolPermission"`
	TrustedWorkspaces       []string               `json:"trustedWorkspaces"`
}

var (
	permMu sync.RWMutex
)

// GetSettingsPath localiza o arquivo oficial settings.json do Antigravity CLI.
func GetSettingsPath() (string, error) {
	userHome, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(userHome, ".gemini", "antigravity-cli", "settings.json"),
		filepath.Join(userHome, ".gemini", "config", "settings.json"),
	}

	for _, p := range candidates {
		if _, errStat := os.Stat(p); errStat == nil {
			return p, nil
		}
	}

	// Se não existir nenhum, cria no caminho padrão oficial
	target := filepath.Join(userHome, ".gemini", "antigravity-cli", "settings.json")
	_ = os.MkdirAll(filepath.Dir(target), 0755)
	return target, nil
}

// LoadAntigravitySettings lê o arquivo de configurações do Antigravity CLI.
func LoadAntigravitySettings() (*AntigravitySettings, string, error) {
	permMu.RLock()
	defer permMu.RUnlock()

	path, err := GetSettingsPath()
	if err != nil {
		return nil, "", err
	}

	data, errRead := os.ReadFile(path)
	if errRead != nil {
		// Retorna default se não existir
		settings := &AntigravitySettings{
			AgentMode:               "default",
			AllowNonWorkspaceAccess: true,
			ColorScheme:             "dark",
			Model:                   "Gemini 3.1 Pro (Low)",
			Permissions: AntigravityPermissions{
				Allow: []string{},
				Ask:   []string{},
				Deny:  []string{},
			},
			ToolPermission:    "ask",
			TrustedWorkspaces: []string{},
		}
		return settings, path, nil
	}

	var settings AntigravitySettings
	if errJson := json.Unmarshal(data, &settings); errJson != nil {
		return nil, path, fmt.Errorf("falha ao interpretar settings.json: %w", errJson)
	}

	if settings.Permissions.Allow == nil {
		settings.Permissions.Allow = []string{}
	}
	if settings.Permissions.Ask == nil {
		settings.Permissions.Ask = []string{}
	}
	if settings.Permissions.Deny == nil {
		settings.Permissions.Deny = []string{}
	}
	if settings.TrustedWorkspaces == nil {
		settings.TrustedWorkspaces = []string{}
	}

	return &settings, path, nil
}

// SaveAntigravitySettings persiste as configurações no arquivo settings.json.
func SaveAntigravitySettings(settings AntigravitySettings) error {
	permMu.Lock()
	defer permMu.Unlock()

	path, err := GetSettingsPath()
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(path), 0755)

	encoded, errEnc := json.MarshalIndent(settings, "", "  ")
	if errEnc != nil {
		return fmt.Errorf("falha ao serializar settings.json: %w", errEnc)
	}

	return os.WriteFile(path, encoded, 0644)
}

// FormatRule monta a string no padrão action(target)
func FormatRule(action string, target string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	target = strings.TrimSpace(target)
	return fmt.Sprintf("%s(%s)", action, target)
}

// ParseRule extrai action e target de uma string action(target)
func ParseRule(rule string) (string, string) {
	rule = strings.TrimSpace(rule)
	idxOpen := strings.Index(rule, "(")
	idxClose := strings.LastIndex(rule, ")")
	if idxOpen > 0 && idxClose > idxOpen {
		action := strings.TrimSpace(rule[:idxOpen])
		target := strings.TrimSpace(rule[idxOpen+1 : idxClose])
		return action, target
	}
	return "", rule
}

// MatchRule verifica se uma ação e alvo casam com a regra action(target).
func MatchRule(rule string, action string, target string) bool {
	ruleAction, ruleTarget := ParseRule(rule)
	if !strings.EqualFold(ruleAction, action) {
		// Equivalências: command <-> run
		if !((ruleAction == "command" || ruleAction == "run") && (action == "command" || action == "run")) {
			return false
		}
	}

	ruleTargetClean := strings.TrimSpace(strings.ToLower(ruleTarget))
	targetClean := strings.TrimSpace(strings.ToLower(target))

	// Match exato
	if ruleTargetClean == targetClean {
		return true
	}

	// Match de prefixo ou substring de comando (ex: "Remove-Item" casa com comando Remove-Item -Recurse)
	if strings.Contains(targetClean, ruleTargetClean) {
		return true
	}

	// Match com coringa (wildcard *)
	if strings.HasSuffix(ruleTargetClean, "*") {
		prefix := strings.TrimSuffix(ruleTargetClean, "*")
		if strings.HasPrefix(targetClean, prefix) {
			return true
		}
	}

	return false
}

// EvaluatePermission avalia o nível de permissão respeitando a prioridade:
// Deny > Ask > Allow
func EvaluatePermission(settings *AntigravitySettings, action string, target string) string {
	if settings == nil {
		return "ask"
	}

	action = strings.ToLower(strings.TrimSpace(action))
	target = strings.TrimSpace(target)

	// 1. Prioridade Máxima: DENY
	for _, rule := range settings.Permissions.Deny {
		if MatchRule(rule, action, target) {
			return "deny"
		}
	}

	// 2. Prioridade Média: ASK
	for _, rule := range settings.Permissions.Ask {
		if MatchRule(rule, action, target) {
			return "ask"
		}
	}

	// 3. Prioridade Base: ALLOW
	for _, rule := range settings.Permissions.Allow {
		if MatchRule(rule, action, target) {
			return "allow"
		}
	}

	// Fallback padrão se nenhuma regra específica casar
	switch action {
	case "read":
		return "allow"
	case "write", "create", "delete", "move":
		if settings.ToolPermission == "always-proceed" {
			return "allow"
		}
		return "ask"
	case "command", "run":
		if settings.ToolPermission == "always-proceed" {
			return "allow"
		}
		return "ask"
	default:
		return "ask"
	}
}

// AddPermissionRule adiciona uma nova regra ao nível especificado ("allow", "ask", "deny").
func AddPermissionRule(level string, action string, target string) error {
	settings, _, err := LoadAntigravitySettings()
	if err != nil {
		return err
	}

	ruleStr := FormatRule(action, target)
	level = strings.ToLower(strings.TrimSpace(level))

	// Remove de outras listas para não haver conflitos
	settings.Permissions.Allow = removeRuleFromSlice(settings.Permissions.Allow, ruleStr)
	settings.Permissions.Ask = removeRuleFromSlice(settings.Permissions.Ask, ruleStr)
	settings.Permissions.Deny = removeRuleFromSlice(settings.Permissions.Deny, ruleStr)

	switch level {
	case "allow":
		settings.Permissions.Allow = append(settings.Permissions.Allow, ruleStr)
	case "ask":
		settings.Permissions.Ask = append(settings.Permissions.Ask, ruleStr)
	case "deny":
		settings.Permissions.Deny = append(settings.Permissions.Deny, ruleStr)
	default:
		return fmt.Errorf("nível de permissão inválido '%s'. Use 'allow', 'ask' ou 'deny'", level)
	}

	return SaveAntigravitySettings(*settings)
}

// RemovePermissionRule remove uma regra existente de qualquer nível.
func RemovePermissionRule(rule string) error {
	settings, _, err := LoadAntigravitySettings()
	if err != nil {
		return err
	}

	settings.Permissions.Allow = removeRuleFromSlice(settings.Permissions.Allow, rule)
	settings.Permissions.Ask = removeRuleFromSlice(settings.Permissions.Ask, rule)
	settings.Permissions.Deny = removeRuleFromSlice(settings.Permissions.Deny, rule)

	return SaveAntigravitySettings(*settings)
}

func removeRuleFromSlice(slice []string, rule string) []string {
	var result []string
	for _, r := range slice {
		if !strings.EqualFold(strings.TrimSpace(r), strings.TrimSpace(rule)) {
			result = append(result, r)
		}
	}
	return result
}
