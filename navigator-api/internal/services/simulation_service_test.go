package services

import (
	"math"
	"testing"
	"time"

	"navigator-api/internal/models"
)

func TestGenerateSipDates(t *testing.T) {
	start := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)

	t.Run("SIP on 10th of month (starts next month)", func(t *testing.T) {
		dates := generateSipDates(start, end, 10)
		// Expected: Feb 10, Mar 10, Apr 10, May 10, Jun 10 (5 dates)
		if len(dates) != 5 {
			t.Errorf("Expected 5 SIP dates, got %d", len(dates))
		}
		if len(dates) > 0 && dates[0].Month() != time.February {
			t.Errorf("First SIP should be in Feb, got %v", dates[0])
		}
	})

	t.Run("SIP on 20th of month (starts same month)", func(t *testing.T) {
		dates := generateSipDates(start, end, 20)
		// Expected: Jan 20, Feb 20, Mar 20, Apr 20, May 20 (5 dates)
		if len(dates) != 5 {
			t.Errorf("Expected 5 SIP dates, got %d", len(dates))
		}
		if len(dates) > 0 && dates[0].Month() != time.January {
			t.Errorf("First SIP should be in Jan, got %v", dates[0])
		}
	})

	t.Run("Clamps sipDay to 1..28", func(t *testing.T) {
		datesLow := generateSipDates(start, end, -5)
		if len(datesLow) > 0 && datesLow[0].Day() != 1 {
			t.Errorf("Expected sipDay clamped to 1, got day %d", datesLow[0].Day())
		}

		datesHigh := generateSipDates(start, end, 31)
		if len(datesHigh) > 0 && datesHigh[0].Day() != 28 {
			t.Errorf("Expected sipDay clamped to 28, got day %d", datesHigh[0].Day())
		}
	})
}

func TestGenerateRebalanceDates(t *testing.T) {
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("Quarterly rebalance over 1 year", func(t *testing.T) {
		dates := generateRebalanceDates("Quarterly", start, end)
		// Apr 1, Jul 1, Oct 1, Jan 1 => 4 dates
		if len(dates) != 4 {
			t.Errorf("Expected 4 quarterly rebalance dates, got %d", len(dates))
		}
	})

	t.Run("Yearly rebalance over 1 year", func(t *testing.T) {
		dates := generateRebalanceDates("Yearly", start, end)
		if len(dates) != 1 {
			t.Errorf("Expected 1 yearly rebalance date, got %d", len(dates))
		}
	})

	t.Run("None frequency returns nil", func(t *testing.T) {
		dates := generateRebalanceDates("None", start, end)
		if dates != nil {
			t.Errorf("Expected nil for 'None' frequency, got %v", dates)
		}
	})
}

func TestComputeRiskMetrics(t *testing.T) {
	t.Run("Computes Max Drawdown and Volatility", func(t *testing.T) {
		snaps := []models.PortfolioSnapshot{
			{Date: "01-01-2023", PortfolioValue: 100000, TotalInvested: 100000},
			{Date: "01-02-2023", PortfolioValue: 110000, TotalInvested: 100000},
			{Date: "01-03-2023", PortfolioValue: 88000, TotalInvested: 100000}, // 20% drop from 110k peak
			{Date: "01-04-2023", PortfolioValue: 99000, TotalInvested: 100000},
			{Date: "01-05-2023", PortfolioValue: 120000, TotalInvested: 100000},
		}

		mdd, vol := computeRiskMetrics(snaps)
		if mdd == nil {
			t.Fatalf("Expected Max Drawdown, got nil")
		}
		// Peak = 110k, trough = 88k => (110k - 88k)/110k = 22k / 110k = 20.00%
		if math.Abs(*mdd-20.0) > 0.1 {
			t.Errorf("Expected Max Drawdown ~20.00%%, got %.2f%%", *mdd)
		}
		if vol == nil || *vol <= 0 {
			t.Errorf("Expected positive annualized volatility, got %v", vol)
		}
	})

	t.Run("Less than 2 snapshots returns nil", func(t *testing.T) {
		snaps := []models.PortfolioSnapshot{
			{Date: "01-01-2023", PortfolioValue: 100000},
		}
		mdd, vol := computeRiskMetrics(snaps)
		if mdd != nil || vol != nil {
			t.Errorf("Expected nil for single snapshot")
		}
	})
}

func TestGenerateInsights(t *testing.T) {
	cagr1 := 15.5
	cagr2 := 18.2
	vol1 := 12.0
	vol2 := 16.5

	scenarios := []models.WhatIfScenarioResult{
		{
			Name:                   "Conservative 60/40",
			Cagr:                   &cagr1,
			CorpusPerRupeeInvested: 1.45,
			Volatility:             &vol1,
		},
		{
			Name:                   "Aggressive 100% Equity",
			Cagr:                   &cagr2,
			CorpusPerRupeeInvested: 1.68,
			Volatility:             &vol2,
		},
	}

	insights := generateInsights(scenarios)
	if len(insights) < 3 {
		t.Fatalf("Expected at least 3 insights, got %d", len(insights))
	}
}
