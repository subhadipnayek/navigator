package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"navigator-api/internal/config"
	"navigator-api/internal/models"
)

func TestInitDB_Sqlite(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "navigator_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_navigator.db")
	cfg := &config.Config{
		DatabaseType: "sqlite",
		DatabaseURL:  dbPath,
	}

	db, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed for sqlite: %v", err)
	}

	if db.Dialector.Name() != "sqlite" {
		t.Errorf("expected dialector 'sqlite', got '%s'", db.Dialector.Name())
	}

	// 1. Verify WAL Mode
	var journalMode string
	row := db.Raw("PRAGMA journal_mode;").Row()
	_ = row.Scan(&journalMode)
	if !strings.EqualFold(journalMode, "wal") {
		t.Errorf("expected journal_mode 'wal', got '%s'", journalMode)
	}

	// 2. Verify Schema AutoMigration: Seeded endpoints should exist
	var count int64
	if err := db.Model(&models.ApiEndpointConfig{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to query ApiEndpointConfigs: %v", err)
	}
	if count < 3 {
		t.Errorf("expected at least 3 seeded endpoint configs, found %d", count)
	}

	// 3. Insert and Search Fund using portable LOWER LIKE
	testFund := models.Fund{
		SchemeCode:   120716,
		SchemeName:   "Nippon India Small Cap Fund - Direct Plan - Growth",
		FundHouse:    ptr("Nippon India Mutual Fund"),
		SchemeType:   ptr("Open Ended Schemes"),
		CreatedAtUtc: time.Now().UTC(),
		UpdatedAtUtc: time.Now().UTC(),
	}
	if err := db.Create(&testFund).Error; err != nil {
		t.Fatalf("failed to create test fund: %v", err)
	}

	var found models.Fund
	searchQuery := "%nippon india small cap%"
	err = db.Where("LOWER(scheme_name) LIKE LOWER(?)", searchQuery).First(&found).Error
	if err != nil {
		t.Fatalf("portable search query failed on SQLite: %v", err)
	}
	if found.SchemeCode != 120716 {
		t.Errorf("expected scheme code 120716, got %d", found.SchemeCode)
	}

	// 4. Test NavHistory record storage and timestamp retrieval
	now := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	testNav := models.NavHistory{
		SchemeCode: 120716,
		Date:       now,
		Nav:        152.3456,
	}
	if err := db.Create(&testNav).Error; err != nil {
		t.Fatalf("failed to insert nav history: %v", err)
	}

	var fetchedNav models.NavHistory
	err = db.Where("scheme_code = ? AND date = ?", 120716, now).First(&fetchedNav).Error
	if err != nil {
		t.Fatalf("failed to query nav history: %v", err)
	}
	if fetchedNav.Nav != 152.3456 {
		t.Errorf("expected NAV 152.3456, got %f", fetchedNav.Nav)
	}

	// 5. Test Partial Unique Index on PortfolioTransaction (portfolio_id, fingerprint)
	tx1 := models.PortfolioTransaction{
		PortfolioId:     1,
		SchemeCode:      120716,
		TransactionDate: now,
		TransactionType: "SIP",
		Units:           10.5,
		Nav:             152.3456,
		Amount:          1600.00,
		Fingerprint:     "fp_unique_123",
	}
	if err := db.Create(&tx1).Error; err != nil {
		t.Fatalf("failed to insert transaction with fingerprint: %v", err)
	}

	// Duplicate fingerprint under same portfolio should violate unique index
	tx2 := tx1
	tx2.Id = 0
	err = db.Create(&tx2).Error
	if err == nil {
		t.Errorf("expected error on duplicate (portfolio_id, fingerprint), but insert succeeded")
	}

	// But an empty fingerprint should be allowed multiple times (due to partial index `WHERE fingerprint != ''`)
	txEmpty1 := models.PortfolioTransaction{
		PortfolioId:     1,
		SchemeCode:      120716,
		TransactionDate: now,
		TransactionType: "Purchase",
		Units:           5.0,
		Nav:             152.3456,
		Amount:          760.00,
		Fingerprint:     "",
	}
	if err := db.Create(&txEmpty1).Error; err != nil {
		t.Fatalf("failed to insert transaction with empty fingerprint: %v", err)
	}

	txEmpty2 := txEmpty1
	txEmpty2.Id = 0
	if err := db.Create(&txEmpty2).Error; err != nil {
		t.Errorf("expected empty fingerprint to be allowed multiple times in partial index: %v", err)
	}
}

func ptr(s string) *string {
	return &s
}
