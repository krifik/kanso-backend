package database

import (
	"log"
	"kanso/backend/internal/infrastructure/config"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg config.Config) (*gorm.DB, error) {
	log.Printf("Connecting to SQLite database: %s", cfg.DBPath)
	dialector := sqlite.Open(cfg.DBPath)

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
