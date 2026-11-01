package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"navigator-api/internal/models"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavService struct {
	db          *gorm.DB
	amfiService *AmfiService
	caService   *CorporateActionService
	sfGroup     singleflight.Group
}

func NewNavService(db *gorm.DB, amfi *AmfiService, ca *CorporateActionService) *NavService {
	return &NavService{db: db, amfiService: amfi, caService: ca}
}

func (s *NavService) GetNavHistory(
	ctx context.Context,
	schemeCode int,
	from, to *time.Time,
	interval string,
	adjusted bool,
) (*models.NavDataResponse, error) {
	var fund models.Fund
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&fund).Error; err != nil {
		return nil, err
	}

	wasFetchedFromApi, _ := s.EnsureHistoricalRange(ctx, schemeCode, from, to)

	query := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode)
	if from != nil {
		query = query.Where("date >= ?", *from)
	}
	if to != nil {
		query = query.Where("date <= ?", *to)
	}

	var navs []models.NavHistory
	if err := query.Order("date asc").Find(&navs).Error; err != nil {
		return nil, err
	}

	var caActions []models.CorporateActionDto
	caActions, _ = s.caService.GetActions(ctx, schemeCode)
	if caActions == nil {
		caActions = []models.CorporateActionDto{}
	}

	var dataPoints []models.NavDataPointDto
	for _, n := range navs {
		navValue := n.Nav
		if adjusted && len(caActions) > 0 {
			// Apply split adjustment factor for dates before corporate actions
			for _, ca := range caActions {
				if IsSeriesBreakAction(ca.ActionType) {
					continue
				}
				actDate, err := ParseDate(ca.ActionDate)
				if err == nil && n.Date.Before(actDate) && ca.AdjustmentFactor > 0 {
					navValue /= ca.AdjustmentFactor
				}
			}
		}

		dataPoints = append(dataPoints, models.NavDataPointDto{
			Date: FormatDateDDMMYYYY(n.Date),
			Nav:  roundFloat(navValue, 4),
		})
	}

	// Interval sampling
	dataPoints = SampleNavDataPoints(dataPoints, interval)

	var latestNavDateStr *string
	var navStartDateStr *string
	var navEndDateStr *string
	var lastSuccessfulNavCheckUtc *string
	navStatus := "Active"

	if len(navs) > 0 {
		startD := FormatDateDDMMYYYY(navs[0].Date)
		endD := FormatDateDDMMYYYY(navs[len(navs)-1].Date)
		navStartDateStr = &startD
		navEndDateStr = &endD
		latestNavDateStr = &endD
	}

	var tracker models.NavSyncTracker
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&tracker).Error; err == nil {
		if tracker.NavStartDate != nil {
			s := FormatDateDDMMYYYY(*tracker.NavStartDate)
			navStartDateStr = &s
		}
		if tracker.NavEndDate != nil {
			s := FormatDateDDMMYYYY(*tracker.NavEndDate)
			navEndDateStr = &s
		}
		if tracker.LatestNavDate != nil {
			s := FormatDateDDMMYYYY(*tracker.LatestNavDate)
			latestNavDateStr = &s
		}
		utcStr := tracker.UpdatedAtUtc.Format(time.RFC3339)
		lastSuccessfulNavCheckUtc = &utcStr
	}

	if latestNavDateStr != nil {
		if latestDate, err := ParseDate(*latestNavDateStr); err == nil {
			if time.Since(latestDate).Hours() > 24*90 {
				navStatus = "LikelyDiscontinued"
			}
		}
	}

	return &models.NavDataResponse{
		SchemeCode:                fund.SchemeCode,
		SchemeName:                fund.SchemeName,
		FundHouse:                 fund.FundHouse,
		Category:                  fund.SchemeCategory,
		LatestNavDate:             latestNavDateStr,
		NavStartDate:              navStartDateStr,
		NavEndDate:                navEndDateStr,
		LastSuccessfulNavCheckUtc: lastSuccessfulNavCheckUtc,
		NavStatus:                 navStatus,
		IsLikelyDiscontinued:      navStatus == "LikelyDiscontinued",
		WasFetchedFromApi:         wasFetchedFromApi,
		IsAdjusted:                adjusted,
		Data:                      dataPoints,
		CorporateActions:          caActions,
	}, nil
}

