package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type PortfolioSimulationService struct {
	db        *gorm.DB
	navSvc    *NavService
	caSvc     *CorporateActionService
	fundSvc   *FundService
}

func NewPortfolioSimulationService(
	db *gorm.DB,
	nav *NavService,
	ca *CorporateActionService,
	fund *FundService,
) *PortfolioSimulationService {
	return &PortfolioSimulationService{
		db:      db,
		navSvc:  nav,
		caSvc:   ca,
		fundSvc: fund,
	}
}

type fundNavData struct {
	Fund          models.Fund
	Navs          []models.NavHistory
	Actions       []models.CorporateActionDto
	AllocationPct float64
}

type fundState struct {
	Units              float64
	Invested           float64
	LastCaAppliedUpTo  time.Time
}

type simEvent struct {
	Date     time.Time
	Priority int // 0: Lump/SIP, 1: Rebalance, 2: Snapshot
	Type     string
}

func (s *PortfolioSimulationService) Simulate(ctx context.Context, req *models.SimulationRequest) (*models.SimulationResponse, error) {
	reqStart, err := ParseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	reqEnd, err := ParseDate(req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	effStart, effEnd, wasClamped, note, err := s.computeEffectiveRange(ctx, []models.BasketDefinition{req.Basket}, reqStart, reqEnd)
	if err != nil {
		return nil, err
	}

	res, err := s.runSimulationCore(ctx, req.Basket, req.SipAmount, req.SipDay, req.LumpSumAmount, effStart, effEnd)
	if err != nil {
		return nil, err
	}

	res.StartDate = FormatDateDDMMYYYY(reqStart)
	res.EndDate = FormatDateDDMMYYYY(reqEnd)
	res.EffectiveStartDate = FormatDateDDMMYYYY(effStart)
	res.EffectiveEndDate = FormatDateDDMMYYYY(effEnd)
	res.WasDateRangeClamped = wasClamped
	res.DateRangeNote = note

	return res, nil
}

func (s *PortfolioSimulationService) runSimulationCore(
	ctx context.Context,
	basket models.BasketDefinition,
	sipAmount float64,
	sipDay int,
	lumpSum float64,
	startDate, endDate time.Time,
) (*models.SimulationResponse, error) {
	schemeCodes := make([]int, len(basket.Funds))
	for i, f := range basket.Funds {
		schemeCodes[i] = f.SchemeCode
	}

	navHistories, err := s.navSvc.EnsureNavHistories(ctx, schemeCodes, startDate, endDate)
	if err != nil {
		return nil, err
	}

	fundDataMap := make(map[int]*fundNavData)
	stateMap := make(map[int]*fundState)

	for _, alloc := range basket.Funds {
		var fund models.Fund
		if err := s.db.WithContext(ctx).Where("scheme_code = ?", alloc.SchemeCode).First(&fund).Error; err != nil {
			return nil, fmt.Errorf("fund %d not found", alloc.SchemeCode)
		}

		actions, _ := s.caSvc.GetActions(ctx, alloc.SchemeCode)
		navs := navHistories[alloc.SchemeCode]

		fundDataMap[alloc.SchemeCode] = &fundNavData{
			Fund:          fund,
			Navs:          navs,
			Actions:       actions,
			AllocationPct: alloc.AllocationPercent,
		}

		stateMap[alloc.SchemeCode] = &fundState{
			LastCaAppliedUpTo: startDate.AddDate(0, 0, -1),
		}
	}

	totalInvested := 0.0
	sipInstallments := 0
	var cashFlows []CashFlow
	var timeSeries []models.PortfolioSnapshot

	// Build events
	var events []simEvent
	if lumpSum > 0 {
		events = append(events, simEvent{Date: startDate, Priority: 0, Type: "LUMP"})
	}

	if sipAmount > 0 {
		sipDates := generateSipDates(startDate, endDate, sipDay)
		for _, d := range sipDates {
			events = append(events, simEvent{Date: d, Priority: 0, Type: "SIP"})
		}
	}

	if !strings.EqualFold(basket.RebalanceFrequency, "None") && basket.RebalanceFrequency != "" {
		rebalDates := generateRebalanceDates(basket.RebalanceFrequency, startDate, endDate)
		for _, d := range rebalDates {
			events = append(events, simEvent{Date: d, Priority: 1, Type: "REBAL"})
		}
	}

	snapDates := buildSnapDates(startDate, endDate)
	for _, d := range snapDates {
		events = append(events, simEvent{Date: d, Priority: 2, Type: "SNAP"})
	}

	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].Date.Equal(events[j].Date) {
			return events[i].Date.Before(events[j].Date)
		}
		return events[i].Priority < events[j].Priority
	})

	for _, ev := range events {
		switch ev.Type {
		case "LUMP", "SIP":
			amount := sipAmount
			if ev.Type == "LUMP" {
				amount = lumpSum
			}
			processed := false

			for code, fd := range fundDataMap {
				nav := FindNavOnOrAfter(fd.Navs, ev.Date, 7, true)
				if nav == nil || nav.Nav <= 0 {
					continue
				}

				st := stateMap[code]
				applyCorporateActions(st, fd.Actions, st.LastCaAppliedUpTo, nav.Date)
				st.LastCaAppliedUpTo = nav.Date

				allocAmount := amount * fd.AllocationPct / 100.0
				st.Units += allocAmount / nav.Nav
				st.Invested += allocAmount
				processed = true
			}

			if processed {
				totalInvested += amount
				cashFlows = append(cashFlows, CashFlow{Date: ev.Date, Amount: -amount})
				if ev.Type == "SIP" {
					sipInstallments++
				}
			}

		case "REBAL":
			totalValue := 0.0
			navMap := make(map[int]*models.NavHistory)

			for code, fd := range fundDataMap {
				nav := FindNavOnOrAfter(fd.Navs, ev.Date, 7, true)
				if nav == nil || nav.Nav <= 0 {
					continue
				}

				st := stateMap[code]
				applyCorporateActions(st, fd.Actions, st.LastCaAppliedUpTo, nav.Date)
				st.LastCaAppliedUpTo = nav.Date

				navMap[code] = nav
				totalValue += st.Units * nav.Nav
			}

			if totalValue > 0 {
				for code, fd := range fundDataMap {
					nav, ok := navMap[code]
					if !ok || nav.Nav <= 0 {
						continue
					}
					targetVal := totalValue * fd.AllocationPct / 100.0
					st := stateMap[code]
					st.Units = targetVal / nav.Nav
					st.Invested = totalInvested * fd.AllocationPct / 100.0
				}
			}

		case "SNAP":
			portValue := 0.0
			var fundSnaps []models.FundSnapshot

			for code, fd := range fundDataMap {
				nav := FindNavOnOrBefore(fd.Navs, ev.Date)
				if nav == nil || nav.Nav <= 0 {
					continue
				}

				st := stateMap[code]
				applyCorporateActions(st, fd.Actions, st.LastCaAppliedUpTo, nav.Date)
				st.LastCaAppliedUpTo = nav.Date

				val := st.Units * nav.Nav
				portValue += val
				fundSnaps = append(fundSnaps, models.FundSnapshot{
					SchemeCode: code,
					Value:      roundFloat(val, 2),
					Units:      roundFloat(st.Units, 4),
				})
			}

			gainPct := 0.0
			if totalInvested > 0 {
				gainPct = roundFloat((portValue-totalInvested)/totalInvested*100.0, 2)
			}

			timeSeries = append(timeSeries, models.PortfolioSnapshot{
				Date:           FormatDateDDMMYYYY(ev.Date),
				PortfolioValue: roundFloat(portValue, 2),
				TotalInvested:  roundFloat(totalInvested, 2),
				GainPercent:    gainPct,
				Funds:          fundSnaps,
			})
		}
	}

	finalCorpus := 0.0
	var fundDetails []models.FundSimulationDetail

	for code, fd := range fundDataMap {
		lastNav := FindNavOnOrBefore(fd.Navs, endDate)
		if lastNav == nil || lastNav.Nav <= 0 {
			continue
		}

		st := stateMap[code]
		applyCorporateActions(st, fd.Actions, st.LastCaAppliedUpTo, lastNav.Date)
		st.LastCaAppliedUpTo = lastNav.Date

		val := st.Units * lastNav.Nav
		invested := st.Invested
		finalCorpus += val

		absRet := 0.0
		if invested > 0 {
			absRet = roundFloat((val-invested)/invested*100.0, 2)
		}

		fundDetails = append(fundDetails, models.FundSimulationDetail{
			SchemeCode:            code,
			SchemeName:            fd.Fund.SchemeName,
			AllocationPercent:     fd.AllocationPct,
			TotalUnits:            roundFloat(st.Units, 4),
			TotalInvested:         roundFloat(invested, 2),
			FinalValue:            roundFloat(val, 2),
			AbsoluteReturnPercent: absRet,
		})
	}

	absReturn := 0.0
	if totalInvested > 0 {
		absReturn = roundFloat((finalCorpus-totalInvested)/totalInvested*100.0, 2)
	}

	cagr := CalculateCagr(totalInvested, finalCorpus, startDate, endDate)
	cashFlows = append(cashFlows, CashFlow{Date: endDate, Amount: finalCorpus})
	xirr := CalculateXIRR(cashFlows)
	maxDd, vol := computeRiskMetrics(timeSeries)

	return &models.SimulationResponse{
		BasketName:            basket.Name,
		TotalInvested:         roundFloat(totalInvested, 2),
		FinalCorpusValue:      roundFloat(finalCorpus, 2),
		AbsoluteReturnPercent: absReturn,
		Cagr:                  cagr,
		Xirr:                  xirr,
		MaxDrawdownPercent:    maxDd,
		Volatility:            vol,
		StartDate:             FormatDateDDMMYYYY(startDate),
		EndDate:               FormatDateDDMMYYYY(endDate),
		EffectiveStartDate:    FormatDateDDMMYYYY(startDate),
		EffectiveEndDate:      FormatDateDDMMYYYY(endDate),
		SipInstallments:       sipInstallments,
		SipAmount:             sipAmount,
		LumpSumAmount:         lumpSum,
		FundDetails:           fundDetails,
		TimeSeries:            timeSeries,
	}, nil
}

