package db

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InstanceDB mantém a referência global para o banco de dados principal (SQLite).
var InstanceDB *gorm.DB

// AnalyticsDB mantém a referência global para o banco analítico (DuckDB).
var AnalyticsDB interface{} // Será tipado como *lightning.DuckDBStore após importação no main ou via interface

// InitDB inicializa o banco SQLite (sem CGO) e propaga o schema do Paperclip.
func InitDB() error {
	// Cria uma pasta oculta local para guardar o banco da empresa
	dbFolder := filepath.Join(".lumaestro")
	err := os.MkdirAll(dbFolder, 0755)
	if err != nil {
		log.Printf("Falha ao criar diretório %s: %v\n", dbFolder, err)
		return err
	}

	dbPath := filepath.Join(dbFolder, "database.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	
	if err != nil {
		log.Printf("Falha ao abrir o banco de dados Lumaestro: %v\n", err)
		return err
	}

	InstanceDB = db

	// Processo de Auto Migrate: garante que a estrutura em schema.go 
	// sempre reflita as tabelas físicas.
	log.Println("Migrando Schemas Corporativos (Companies, Agents, Issues, Costs)...")
	err = db.AutoMigrate(
		&Agent{},
		&AgentSecret{},
		&Goal{},
		&Project{},
		&Issue{},
		&IssueComment{},
		&Document{},
		&DocumentRevision{},
		&Asset{},
		&IssueAttachment{},
		&HeartbeatRun{},
		&CostEvent{},
		&Approval{},
		&ActivityLog{},
	)
	if err != nil {
		log.Printf("Falha ao migrar schemas no SQLite: %v\n", err)
		return err
	}

	log.Println("Banco de dados SQLite (Paperclip Mode) inicializado e fundido com sucesso! 🧠")

	// 📊 Inicialização do Pulmão Analítico (DuckDB)
	analyticsPath := filepath.Join(dbFolder, "analytics.db")
	log.Printf("Inicializando Pulmão Analítico DuckDB em %s...\n", analyticsPath)
	
	// Nota: A inicialização real do motor Lightning será feita no main 
	// para evitar dependência circular entre db e lightning.
	
	return nil
}