func (s *NavService) GetReturns(ctx context.Context, schemeCode int, startDate, endDate time.Time) (*models.ReturnsResponse, error) {
	var fund models.Fund
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&fund).Error; err != nil {
		return nil, err
	}

	var navs []models.NavHistory
	_ = s.db.WithContext(ctx).Where("scheme_code = ? AND date >= ? AND date <= ?", schemeCode, startDate.AddDate(0, 0, -10), endDate.AddDate(0, 0, 10)).Order("date asc").Find(&navs).Error

	startEntry := FindNavOnOrAfter(navs, startDate, 7, true)
	endEntry := FindNavOnOrBefore(navs, endDate)

	if startEntry == nil || endEntry == nil {
		// Ensure missing range is fetched via smart delta
		_, _ = s.EnsureHistoricalRange(ctx, schemeCode, &startDate, &endDate)
		_ = s.db.WithContext(ctx).Where("scheme_code = ? AND date >= ? AND date <= ?", schemeCode, startDate.AddDate(0, 0, -10), endDate.AddDate(0, 0, 10)).Order("date asc").Find(&navs).Error
		startEntry = FindNavOnOrAfter(navs, startDate, 7, true)
		endEntry = FindNavOnOrBefore(navs, endDate)
	}

	if startEntry == nil || endEntry == nil || startEntry.Nav <= 0 || endEntry.Nav <= 0 {
		return nil, fmt.Errorf("could not locate valid NAV points for date range")
	}

	absReturn := (endEntry.Nav - startEntry.Nav) / startEntry.Nav * 100.0
	cagr := CalculateCagr(startEntry.Nav, endEntry.Nav, startEntry.Date, endEntry.Date)
	cagrVal := 0.0
	if cagr != nil {
		cagrVal = *cagr
	}

	days := DaysBetween(startEntry.Date, endEntry.Date)

	return &models.ReturnsResponse{
		SchemeCode:            schemeCode,
		SchemeName:            fund.SchemeName,
		FundHouse:             fund.FundHouse,
		StartDate:             FormatDateDDMMYYYY(startEntry.Date),
		EndDate:               FormatDateDDMMYYYY(endEntry.Date),
		StartNav:              roundFloat(startEntry.Nav, 4),
		EndNav:                roundFloat(endEntry.Nav, 4),
		AbsoluteReturn:        roundFloat(absReturn, 2),
		AbsoluteReturnPercent: roundFloat(absReturn, 2),
		Cagr:                  cagrVal,
		Days:                  days,
		HasDiscontinuity:      false,
	}, nil
}

