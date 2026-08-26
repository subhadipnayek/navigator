package database

import (
	"log"
	"strings"
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
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
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
		&models.ApiEndpointConfig{},
	)
	if err != nil {
		log.Printf("AutoMigrate warning: %v", err)
	}

	// Create GIN indexes with pg_trgm for ultra-fast fund searching if not already created
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_funds_scheme_name_trgm ON "Funds" USING gin (scheme_name gin_trgm_ops);`).Error
	_ = db.Exec(`CREATE INDEX IF NOT EXISTS idx_funds_fund_house_trgm ON "Funds" USING gin (fund_house gin_trgm_ops);`).Error

	// Create unique constraint on (portfolio_id, fingerprint)
	_ = db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_portfolio_transactions_port_fp ON "PortfolioTransactions" (portfolio_id, fingerprint) WHERE fingerprint != '';`).Error

	seedDefaultEndpointConfigs(db)

	DB = db
	log.Println("Database connection established and schema verified.")
	return db, nil
}

func seedDefaultEndpointConfigs(db *gorm.DB) {
	defaults := []models.ApiEndpointConfig{
		{
			Key:             "AMFI_MASTER_URL",
			Name:            "AMFI Scheme Master Directory",
			Description:     "Master CSV containing all 16,000+ registered mutual fund schemes with AMCs, categories, and ISINs.",
			Url:             "https://portal.amfiindia.com/DownloadSchemeData_Po.aspx?mf=0",
			DefaultUrl:      "https://portal.amfiindia.com/DownloadSchemeData_Po.aspx?mf=0",
			Method:          "GET",
			HeadersJson:     `{"User-Agent": "NAVigator/2.0"}`,
			PayloadTemplate: "",
			IsActive:        true,
		},
		{
			Key:             "AMFI_LATEST_NAV_URL",
			Name:            "AMFI Daily NAV Snapshot (NAVAll.txt)",
			Description:     "Daily updated NAV text file covering latest published NAV values for all mutual funds.",
			Url:             "https://portal.amfiindia.com/spages/NAVAll.txt",
			DefaultUrl:      "https://portal.amfiindia.com/spages/NAVAll.txt",
			Method:          "GET",
			HeadersJson:     `{"User-Agent": "NAVigator/2.0"}`,
			PayloadTemplate: "",
			IsActive:        true,
		},
		{
			Key:             "AMFI_NAV_HISTORY_URL",
			Name:            "AMFI Historical NAV API",
			Description:     "AMFI official historical time-series endpoint for fetching 30+ year daily NAV records by scheme code (supports multi-chunk sliding windows).",
			Url:             "https://www.amfiindia.com/api/nav-history",
			DefaultUrl:      "https://www.amfiindia.com/api/nav-history",
			Method:          "GET",
			HeadersJson:     `{"User-Agent": "NAVigator/2.0"}`,
			PayloadTemplate: "query_type=historical_period&from_date={{fromDate}}&to_date={{toDate}}&sd_id={{schemeCode}}",
			IsActive:        true,
		},
		{
			Key:             "AMFI_PORTAL_NAV_REPORT_URL",
			Name:            "AMFI Portal NAV History Report (Deprecated)",
			Description:     "[Expired/Deprecated] Legacy AMFI portal download report endpoint no longer maintained upstream.",
			Url:             "https://portal.amfiindia.com/DownloadNAVHistoryReport_Po.aspx",
			DefaultUrl:      "https://portal.amfiindia.com/DownloadNAVHistoryReport_Po.aspx",
			Method:          "GET",
			HeadersJson:     `{"User-Agent": "NAVigator/2.0"}`,
			PayloadTemplate: "mf=0&tp=1&frmdt={{fromDate}}&todt={{toDate}}",
			IsActive:        false,
		},
	}

	for _, d := range defaults {
		var existing models.ApiEndpointConfig
		if err := db.Where("key = ?", d.Key).First(&existing).Error; err != nil {
			now := time.Now().UTC()
			d.CreatedAtUtc = now
			d.UpdatedAtUtc = now
			d.LastHealthStatus = "Unknown"
			_ = db.Create(&d).Error
		} else {
			// Ensure template and active status reflect updated defaults
			if d.Key == "AMFI_NAV_HISTORY_URL" && strings.Contains(existing.PayloadTemplate, "schemeCode=") {
				_ = db.Model(&existing).Updates(map[string]interface{}{
					"payload_template": d.PayloadTemplate,
					"description":      d.Description,
					"updated_at":       time.Now().UTC(),
				}).Error
			}
			if d.Key == "AMFI_PORTAL_NAV_REPORT_URL" && existing.IsActive {
				_ = db.Model(&existing).Updates(map[string]interface{}{
					"is_active":   false,
					"name":        d.Name,
					"description": d.Description,
					"updated_at":  time.Now().UTC(),
				}).Error
			}
		}
	}
}
