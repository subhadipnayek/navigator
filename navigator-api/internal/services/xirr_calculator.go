package services

import (
	"math"
	"time"
)

const (
	XirrMaxIterations = 200
	XirrTolerance     = 1e-10
	DaysPerYear       = 365.25
)

type CashFlow struct {
	Date   time.Time
	Amount float64
}

// Calculate computes XIRR (extended internal rate of return) for a series of cash flows.
// Negative amounts = outflows (investments), positive amounts = inflows (redemptions / final corpus).
// Returns annualized percentage (e.g., 14.87 for 14.87%) or nil if non-convergent.
func CalculateXIRR(cashFlows []CashFlow) *float64 {
	if len(cashFlows) < 2 {
		return nil
	}

	hasNegative := false
	hasPositive := false
	for _, cf := range cashFlows {
		if cf.Amount < 0 {
			hasNegative = true
		}
		if cf.Amount > 0 {
			hasPositive = true
		}
		if hasNegative && hasPositive {
			break
		}
	}
	if !hasNegative || !hasPositive {
		return nil
	}

	baseDate := cashFlows[0].Date
	n := len(cashFlows)
	dayFractions := make([]float64, n)
	amounts := make([]float64, n)

	for i := 0; i < n; i++ {
		dayFractions[i] = cashFlows[i].Date.Sub(baseDate).Hours() / (24.0 * DaysPerYear)
		amounts[i] = cashFlows[i].Amount
	}

	// Initial guess: 10%
	rate := 0.1

	for iter := 0; iter < XirrMaxIterations; iter++ {
		npv := 0.0
		dnpv := 0.0

		for i := 0; i < n; i++ {
			t := dayFractions[i]
			denom := math.Pow(1.0+rate, t)

			if denom == 0 || math.IsNaN(denom) || math.IsInf(denom, 0) {
				break
			}

			npv += amounts[i] / denom
			dnpv -= t * amounts[i] / (denom * (1.0 + rate))
		}

		if math.IsNaN(npv) || math.IsInf(npv, 0) {
			break
		}

		if math.Abs(npv) < XirrTolerance {
			res := roundFloat(rate*100.0, 4)
			return &res
		}

		if math.Abs(dnpv) < 1e-20 {
			break
		}

		newRate := rate - npv/dnpv

		// Guard against divergence: clamp step size
		if newRate < -0.999 {
			newRate = (rate - 0.999) / 2.0
		}
		if newRate > 100.0 {
			newRate = (rate + 100.0) / 2.0
		}

		rate = newRate
	}

	// Fallback to bisection
	return xirrBisectionFallback(amounts, dayFractions)
}

func xirrBisectionFallback(amounts []float64, dayFractions []float64) *float64 {
	lo := -0.99
	hi := 10.0

	npvLo := computeNpv(amounts, dayFractions, lo)
	npvHi := computeNpv(amounts, dayFractions, hi)

	if npvLo*npvHi > 0 {
		return nil
	}

	for iter := 0; iter < XirrMaxIterations*2; iter++ {
		mid := (lo + hi) / 2.0
		npvMid := computeNpv(amounts, dayFractions, mid)

		if math.Abs(npvMid) < XirrTolerance || (hi-lo) < XirrTolerance {
			res := roundFloat(mid*100.0, 4)
			return &res
		}

		if npvMid*npvLo < 0 {
			hi = mid
			npvHi = npvMid
		} else {
			lo = mid
			npvLo = npvMid
		}
	}

	return nil
}

func computeNpv(amounts []float64, dayFractions []float64, rate float64) float64 {
	npv := 0.0
	for i := 0; i < len(amounts); i++ {
		denom := math.Pow(1.0+rate, dayFractions[i])
		if denom == 0 || math.IsNaN(denom) || math.IsInf(denom, 0) {
			return math.NaN()
		}
		npv += amounts[i] / denom
	}
	return npv
}

func roundFloat(val float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(val*p) / p
}