func (s *NavService) GetRollingReturns(
	ctx context.Context,
	schemeCode int,
	periodInYears int,
	from, to *time.Time,
	interval string,
) (*models.RollingReturnResponse, error) {
	if periodInYears < 1 || periodInYears > 30 {
		periodInYears = 1
	}

	var navs []models.NavHistory
	query := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode)
	if err := query.Order("date asc").Find(&navs).Error; err != nil || len(navs) < 20 {
		startDateNeeded := IndiaToday().AddDate(-periodInYears, 0, 0)
		_, _ = s.EnsureHistoricalRange(ctx, schemeCode, &startDateNeeded, to)
		_ = s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).Order("date asc").Find(&navs).Error
	}
	if len(navs) < 20 {
		return nil, fmt.Errorf("insufficient NAV data for rolling returns")
	}

	var rollingReturns []models.RollingReturnDto
	var returnValues []float64

	for i := 0; i < len(navs); i++ {
		endDate := navs[i].Date
		if from != nil && endDate.Before(*from) {
			continue
		}
		if to != nil && endDate.After(*to) {
			continue
		}

		startDate := endDate.AddDate(-periodInYears, 0, 0)
		startNavEntry := FindClosestNav(navs, startDate, 15)
		if startNavEntry == nil || startNavEntry.Nav <= 0 || navs[i].Nav <= 0 {
			continue
		}

		cagr := CalculateCagr(startNavEntry.Nav, navs[i].Nav, startNavEntry.Date, endDate)
		if cagr != nil {
			rollingReturns = append(rollingReturns, models.RollingReturnDto{
				Date:             FormatDateDDMMYYYY(endDate),
				ReturnPercentage: *cagr,
			})
			returnValues = append(returnValues, *cagr)
		}
	}

	if len(returnValues) == 0 {
		return &models.RollingReturnResponse{
			SchemeCode:    schemeCode,
			PeriodInYears: periodInYears,
			DataPoints:    0,
			Data:          []models.RollingReturnDto{},
		}, nil
	}

	sort.Float64s(returnValues)
	minRet := returnValues[0]
	maxRet := returnValues[len(returnValues)-1]
	sum := 0.0
	for _, v := range returnValues {
		sum += v
	}
	avgRet := roundFloat(sum/float64(len(returnValues)), 2)
	medRet := GetMedian(returnValues)

	sampled := SampleRollingReturns(rollingReturns, interval)

	return &models.RollingReturnResponse{
		SchemeCode:    schemeCode,
		PeriodInYears: periodInYears,
		DataPoints:    len(rollingReturns),
		MinReturn:     &minRet,
		MaxReturn:     &maxRet,
		AverageReturn: &avgRet,
		MedianReturn:  medRet,
		Data:          sampled,
	}, nil
}

func (s *NavService) DetectNavGaps(ctx context.Context, schemeCode int, minGapDays int) ([]models.NavGapDto, error) {
	if minGapDays < 7 {
		minGapDays = 30
	}

	var fund models.Fund
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&fund).Error; err != nil {
		return nil, err
	}

	var navs []models.NavHistory
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).Order("date asc").Find(&navs).Error; err != nil {
		return nil, err
	}

	var gaps []models.NavGapDto
	for i := 1; i < len(navs); i++ {
		prev := navs[i-1]
		curr := navs[i]
		days := DaysBetween(prev.Date, curr.Date)
		if days >= minGapDays {
			gaps = append(gaps, models.NavGapDto{
				SchemeCode: schemeCode,
				SchemeName: fund.SchemeName,
				FromDate:   FormatDateDDMMYYYY(prev.Date),
				ToDate:     FormatDateDDMMYYYY(curr.Date),
				GapDays:    days,
			})
		}
	}

	return gaps, nil
}

func (s *NavService) DetectAllNavGaps(ctx context.Context, minGapDays int) ([]models.NavGapDto, error) {
	var funds []models.Fund
	if err := s.db.WithContext(ctx).Order("scheme_code asc").Find(&funds).Error; err != nil {
		return nil, err
	}

	var allGaps []models.NavGapDto
	for _, f := range funds {
		gaps, _ := s.DetectNavGaps(ctx, f.SchemeCode, minGapDays)
		if len(gaps) > 0 {
			allGaps = append(allGaps, gaps...)
		}
	}
	return allGaps, nil
}

func (s *NavService) RepairNavGaps(ctx context.Context, schemeCode int, minGapDays int) ([]models.NavGapDto, error) {
	gaps, err := s.DetectNavGaps(ctx, schemeCode, minGapDays)
	if err != nil || len(gaps) == 0 {
		return gaps, err
	}

	for _, g := range gaps {
		from, _ := ParseDate(g.FromDate)
		to, _ := ParseDate(g.ToDate)
		fetched, err := s.amfiService.FetchNavHistory(ctx, schemeCode, from, to)
		if err == nil && len(fetched) > 0 {
			s.saveNavHistoriesAndSyncTracker(ctx, schemeCode, fetched)
		}
	}

	return s.DetectNavGaps(ctx, schemeCode, minGapDays)
}

