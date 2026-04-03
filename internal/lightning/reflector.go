package lightning

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Reflector é o serviço que destila o conhecimento analítico em lições do Obsidian (RAG).
type Reflector struct {
	Store     *DuckDBStore
	VaultPath string
}

// NewReflector cria uma nova instância de reflexão.
func NewReflector(store *DuckDBStore, vaultPath string) *Reflector {
	return &Reflector{
		Store:     store,
		VaultPath: vaultPath,
	}
}

// DistillLesson analisa um rollout e salva o aprendizado no vault do Obsidian.
func (r *Reflector) DistillLesson(rolloutID string) error {
	if r.VaultPath == "" {
		return fmt.Errorf("caminho do vault não configurado")
	}

	// 1. Coletar dados do DuckDB
	var avgReward float64
	var totalTokens int
	row := r.Store.GetDB().QueryRow(`
		SELECT 
			avg(r.reward), 
			sum(s.prompt_tokens + s.completion_tokens) 
		FROM rewards r
		JOIN spans s ON r.rollout_id = s.rollout_id
		WHERE r.rollout_id = ?
	`, rolloutID)
	
	if err := row.Scan(&avgReward, &totalTokens); err != nil {
		return fmt.Errorf("falha ao ler dados da sessão: %w", err)
	}

	// 2. Determinar o Título e Status
	status := "SUCESSO"
	if avgReward < 0 {
		status = "FALHA / REJEIÇÃO"
	}

	// 3. Gerar o Conteúdo Markdown (Template)
	content := fmt.Sprintf(`# 🧠 Lição Aprendida: %s
- **Rollout ID**: %s
- **Status Ético**: %s
- **Consumo**: %d tokens
- **Data**: %s

## 📝 Descrição do Evento
O enxame realizou uma tarefa vinculada a este ID. Através do feedback do Comandante, o sistema identificou um padrão de %s.

## 💡 Diretiva para o Enxame (RAG)
[[Lumaestro-Lightning]] analisou este rastro e recomenda:
> Se o objetivo for similar a este rollout, evite caminhos que levaram a recompensas negativas. 
> Priorize prompts que mantenham a concisão e a precisão financeira.

#lightning #aprendizado #mente-colmeia #rollout-%s
`, rolloutID, rolloutID, status, totalTokens, time.Now().Format("2006-01-02 15:04:05"), status, rolloutID)

	// 4. Salvar no Vault (Pasta de Lições)
	lessonsDir := filepath.Join(r.VaultPath, ".lumaestro", "lessons")
	if err := os.MkdirAll(lessonsDir, 0755); err != nil {
		return fmt.Errorf("falha ao criar diretório de lições: %w", err)
	}

	filename := fmt.Sprintf("lesson_%s.md", rolloutID)
	fullPath := filepath.Join(lessonsDir, filename)

	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("falha ao salvar arquivo de lição: %w", err)
	}

	fmt.Printf("[🧠 Hive Mind] Lição destilada e salva: %s\n", fullPath)
	return nil
}
