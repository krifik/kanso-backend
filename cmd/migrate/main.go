package main

import (
	"fmt"
	"log"
	"os"

	"kanso/backend/internal/infrastructure/config"
	"kanso/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func main() {
	cfg := config.LoadConfig()

	sqlitePath := cfg.DBPath
	if len(os.Args) > 1 {
		sqlitePath = os.Args[1]
	}

	log.Printf("📥 Reading data from SQLite: %s", sqlitePath)
	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to open SQLite database: %v", err)
	}

	dsn := cfg.PostgresDSN()
	log.Printf("📤 Connecting to target PostgreSQL...")
	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	log.Println("🔄 Auto-migrating PostgreSQL schema...")
	err = pgDB.AutoMigrate(
		&model.HouseholdCouple{},
		&model.User{},
		&model.PocketPod{},
		&model.Transaction{},
		&model.WishlistItem{},
		&model.LoveTip{},
		&model.EmailMutationLog{},
		&model.UserOAuthSession{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate PostgreSQL schema: %v", err)
	}

	// 1. Migrate Couples
	var couples []model.HouseholdCouple
	if err := sqliteDB.Find(&couples).Error; err == nil && len(couples) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&couples).Error; err != nil {
			log.Printf("Warning migrating couples: %v", err)
		} else {
			log.Printf("✅ Migrated %d couples", len(couples))
		}
	}

	// 2. Migrate Users
	var users []model.User
	if err := sqliteDB.Find(&users).Error; err == nil && len(users) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&users).Error; err != nil {
			log.Printf("Warning migrating users: %v", err)
		} else {
			log.Printf("✅ Migrated %d users", len(users))
		}
	}

	// 3. Migrate Pods
	var pods []model.PocketPod
	if err := sqliteDB.Find(&pods).Error; err == nil && len(pods) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&pods).Error; err != nil {
			log.Printf("Warning migrating pods: %v", err)
		} else {
			log.Printf("✅ Migrated %d pods", len(pods))
		}
	}

	// 4. Migrate Transactions
	var txs []model.Transaction
	if err := sqliteDB.Find(&txs).Error; err == nil && len(txs) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&txs).Error; err != nil {
			log.Printf("Warning migrating transactions: %v", err)
		} else {
			log.Printf("✅ Migrated %d transactions", len(txs))
		}
	}

	// 5. Migrate Wishlist
	var wishlist []model.WishlistItem
	if err := sqliteDB.Find(&wishlist).Error; err == nil && len(wishlist) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&wishlist).Error; err != nil {
			log.Printf("Warning migrating wishlist: %v", err)
		} else {
			log.Printf("✅ Migrated %d wishlist items", len(wishlist))
		}
	}

	// 6. Migrate Love Tips
	var tips []model.LoveTip
	if err := sqliteDB.Find(&tips).Error; err == nil && len(tips) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&tips).Error; err != nil {
			log.Printf("Warning migrating love tips: %v", err)
		} else {
			log.Printf("✅ Migrated %d love tips", len(tips))
		}
	}

	// 7. Migrate Email Mutation Logs
	var logs []model.EmailMutationLog
	if err := sqliteDB.Find(&logs).Error; err == nil && len(logs) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&logs).Error; err != nil {
			log.Printf("Warning migrating email logs: %v", err)
		} else {
			log.Printf("✅ Migrated %d email mutation logs", len(logs))
		}
	}

	// 8. Migrate OAuth Sessions
	var sessions []model.UserOAuthSession
	if err := sqliteDB.Find(&sessions).Error; err == nil && len(sessions) > 0 {
		if err := pgDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&sessions).Error; err != nil {
			log.Printf("Warning migrating oauth sessions: %v", err)
		} else {
			log.Printf("✅ Migrated %d user oauth sessions", len(sessions))
		}
	}

	fmt.Println("\n🎉 Database migration from SQLite to PostgreSQL completed successfully!")
}