func (s *NavService) EnsureNavHistories(ctx context.Context, schemeCodes []int, start, end time.Time) (map[int][]models.NavHistory, error) {
	res := make(map[int][]models.NavHistory)
	for _, sc := range schemeCodes {
		_, _ = s.EnsureHistoricalRange(ctx, sc, &start, &end)
		var navs []models.NavHistory
		_ = s.db.WithContext(ctx).
			Where("scheme_code = ? AND date >= ? AND date <= ?", sc, start.AddDate(0, 0, -30), end.AddDate(0, 0, 30)).
			Order("date asc").
			Find(&navs).Error
		res[sc] = navs
	}
	return res, nil
}

// EnsureHistoricalRange inspects local DB coverage for schemeCode.
// It uses singleflight deduplication to collapse any concurrent requests for the same fund,
// runs ingestion under a detached 2-minute background context so client disconnections never abort in-flight work,
// and saves data incrementally ("store as we go") as each 2-year chunk arrives from AMFI.
// If local DB already contains complete coverage, it bypasses AMFI completely (0 HTTP calls).
func (s *NavService) EnsureHistoricalRange(ctx context.Context, schemeCode int, from, to *time.Time) (bool, error) {
	if schemeCode <= 0 {
		return false, fmt.Errorf("invalid scheme code %d", schemeCode)
	}

	// Detached context: Ingestion must not be killed if the client cancels or switches tabs.
	// We run ingestion under a detached background context with a generous 2-minute deadline.
	detachedCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Singleflight deduplication: collapse concurrent calls for the same scheme into 1 execution
	sfKey := fmt.Sprintf("nav-sync:%d", schemeCode)
	val, err, _ := s.sfGroup.Do(sfKey, func() (interface{}, error) {
		return s.doEnsureHistoricalRange(detachedCtx, schemeCode, from, to)
	})
	if err != nil {
		return false, err
	}
	return val.(bool), nil
}

func (s *NavService) doEnsureHistoricalRange(ctx context.Context, schemeCode int, from, to *time.Time) (bool, error) {
	now := IndiaToday()
	effectiveTo := now
	if to != nil && !to.IsZero() {
		effectiveTo = *to
	}
	effectiveFrom := now.AddDate(-30, 0, 0)
	if from != nil && !from.IsZero() {
		effectiveFrom = *from
	}

	var agg struct {
		MinDate *string `gorm:"column:min_date"`
		MaxDate *string `gorm:"column:max_date"`
		Count   int64   `gorm:"column:count"`
	}
	_ = s.db.WithContext(ctx).Model(&models.NavHistory{}).
		Where("scheme_code = ?", schemeCode).
		Select("MIN(date) as min_date, MAX(date) as max_date, COUNT(*) as count").
		Scan(&agg).Error

	var minDate, maxDate *time.Time
	if agg.MinDate != nil && *agg.MinDate != "" {
		if t, err := ParseDate(*agg.MinDate); err == nil {
			minDate = &t
		}
	}
	if agg.MaxDate != nil && *agg.MaxDate != "" {
		if t, err := ParseDate(*agg.MaxDate); err == nil {
			maxDate = &t
		}
	}

	var tracker models.NavSyncTracker
	_ = s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&tracker).Error

	wasFetched := false

	// Incremental chunk saver: "store as we go"
	// As soon as each chunk arrives from AMFI, it is immediately committed to SQLite and NavSyncTracker updated
	saver := func(chunkRecords []models.NavHistory) {
		s.saveNavHistoriesAndSyncTracker(ctx, schemeCode, chunkRecords)
	}

	// Case 1: Database has <= 1 record (e.g. only 1 daily snapshot or completely empty)
	if agg.Count <= 1 {
		fetched, err := s.amfiService.FetchNavHistory(ctx, schemeCode, effectiveFrom, effectiveTo, saver)
		if err == nil && len(fetched) > 0 {
			wasFetched = true
			if from == nil {
				_ = s.db.WithContext(ctx).Model(&models.NavSyncTracker{}).Where("scheme_code = ?", schemeCode).Update("inception_reached", true).Error
			}
		}
		return wasFetched, nil
	}

	// Case 2: Database already has history. Only fetch missing delta intervals!
	var missingRanges [][2]time.Time

	// 2a. Missing older prefix delta: if effectiveFrom is before our earliest recorded NAV and inception not yet marked
	if from != nil && !tracker.InceptionReached && minDate != nil && effectiveFrom.Before(*minDate) {
		deltaTo := minDate.AddDate(0, 0, -1)
		if !deltaTo.Before(effectiveFrom) {
			missingRanges = append(missingRanges, [2]time.Time{effectiveFrom, deltaTo})
		}
	}

	// 2b. Missing newer suffix delta: if effectiveTo is after our latest recorded NAV
	if maxDate != nil && effectiveTo.After(*maxDate) && maxDate.Before(now) {
		deltaFrom := maxDate.AddDate(0, 0, 1)
		if !deltaFrom.After(effectiveTo) {
			missingRanges = append(missingRanges, [2]time.Time{deltaFrom, effectiveTo})
		}
	}

	// If no missing ranges, DB already covers the entire requested window!
	// ZERO calls to AMFI!
	if len(missingRanges) == 0 {
		return false, nil
	}

	// Fetch strictly the missing delta intervals, saving incrementally as we go
	for _, rng := range missingRanges {
		fetched, err := s.amfiService.FetchNavHistory(ctx, schemeCode, rng[0], rng[1], saver)
		if err == nil && len(fetched) > 0 {
			wasFetched = true
		}
		if minDate != nil && rng[1].Before(*minDate) {
			// Prefix interval reached inception limit; mark so we never re-scan pre-inception dates
			_ = s.db.WithContext(ctx).Model(&models.NavSyncTracker{}).Where("scheme_code = ?", schemeCode).Update("inception_reached", true).Error
		}
	}

	return wasFetched, nil
}

