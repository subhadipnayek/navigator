package database

import (
	"log"
	"time"

	"navigator-api/internal/config"
	"navigator-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Create extensions
	_ = db.Exec(`CREATE EXTENSION IF NOT EXISTS "pg_trgm";`).Error

	// Auto migrate tables
	err = db.AutoMigrate(
		&models.AppUser{},
		&models.Fund{},
		&models.NavHistory{},
		&models.NavSyncTracker{},
		&models.CorporateAction{},
		&models.UserPortfolio{},
		&models.PortfolioTransaction{},
		&models.UserGridPreference{},
		&models.UserPortfolioDashboardCache{},
	)
	if err != nil {
		log.Printf("AutoMigrate warning: %v", err)
	}

	// Create GIN indexes with pg_trgm for ultra-fast fund searching if not already created
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_funds_scheme_name_trgm ON "Funds" USING gin (scheme_name gin_trgm_ops);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_funds_fund_house_trgm ON "Funds" USING gin (fund_house gin_trgm_ops);`).Error

	// Create unique constraint on (portfolio_id, fingerprint)
	_ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_portfolio_transactions_port_fp ON "PortfolioTransactions" (portfolio_id, fingerprint) WHERE fingerprint != '';`).Error

	DB = db
	log.Println("Database connection established and schema verified.")
	return db, nil
}