func (s *PortfolioSimulationService) Compare(ctx context.Context, req *models.ComparisonRequest) (*models.ComparisonResponse, error) {
	reqStart, err := ParseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	reqEnd, err := ParseDate(req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	var baskets []models.BasketDefinition
	for _, entry := range req.Entries {
		baskets = append(baskets, resolveBasket(entry))
	}

	effStart, effEnd, wasClamped, note, err := s.computeEffectiveRange(ctx, baskets, reqStart, reqEnd)
	if err != nil {
		return nil, err
	}

	var results []models.ComparisonEntryResult
	allTimeSeries := make(map[string][]models.PortfolioSnapshot)

	for _, entry := range req.Entries {
		basket := resolveBasket(entry)
		sim, err := s.runSimulationCore(ctx, basket, entry.SipAmount, entry.SipDay, entry.LumpSumAmount, effStart, effEnd)
		if err != nil {
			return nil, err
		}

		results = append(results, models.ComparisonEntryResult{
			Name:                  entry.Name,
			TotalInvested:         sim.TotalInvested,
			FinalCorpusValue:      sim.FinalCorpusValue,
			AbsoluteReturnPercent: sim.AbsoluteReturnPercent,
			Cagr:                  sim.Cagr,
			Xirr:                  sim.Xirr,
			MaxDrawdownPercent:    sim.MaxDrawdownPercent,
			Volatility:            sim.Volatility,
		})
		allTimeSeries[entry.Name] = sim.TimeSeries
	}

	// Compute alpha vs first entry
	if len(results) > 1 && results[0].Cagr != nil {
		benchCagr := *results[0].Cagr
		for i := 1; i < len(results); i++ {
			if results[i].Cagr != nil {
				alpha := roundFloat(*results[i].Cagr-benchCagr, 2)
				results[i].AlphaVsFirst = &alpha
			}
		}
	}

	alignedTs := buildAlignedTimeSeries(allTimeSeries, req.NormalizeSipAmounts)

	return &models.ComparisonResponse{
		StartDate:           FormatDateDDMMYYYY(reqStart),
		EndDate:             FormatDateDDMMYYYY(reqEnd),
		EffectiveStartDate:  FormatDateDDMMYYYY(effStart),
		EffectiveEndDate:    FormatDateDDMMYYYY(effEnd),
		WasDateRangeClamped: wasClamped,
		DateRangeNote:       note,
		IsNormalized:        req.NormalizeSipAmounts,
		Results:             results,
		TimeSeries:          alignedTs,
	}, nil
}

func (s *PortfolioSimulationService) WhatIf(ctx context.Context, req *models.WhatIfRequest) (*models.WhatIfResponse, error) {
	reqStart, err := ParseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	reqEnd, err := ParseDate(req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	var baskets []models.BasketDefinition
	for _, sc := range req.Scenarios {
		baskets = append(baskets, sc.Basket)
	}

	effStart, effEnd, wasClamped, note, err := s.computeEffectiveRange(ctx, baskets, reqStart, reqEnd)
	if err != nil {
		return nil, err
	}

	var results []models.WhatIfScenarioResult
	for _, sc := range req.Scenarios {
		sim, err := s.runSimulationCore(ctx, sc.Basket, sc.SipAmount, sc.SipDay, sc.LumpSumAmount, effStart, effEnd)
		if err != nil {
			return nil, err
		}

		cpr := 0.0
		if sim.TotalInvested > 0 {
			cpr = roundFloat(sim.FinalCorpusValue/sim.TotalInvested, 4)
		}

		results = append(results, models.WhatIfScenarioResult{
			Name:                   sc.Name,
			SipAmount:              sc.SipAmount,
			LumpSumAmount:          sc.LumpSumAmount,
			TotalInvested:          sim.TotalInvested,
			FinalCorpusValue:       sim.FinalCorpusValue,
			AbsoluteReturnPercent:  sim.AbsoluteReturnPercent,
			Cagr:                   sim.Cagr,
			Xirr:                   sim.Xirr,
			MaxDrawdownPercent:     sim.MaxDrawdownPercent,
			Volatility:             sim.Volatility,
			CorpusPerRupeeInvested: cpr,
		})
	}

	insights := generateInsights(results)

	return &models.WhatIfResponse{
		StartDate:           FormatDateDDMMYYYY(reqStart),
		EndDate:             FormatDateDDMMYYYY(reqEnd),
		EffectiveStartDate:  FormatDateDDMMYYYY(effStart),
		EffectiveEndDate:    FormatDateDDMMYYYY(effEnd),
		WasDateRangeClamped: wasClamped,
		DateRangeNote:       note,
		Scenarios:           results,
		Insights:            insights,
	}, nil
}

func (s *PortfolioSimulationService) RollingComparison(ctx context.Context, req *models.RollingComparisonRequest) (*models.RollingComparisonResponse, error) {
	startDate, err := ParseDate(req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %w", err)
	}
	endDate, err := ParseDate(req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %w", err)
	}

	var entryResults []models.RollingComparisonEntryResult
	allSeries := make(map[string][]models.RollingReturnDto)

	for _, entry := range req.Entries {
		basket := resolveBasket(entry)
		rollingData, err := s.computeRollingReturnsForBasket(ctx, basket, req.PeriodInYears, startDate, endDate)
		if err != nil {
			continue
		}

		var values []float64
		for _, r := range rollingData {
			values = append(values, r.ReturnPercentage)
		}
		sort.Float64s(values)

		var minRet, maxRet, avgRet, medRet *float64
		if len(values) > 0 {
			mn := values[0]
			mx := values[len(values)-1]
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			av := roundFloat(sum/float64(len(values)), 2)
			md := GetMedian(values)
			minRet = &mn
			maxRet = &mx
			avgRet = &av
			medRet = md
		}

		entryResults = append(entryResults, models.RollingComparisonEntryResult{
			Name:          entry.Name,
			DataPoints:    len(values),
			MinReturn:     minRet,
			MaxReturn:     maxRet,
			AverageReturn: avgRet,
			MedianReturn:  medRet,
		})

		allSeries[entry.Name] = rollingData
	}

	// Build aligned time series
	dateMap := make(map[string]bool)
	for _, series := range allSeries {
		for _, pt := range series {
			dateMap[pt.Date] = true
		}
	}

	var allDates []time.Time
	for ds := range dateMap {
		if t, err := ParseDate(ds); err == nil {
			allDates = append(allDates, t)
		}
	}
	sort.Slice(allDates, func(i, j int) bool { return allDates[i].Before(allDates[j]) })

	intervalStr := ""
	if req.Interval != nil {
		intervalStr = *req.Interval
	}
	allDates = SampleDates(allDates, intervalStr)

	var timeSeries []models.RollingComparisonTimePoint
	for _, d := range allDates {
		ds := FormatDateDDMMYYYY(d)
		point := models.RollingComparisonTimePoint{
			Date:   ds,
			Values: []models.RollingComparisonSeriesValue{},
		}
		for name, series := range allSeries {
			var retPct *float64
			for _, s := range series {
				if s.Date == ds {
					val := s.ReturnPercentage
					retPct = &val
					break
				}
			}
			point.Values = append(point.Values, models.RollingComparisonSeriesValue{
				Name:          name,
				ReturnPercent: retPct,
			})
		}
		timeSeries = append(timeSeries, point)
	}

	return &models.RollingComparisonResponse{
		StartDate:     FormatDateDDMMYYYY(startDate),
		EndDate:       FormatDateDDMMYYYY(endDate),
		PeriodInYears: req.PeriodInYears,
		Results:       entryResults,
		TimeSeries:    timeSeries,
	}, nil
}

func (s *PortfolioSimulationService) computeRollingReturnsForBasket(
	ctx context.Context,
	basket models.BasketDefinition,
	periodYears int,
	start, end time.Time,
) ([]models.RollingReturnDto, error) {
	if len(basket.Funds) == 1 {
		res, err := s.navSvc.GetRollingReturns(ctx, basket.Funds[0].SchemeCode, periodYears, &start, &end, "daily")
		if err != nil {
			return nil, err
		}
		return res.Data, nil
	}

	// Multi fund rolling returns: compute composite daily NAV index
	var schemeCodes []int
	for _, f := range basket.Funds {
		schemeCodes = append(schemeCodes, f.SchemeCode)
	}
	navMap, err := s.navSvc.EnsureNavHistories(ctx, schemeCodes, start.AddDate(-periodYears, 0, 0), end)
	if err != nil {
		return nil, err
	}

	// Find common dates
	firstFundNavs := navMap[schemeCodes[0]]
	var rolling []models.RollingReturnDto

	for i := 0; i < len(firstFundNavs); i++ {
		endDate := firstFundNavs[i].Date
		if endDate.Before(start) || endDate.After(end) {
			continue
		}
		startDate := endDate.AddDate(-periodYears, 0, 0)

		startVal := 0.0
		endVal := 0.0
		valid := true

		for _, alloc := range basket.Funds {
			navs := navMap[alloc.SchemeCode]
			sNav := FindClosestNav(navs, startDate, 15)
			eNav := FindClosestNav(navs, endDate, 15)
			if sNav == nil || eNav == nil || sNav.Nav <= 0 || eNav.Nav <= 0 {
				valid = false
				break
			}
			startVal += alloc.AllocationPercent
			endVal += alloc.AllocationPercent * (eNav.Nav / sNav.Nav)
		}

		if valid && startVal > 0 && endVal > 0 {
			cagr := CalculateCagr(startVal, endVal, startDate, endDate)
			if cagr != nil {
				rolling = append(rolling, models.RollingReturnDto{
					Date:             FormatDateDDMMYYYY(endDate),
					ReturnPercentage: *cagr,
				})
			}
		}
	}

	return rolling, nil
}

// ─── Helpers ───

func (s *PortfolioSimulationService) computeEffectiveRange(
	ctx context.Context,
	baskets []models.BasketDefinition,
	reqStart, reqEnd time.Time,
) (time.Time, time.Time, bool, *string, error) {
	effStart := reqStart
	effEnd := reqEnd

	var allCodes []int
	for _, b := range baskets {
		for _, f := range b.Funds {
			allCodes = append(allCodes, f.SchemeCode)
		}
	}

	var trackers []models.NavSyncTracker
	_ = s.db.WithContext(ctx).Where("scheme_code IN ?", allCodes).Find(&trackers).Error
	trackerMap := make(map[int]models.NavSyncTracker)
	for _, t := range trackers {
		trackerMap[t.SchemeCode] = t
	}

	for _, code := range allCodes {
		t, ok := trackerMap[code]
		if ok {
			if t.NavStartDate != nil && t.NavStartDate.After(effStart) {
				effStart = *t.NavStartDate
			}
			if t.NavEndDate != nil && t.NavEndDate.Before(effEnd) {
				effEnd = *t.NavEndDate
			}
		}
	}

	if effStart.After(effEnd) {
		return reqStart, reqEnd, false, nil, fmt.Errorf("no overlapping date range across funds")
	}

	wasClamped := !effStart.Equal(reqStart) || !effEnd.Equal(reqEnd)
	var note *string
	if wasClamped {
		msg := fmt.Sprintf("Date range adjusted to %s — %s based on available fund history.",
			FormatDateDDMMYYYY(effStart), FormatDateDDMMYYYY(effEnd))
		note = &msg
	}

	return effStart, effEnd, wasClamped, note, nil
}

func resolveBasket(entry models.ComparisonEntry) models.BasketDefinition {
	if entry.Basket != nil {
		return *entry.Basket
	}
	if entry.SingleFundSchemeCode != nil {
		return models.BasketDefinition{
			Name:               entry.Name,
			RebalanceFrequency: "None",
			Funds: []models.FundAllocation{
				{SchemeCode: *entry.SingleFundSchemeCode, AllocationPercent: 100},
			},
		}
	}
	return models.BasketDefinition{Name: entry.Name}
}

func generateSipDates(start, end time.Time, sipDay int) []time.Time {
	if sipDay < 1 {
		sipDay = 1
	}
	if sipDay > 28 {
		sipDay = 28
	}

	var dates []time.Time
	curr := time.Date(start.Year(), start.Month(), sipDay, 0, 0, 0, 0, time.UTC)
	if curr.Before(start) {
		curr = curr.AddDate(0, 1, 0)
	}

	for !curr.After(end) {
		dates = append(dates, curr)
		curr = curr.AddDate(0, 1, 0)
	}
	return dates
}

func generateRebalanceDates(freq string, start, end time.Time) []time.Time {
	months := 0
	switch strings.ToLower(freq) {
	case "monthly":
		months = 1
	case "quarterly":
		months = 3
	case "yearly":
		months = 12
	}
	if months == 0 {
		return nil
	}

	var dates []time.Time
	curr := start.AddDate(0, months, 0)
	for !curr.After(end) {
		dates = append(dates, curr)
		curr = curr.AddDate(0, months, 0)
	}
	return dates
}

func buildSnapDates(start, end time.Time) []time.Time {
	var dates []time.Time
	// First day of next month
	currMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)

	for !currMonth.After(end) {
		// Last day of month
		lastDay := currMonth.AddDate(0, 1, -1)
		if lastDay.After(end) {
			dates = append(dates, end)
			break
		}
		dates = append(dates, lastDay)
		currMonth = currMonth.AddDate(0, 1, 0)
	}

	if len(dates) == 0 || !dates[len(dates)-1].Equal(end) {
		dates = append(dates, end)
	}
	return dates
}

func applyCorporateActions(st *fundState, actions []models.CorporateActionDto, from, to time.Time) {
	for _, a := range actions {
		if IsSeriesBreakAction(a.ActionType) {
			continue
		}
		actDate, err := ParseDate(a.ActionDate)
		if err != nil {
			continue
		}
		if actDate.After(from) && !actDate.After(to) && a.AdjustmentFactor > 0 {
			st.Units *= a.AdjustmentFactor
		}
	}
}

func computeRiskMetrics(snaps []models.PortfolioSnapshot) (*float64, *float64) {
	if len(snaps) < 2 {
		return nil, nil
	}

	peak := 0.0
	maxDd := 0.0
	for _, s := range snaps {
		if s.PortfolioValue > peak {
			peak = s.PortfolioValue
		}
		if peak > 0 {
			dd := (peak - s.PortfolioValue) / peak * 100.0
			if dd > maxDd {
				maxDd = dd
			}
		}
	}

	var returns []float64
	for i := 1; i < len(snaps); i++ {
		if snaps[i-1].PortfolioValue > 0 {
			ret := snaps[i].PortfolioValue/snaps[i-1].PortfolioValue - 1.0
			returns = append(returns, ret)
		}
	}

	var vol *float64
	if len(returns) >= 2 {
		sum := 0.0
		for _, r := range returns {
			sum += r
		}
		avg := sum / float64(len(returns))

		varianceSum := 0.0
		for _, r := range returns {
			varianceSum += (r - avg) * (r - avg)
		}
		variance := varianceSum / float64(len(returns)-1)
		v := roundFloat(math.Sqrt(variance)*math.Sqrt(12)*100.0, 2)
		vol = &v
	}

	mdd := roundFloat(maxDd, 2)
	return &mdd, vol
}

func buildAlignedTimeSeries(allSeries map[string][]models.PortfolioSnapshot, normalize bool) []models.AlignedPortfolioSnapshot {
	dateMap := make(map[string]bool)
	for _, snaps := range allSeries {
		for _, s := range snaps {
			dateMap[s.Date] = true
		}
	}

	var allDates []time.Time
	for ds := range dateMap {
		if t, err := ParseDate(ds); err == nil {
			allDates = append(allDates, t)
		}
	}
	sort.Slice(allDates, func(i, j int) bool { return allDates[i].Before(allDates[j]) })

	var aligned []models.AlignedPortfolioSnapshot
	for _, d := range allDates {
		ds := FormatDateDDMMYYYY(d)
		item := models.AlignedPortfolioSnapshot{
			Date:   ds,
			Values: []models.AlignedSeriesValue{},
		}

		for name, snaps := range allSeries {
			val := 0.0
			for _, s := range snaps {
				if s.Date == ds {
					if normalize && s.TotalInvested > 0 {
						val = (s.PortfolioValue / s.TotalInvested) * 10000.0
					} else {
						val = s.PortfolioValue
					}
					break
				}
			}
			item.Values = append(item.Values, models.AlignedSeriesValue{
				Name:  name,
				Value: roundFloat(val, 2),
			})
		}
		aligned = append(aligned, item)
	}

	return aligned
}

func generateInsights(scenarios []models.WhatIfScenarioResult) []string {
	if len(scenarios) == 0 {
		return nil
	}

	var insights []string
	bestCagrIdx := 0
	bestCprIdx := 0
	lowestVolIdx := 0

	for i, sc := range scenarios {
		if sc.Cagr != nil && (scenarios[bestCagrIdx].Cagr == nil || *sc.Cagr > *scenarios[bestCagrIdx].Cagr) {
			bestCagrIdx = i
		}
		if sc.CorpusPerRupeeInvested > scenarios[bestCprIdx].CorpusPerRupeeInvested {
			bestCprIdx = i
		}
		if sc.Volatility != nil && (scenarios[lowestVolIdx].Volatility == nil || *sc.Volatility < *scenarios[lowestVolIdx].Volatility) {
			lowestVolIdx = i
		}
	}

	if scenarios[bestCagrIdx].Cagr != nil {
		insights = append(insights, fmt.Sprintf("Scenario '%s' delivered the highest CAGR at %.2f%%.",
			scenarios[bestCagrIdx].Name, *scenarios[bestCagrIdx].Cagr))
	}
	insights = append(insights, fmt.Sprintf("Scenario '%s' maximized capital efficiency at ₹%.2f corpus per rupee invested.",
		scenarios[bestCprIdx].Name, scenarios[bestCprIdx].CorpusPerRupeeInvested))
	if scenarios[lowestVolIdx].Volatility != nil {
		insights = append(insights, fmt.Sprintf("Scenario '%s' had the lowest risk profile with annualized volatility of %.2f%%.",
			scenarios[lowestVolIdx].Name, *scenarios[lowestVolIdx].Volatility))
	}

	return insights
}