func (s *NavService) saveNavHistoriesAndSyncTracker(ctx context.Context, schemeCode int, fetched []models.NavHistory) {
	if len(fetched) == 0 {
		return
	}
	const batchSize = 1000
	for i := 0; i < len(fetched); i += batchSize {
		end := i + batchSize
		if end > len(fetched) {
			end = len(fetched)
		}
		_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(fetched[i:end]).Error
	}
	s.updateNavSyncTracker(ctx, schemeCode, fetched)
}

func (s *NavService) updateNavSyncTracker(ctx context.Context, schemeCode int, _ []models.NavHistory) {
	var agg struct {
		MinDate *string `gorm:"column:min_date"`
		MaxDate *string `gorm:"column:max_date"`
		Count   int64   `gorm:"column:count"`
	}
	_ = s.db.WithContext(ctx).Model(&models.NavHistory{}).
		Where("scheme_code = ?", schemeCode).
		Select("MIN(date) as min_date, MAX(date) as max_date, COUNT(*) as count").
		Scan(&agg).Error

	if agg.MaxDate == nil || *agg.MaxDate == "" || agg.Count == 0 {
		return
	}

	var minDate, maxDate *time.Time
	if agg.MinDate != nil && *agg.MinDate != "" {
		if t, err := ParseDate(*agg.MinDate); err == nil {
			minDate = &t
		}
	}
	if agg.MaxDate != nil && *agg.MaxDate != "" {
		if t, err := ParseDate(*agg.MaxDate); err == nil {
			maxDate = &t
		}
	}

	if maxDate == nil {
		return
	}

	var latestNav float64
	_ = s.db.WithContext(ctx).Model(&models.NavHistory{}).
		Where("scheme_code = ? AND date = ?", schemeCode, *maxDate).
		Select("nav").
		Scan(&latestNav).Error

	nowIst := IndiaNow()
	nowUtc := time.Now().UTC()

	tracker := models.NavSyncTracker{
		SchemeCode:              schemeCode,
		NavStartDate:            minDate,
		NavEndDate:              maxDate,
		LatestNav:               &latestNav,
		LatestNavDate:           maxDate,
		NavHistoryCount:         int(agg.Count),
		LastLatestNavCheckAtIst: &nowIst,
		UpdatedAtUtc:            nowUtc,
	}

	_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "scheme_code"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"nav_start_date",
			"nav_end_date",
			"latest_nav",
			"latest_nav_date",
			"nav_history_count",
			"last_latest_nav_check_at_ist",
			"updated_at",
		}),
	}).Create(&tracker).Error
}

