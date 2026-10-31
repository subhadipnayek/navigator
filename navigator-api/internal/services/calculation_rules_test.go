package services_test

import (
	"math"
	"testing"
	"time"

	"navigator-api/internal/models"
	"navigator-api/internal/services"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Time
		hasError bool
	}{
		{
			input:    "15-08-2023",
			expected: time.Date(2023, 8, 15, 0, 0, 0, 0, time.UTC),
			hasError: false,
		},
		{
			input:    "15-Aug-2023",
			expected: time.Date(2023, 8, 15, 0, 0, 0, 0, time.UTC),
			hasError: false,
		},
		{
			input:    "15-Aug-23",
			expected: time.Date(2023, 8, 15, 0, 0, 0, 0, time.UTC),
			hasError: false,
		},
		{
			input:    "2023-08-15",
			expected: time.Date(2023, 8, 15, 0, 0, 0, 0, time.UTC),
			hasError: false,
		},
		{
			input:    "invalid-date-string",
			hasError: true,
		},
		{
			input:    "",
			hasError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := services.ParseDate(tc.input)
			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for '%s', got date %v", tc.input, got)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error for '%s': %v", tc.input, err)
				}
				if !got.Equal(tc.expected) {
					t.Errorf("For '%s', expected %v, got %v", tc.input, tc.expected, got)
				}
			}
		})
	}
}

func TestDateFormattingAndDaysBetween(t *testing.T) {
	d1 := time.Date(2023, 5, 10, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2023, 5, 20, 0, 0, 0, 0, time.UTC)

	if got := services.FormatDateDDMMYYYY(d1); got != "10-05-2023" {
		t.Errorf("FormatDateDDMMYYYY: expected '10-05-2023', got '%s'", got)
	}

	if got := services.FormatDateYYYYMMDD(d1); got != "2023-05-10" {
		t.Errorf("FormatDateYYYYMMDD: expected '2023-05-10', got '%s'", got)
	}

	ptr := services.FormatDatePtr(&d1)
	if ptr == nil || *ptr != "10-05-2023" {
		t.Errorf("FormatDatePtr: expected '10-05-2023', got %v", ptr)
	}
	if zeroPtr := services.FormatDatePtr(nil); zeroPtr != nil {
		t.Errorf("FormatDatePtr(nil) should return nil, got %v", zeroPtr)
	}

	days := services.DaysBetween(d1, d2)
	if days != 10 {
		t.Errorf("DaysBetween: expected 10, got %d", days)
	}
}

func TestNavBinarySearchLocators(t *testing.T) {
	// Sorted ascending NAV series with weekend gaps
	navs := []models.NavHistory{
		{Id: 1, SchemeCode: 1001, Date: time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC), Nav: 100.0}, // Mon
		{Id: 2, SchemeCode: 1001, Date: time.Date(2023, 1, 3, 0, 0, 0, 0, time.UTC), Nav: 101.0}, // Tue
		{Id: 3, SchemeCode: 1001, Date: time.Date(2023, 1, 4, 0, 0, 0, 0, time.UTC), Nav: 102.0}, // Wed
		{Id: 4, SchemeCode: 1001, Date: time.Date(2023, 1, 5, 0, 0, 0, 0, time.UTC), Nav: 103.0}, // Thu
		{Id: 5, SchemeCode: 1001, Date: time.Date(2023, 1, 6, 0, 0, 0, 0, time.UTC), Nav: 104.0}, // Fri
		// Sat & Sun missing
		{Id: 6, SchemeCode: 1001, Date: time.Date(2023, 1, 9, 0, 0, 0, 0, time.UTC), Nav: 105.0}, // Next Mon
	}

	t.Run("FindNavOnOrBefore on weekend (Saturday)", func(t *testing.T) {
		sat := time.Date(2023, 1, 7, 0, 0, 0, 0, time.UTC)
		found := services.FindNavOnOrBefore(navs, sat)
		if found == nil || found.Nav != 104.0 {
			t.Errorf("Expected Friday NAV (104.0), got %v", found)
		}
	})

	t.Run("FindNavOnOrAfter on weekend (Saturday)", func(t *testing.T) {
		sat := time.Date(2023, 1, 7, 0, 0, 0, 0, time.UTC)
		found := services.FindNavOnOrAfter(navs, sat, 7, false)
		if found == nil || found.Nav != 105.0 {
			t.Errorf("Expected Monday NAV (105.0), got %v", found)
		}
	})

	t.Run("FindClosestNav finds nearest within tolerance", func(t *testing.T) {
		sun := time.Date(2023, 1, 8, 0, 0, 0, 0, time.UTC)
		found := services.FindClosestNav(navs, sun, 3)
		if found == nil {
			t.Fatalf("Expected closest NAV for Sunday within 3 days tolerance")
		}
	})

	t.Run("FindNavOnOrAfter outside tolerance returns nil", func(t *testing.T) {
		farDate := time.Date(2023, 1, 7, 0, 0, 0, 0, time.UTC)
		found := services.FindNavOnOrAfter(navs, farDate, 1, false) // 1 day tolerance, next is 2 days away
		if found != nil {
			t.Errorf("Expected nil due to tight tolerance, got %v", found)
		}
	})
}

