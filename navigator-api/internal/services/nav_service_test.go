package services_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestNavService_SingleflightDeduplication(t *testing.T) {
	db := setupTestDB(t)
	schemeCode := 140084

	// Mock server that tracks number of calls
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		time.Sleep(30 * time.Millisecond) // Simulate upstream latency
		resp := `{
			"data": {
				"mf_name": "Nippon",
				"scheme_name": "Nippon ETF",
				"nav_groups": [
					{
						"nav_name": "Nippon ETF",
						"historical_records": [
							{"date": "2024-01-01", "nav": 200.0},
							{"date": "2024-01-02", "nav": 201.0}
						]
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	// Seed fund
	name := "Nippon India ETF Nifty 50 BeES"
	db.Create(&models.Fund{SchemeCode: schemeCode, SchemeName: name})

	db.Create(&models.ApiEndpointConfig{
		Key:         "AMFI_NAV_HISTORY_URL",
		Name:        "AMFI History",
		Description: "Mock",
		Url:         server.URL,
		DefaultUrl:  server.URL,
		IsActive:    true,
	})

	intSvc := services.NewIntegrationService(db)
	amfiSvc := services.NewAmfiService(intSvc)
	caSvc := services.NewCorporateActionService(db)
	navSvc := services.NewNavService(db, amfiSvc, caSvc)

	// Fire 5 concurrent calls simultaneously for the same scheme
	const concurrency = 5
	var wg sync.WaitGroup
	errs := make(chan error, concurrency)

	d1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := navSvc.EnsureHistoricalRange(context.Background(), schemeCode, &d1, &d2)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("Unexpected error in concurrent EnsureHistoricalRange: %v", err)
	}

	// Singleflight must ensure exactly 1 call was made to the upstream server!
	if calls != 1 {
		t.Errorf("Expected exactly 1 upstream call due to singleflight deduplication, got %d", calls)
	}

	// Verify records were saved to SQLite as we go
	var count int64
	db.Model(&models.NavHistory{}).Where("scheme_code = ?", schemeCode).Count(&count)
	if count != 2 {
		t.Errorf("Expected 2 records saved in SQLite, got %d", count)
	}
}
