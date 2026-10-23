package database

import (
	"log"
	"kanso/backend/internal/infrastructure/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if cfg.DBDriver == "postgres" || cfg.DBDriver == "postgresql" {
		log.Printf("Connecting to PostgreSQL database: %s:%s/%s (user: %s)", cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBUser)
		dialector = postgres.Open(cfg.PostgresDSN())
	} else {
		log.Printf("Connecting to SQLite database: %s", cfg.DBPath)
		dialector = sqlite.Open(cfg.DBPath)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// Auto migrate & seed Duo records
	SeedDuoInitialData(db)

	return db, nil
}
