package services_test

import (
	"context"
	"testing"
	"time"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open sqlite memory db: %v", err)
	}

	err = db.AutoMigrate(
		&models.Fund{},
		&models.NavHistory{},
		&models.NavSyncTracker{},
		&models.CorporateAction{},
		&models.ApiEndpointConfig{},
	)
	if err != nil {
		t.Fatalf("Failed to auto-migrate: %v", err)
	}

	return db
}

func TestNavService_GetNavHistoryAndReturns(t *testing.T) {
	db := setupTestDB(t)

	// Seed test fund
	schemeCode := 120716
	schemeName := "UTI Nifty 50 Index Fund - Growth Option- Direct"
	fundHouse := "UTI Mutual Fund"
	schemeCategory := "Other Scheme - Index Funds"
	db.Create(&models.Fund{
		SchemeCode:     schemeCode,
		SchemeName:     schemeName,
		FundHouse:      &fundHouse,
		SchemeCategory: &schemeCategory,
	})

	// Seed multi-year NAV history
	d1 := time.Date(2023, 10, 30, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 10, 30, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2025, 10, 30, 0, 0, 0, 0, time.UTC)
	d4 := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)

	histories := []models.NavHistory{
		{SchemeCode: schemeCode, Date: d1, Nav: 120.0000},
		{SchemeCode: schemeCode, Date: d2, Nav: 135.0000},
		{SchemeCode: schemeCode, Date: d3, Nav: 150.0000},
		{SchemeCode: schemeCode, Date: d4, Nav: 162.3751},
	}
	db.Create(&histories)

	caSvc := services.NewCorporateActionService(db)
	amfiSvc := services.NewAmfiService(nil)
	navSvc := services.NewNavService(db, amfiSvc, caSvc)

	ctx := context.Background()

	// 1. Test GetNavHistory
	res, err := navSvc.GetNavHistory(ctx, schemeCode, nil, nil, "daily", true)
	if err != nil {
		t.Fatalf("GetNavHistory failed: %v", err)
	}

	if res.SchemeCode != schemeCode {
		t.Errorf("Expected schemeCode %d, got %d", schemeCode, res.SchemeCode)
	}
	if len(res.Data) != 4 {
		t.Fatalf("Expected 4 data points, got %d", len(res.Data))
	}

	// Verify date format is dd-MM-yyyy
	expectedDates := []string{"30-10-2023", "30-10-2024", "30-10-2025", "30-10-2026"}
	for i, d := range res.Data {
		if d.Date != expectedDates[i] {
			t.Errorf("Point %d: expected date %s, got %s", i, expectedDates[i], d.Date)
		}
	}

	if res.NavStartDate == nil || *res.NavStartDate != "30-10-2023" {
		t.Errorf("Expected navStartDate 30-10-2023, got %v", res.NavStartDate)
	}
	if res.NavEndDate == nil || *res.NavEndDate != "30-10-2026" {
		t.Errorf("Expected navEndDate 30-10-2026, got %v", res.NavEndDate)
	}
	if res.LatestNavDate == nil || *res.LatestNavDate != "30-10-2026" {
		t.Errorf("Expected latestNavDate 30-10-2026, got %v", res.LatestNavDate)
	}
	if res.CorporateActions == nil {
		t.Errorf("Expected corporateActions not to be nil")
	}

	// 2. Test GetReturns for 3-year period
	retRes, err := navSvc.GetReturns(ctx, schemeCode, d1, d4)
	if err != nil {
		t.Fatalf("GetReturns failed: %v", err)
	}

	if retRes.StartDate != "30-10-2023" {
		t.Errorf("Expected start date 30-10-2023, got %s", retRes.StartDate)
	}
	if retRes.EndDate != "30-10-2026" {
		t.Errorf("Expected end date 30-10-2026, got %s", retRes.EndDate)
	}
	if retRes.StartNav != 120.0 {
		t.Errorf("Expected startNav 120.0, got %f", retRes.StartNav)
	}
	if retRes.EndNav != 162.3751 {
		t.Errorf("Expected endNav 162.3751, got %f", retRes.EndNav)
	}
	if retRes.AbsoluteReturn <= 0 {
		t.Errorf("Expected positive absolute return, got %f", retRes.AbsoluteReturn)
	}
	if retRes.Cagr <= 0 {
		t.Errorf("Expected positive CAGR, got %f", retRes.Cagr)
	}

	// 3. Test Smart Delta Cache Hit (0 API calls when DB already has range)
	// Requested range: 2024-01-01 to 2026-01-01 (fully inside 2023-10-30 to 2026-10-30)
	reqFrom := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	reqTo := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	wasFetched, err := navSvc.EnsureHistoricalRange(ctx, schemeCode, &reqFrom, &reqTo)
	if err != nil {
		t.Fatalf("EnsureHistoricalRange failed: %v", err)
	}
	if wasFetched {
		t.Errorf("Expected wasFetched to be false (0 API calls) for range fully covered by DB")
	}
}
