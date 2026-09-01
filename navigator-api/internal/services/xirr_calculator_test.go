package services_test

import (
	"math"
	"testing"
	"time"

	"navigator-api/internal/services"
)

func TestCalculateXIRR_StandardScenarios(t *testing.T) {
	t.Run("One year 10 percent lump sum gain", func(t *testing.T) {
		t0 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		cfs := []services.CashFlow{
			{Date: t0, Amount: -10000},
			{Date: t1, Amount: 11000},
		}

		result := services.CalculateXIRR(cfs)
		if result == nil {
			t.Fatalf("Expected XIRR result, got nil")
		}

		expected := 10.0
		if math.Abs(*result-expected) > 0.1 {
			t.Errorf("Expected ~%.2f%%, got %.4f%%", expected, *result)
		}
	})

	t.Run("Two year doubling (100% total gain -> ~41.42% CAGR)", func(t *testing.T) {
		t0 := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		cfs := []services.CashFlow{
			{Date: t0, Amount: -100000},
			{Date: t1, Amount: 200000},
		}

		result := services.CalculateXIRR(cfs)
		if result == nil {
			t.Fatalf("Expected XIRR result, got nil")
		}

		// 100k -> 200k in 2 years is sqrt(2)-1 = 41.4213%
		if math.Abs(*result-41.42) > 0.2 {
			t.Errorf("Expected ~41.42%%, got %.4f%%", *result)
		}
	})

	t.Run("Zero return over 1 year", func(t *testing.T) {
		t0 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		cfs := []services.CashFlow{
			{Date: t0, Amount: -50000},
			{Date: t1, Amount: 50000},
		}

		result := services.CalculateXIRR(cfs)
		if result == nil {
			t.Fatalf("Expected XIRR result, got nil")
		}

		if math.Abs(*result-0.0) > 0.05 {
			t.Errorf("Expected ~0.0%%, got %.4f%%", *result)
		}
	})

	t.Run("Negative return (-20% over 1 year)", func(t *testing.T) {
		t0 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

		cfs := []services.CashFlow{
			{Date: t0, Amount: -10000},
			{Date: t1, Amount: 8000},
		}

		result := services.CalculateXIRR(cfs)
		if result == nil {
			t.Fatalf("Expected XIRR result, got nil")
		}

		expected := -20.0
		if math.Abs(*result-expected) > 0.2 {
			t.Errorf("Expected ~%.2f%%, got %.4f%%", expected, *result)
		}
	})

	t.Run("Multi-month SIP with final corpus", func(t *testing.T) {
		startDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		cfs := make([]services.CashFlow, 0, 13)

		// 12 monthly installments of 5000
		for i := 0; i < 12; i++ {
			cfs = append(cfs, services.CashFlow{
				Date:   startDate.AddDate(0, i, 0),
				Amount: -5000,
			})
		}
		// Final valuation at end of year 1: 66,000 (invested 60,000)
		cfs = append(cfs, services.CashFlow{
			Date:   startDate.AddDate(1, 0, 0),
			Amount: 66000,
		})

		result := services.CalculateXIRR(cfs)
		if result == nil {
			t.Fatalf("Expected XIRR result, got nil")
		}

		// Positive return for SIP that grew from 60k to 66k in 1 year should be ~18-20% annualized
		if *result <= 0 || *result > 50 {
			t.Errorf("Expected reasonable SIP XIRR (10-30%%), got %.4f%%", *result)
		}
	})
}

func TestCalculateXIRR_EdgeCasesAndInvalidInputs(t *testing.T) {
	t0 := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("Empty cash flows", func(t *testing.T) {
		result := services.CalculateXIRR([]services.CashFlow{})
		if result != nil {
			t.Errorf("Expected nil for empty cash flows, got %v", *result)
		}
	})

	t.Run("Single cash flow", func(t *testing.T) {
		result := services.CalculateXIRR([]services.CashFlow{
			{Date: t0, Amount: -1000},
		})
		if result != nil {
			t.Errorf("Expected nil for single cash flow, got %v", *result)
		}
	})

	t.Run("All negative cash flows (no redemption/valuation)", func(t *testing.T) {
		result := services.CalculateXIRR([]services.CashFlow{
			{Date: t0, Amount: -1000},
			{Date: t0.AddDate(0, 1, 0), Amount: -1000},
			{Date: t0.AddDate(0, 2, 0), Amount: -1000},
		})
		if result != nil {
			t.Errorf("Expected nil when no positive cash flows exist, got %v", *result)
		}
	})

	t.Run("All positive cash flows (no outflows)", func(t *testing.T) {
		result := services.CalculateXIRR([]services.CashFlow{
			{Date: t0, Amount: 1000},
			{Date: t0.AddDate(0, 1, 0), Amount: 1000},
		})
		if result != nil {
			t.Errorf("Expected nil when no negative cash flows exist, got %v", *result)
		}
	})
}