func TestCalculateCagr(t *testing.T) {
	dStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	dEnd := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC) // 3 years

	t.Run("100 percent gain over 3 years", func(t *testing.T) {
		// 100 -> 200 in 3 years => (2^(1/3)-1)*100 = 25.99%
		cagr := services.CalculateCagr(100.0, 200.0, dStart, dEnd)
		if cagr == nil {
			t.Fatalf("Expected CAGR, got nil")
		}
		if math.Abs(*cagr-25.99) > 0.1 {
			t.Errorf("Expected ~25.99%%, got %.2f%%", *cagr)
		}
	})

	t.Run("Invalid dates or values return nil", func(t *testing.T) {
		if res := services.CalculateCagr(-10, 100, dStart, dEnd); res != nil {
			t.Errorf("Expected nil for negative start value")
		}
		if res := services.CalculateCagr(100, 200, dEnd, dStart); res != nil {
			t.Errorf("Expected nil when endDate before startDate")
		}
	})
}

func TestGetMedian(t *testing.T) {
	t.Run("Odd number of elements", func(t *testing.T) {
		values := []float64{10.0, 20.0, 30.0, 40.0, 50.0}
		med := services.GetMedian(values)
		if med == nil || *med != 30.0 {
			t.Errorf("Expected median 30.0, got %v", med)
		}
	})

	t.Run("Even number of elements", func(t *testing.T) {
		values := []float64{10.0, 20.0, 30.0, 40.0}
		med := services.GetMedian(values)
		if med == nil || *med != 25.0 {
			t.Errorf("Expected median 25.0, got %v", med)
		}
	})

	t.Run("Empty slice returns nil", func(t *testing.T) {
		if med := services.GetMedian([]float64{}); med != nil {
			t.Errorf("Expected nil for empty slice, got %v", med)
		}
	})
}

func TestComputeAdjustmentFactor(t *testing.T) {
	actions := []models.CorporateActionDto{
		{ActionDate: "15-06-2022", ActionType: "Split", AdjustmentFactor: 5.0},
		{ActionDate: "10-01-2023", ActionType: "Bonus", AdjustmentFactor: 2.0},
		{ActionDate: "01-01-2024", ActionType: "Discontinuity", AdjustmentFactor: 1.0}, // Should be ignored
	}

	fromDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	toDate := time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)

	factor := services.ComputeAdjustmentFactor(actions, fromDate, toDate)
	expectedFactor := 10.0 // 5.0 * 2.0
	if math.Abs(factor-expectedFactor) > 1e-6 {
		t.Errorf("Expected adjustment factor %.1f, got %.4f", expectedFactor, factor)
	}
}

func TestSampleNavDataPoints(t *testing.T) {
	points := []models.NavDataPointDto{
		{Date: "02-01-2023", Nav: 100},
		{Date: "03-01-2023", Nav: 101},
		{Date: "09-01-2023", Nav: 105},
		{Date: "01-02-2023", Nav: 110},
	}

	daily := services.SampleNavDataPoints(points, "daily")
	if len(daily) != 4 {
		t.Errorf("Daily sampling expected 4 points, got %d", len(daily))
	}

	monthly := services.SampleNavDataPoints(points, "monthly")
	if len(monthly) != 2 {
		t.Errorf("Monthly sampling expected 2 months, got %d", len(monthly))
	}
}
