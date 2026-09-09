package obsidian

import (
	"testing"
)

func TestIsIgnoredPath(t *testing.T) {
	tests := []struct {
		name     string
		relPath  string
		expected bool
	}{
		// Casos bloqueados: node e variantes
		{"node", "node", true},
		{"node_modules", "node_modules", true},
		{"nested_node_modules", "frontend/node_modules", true},
		{"file_in_node_modules", "node_modules/package/index.js", true},
		{"deep_node_file", "apps/web/node_modules/@types/react/index.d.ts", true},
		{"node_cache", "node_cache", true},
		{"node_temp", ".context/node_temp", true},
		{"folder_named_node", "backend/node/index.js", true},

		// Casos bloqueados: dist e build
		{"dist", "dist", true},
		{"nested_dist", "frontend/dist", true},
		{"dist_file", "frontend/dist/assets/index.js", true},
		{"build", "build", true},
		{"nested_build", "server/build/main.exe", true},
		{"bin", "bin", true},
		{"out", "out", true},
		{"target", "target", true},
		{"vendor", "vendor", true},

		// Casos bloqueados: git e metadados ocultos
		{".git", ".git", true},
		{"git_objects", ".git/objects/2a/3b4c", true},
		{".lumaestro", ".lumaestro/cache/topology.json", true},
		{".vscode", ".vscode/settings.json", true},
		{".idea", ".idea/workspace.xml", true},
		{".next", ".next/cache/build.json", true},

		// Casos permitidos (código e documentação válidos)
		{"normal_go_file", "internal/core/app.go", false},
		{"normal_md_file", "docs/architecture.md", false},
		{"readme", "README.md", false},
		{"frontend_source", "frontend/src/App.jsx", false},
		{"component", "frontend/src/components/GraphView.vue", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := IsIgnoredPath(tc.name, tc.relPath)
			if actual != tc.expected {
				t.Errorf("IsIgnoredPath(%q, %q) = %v; esperado %v", tc.name, tc.relPath, actual, tc.expected)
			}
		})
	}
}
