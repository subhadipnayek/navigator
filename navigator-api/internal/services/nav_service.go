package services

import (
	"context"
	"fmt"
	"sort"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NavService struct {
	db          *gorm.DB
	amfiService *AmfiService
	caService   *CorporateActionService
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

	// If no records in DB, try fetching on-demand from AMFI
	if len(navs) == 0 {
		now := IndiaToday()
		fiveYearsAgo := now.AddDate(-5, 0, 0)
		fetched, err := s.amfiService.FetchNavHistory(ctx, schemeCode, fiveYearsAgo, now)
		if err == nil && len(fetched) > 0 {
			_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&fetched).Error
			navs = fetched
		}
	}

	var caActions []models.CorporateActionDto
	if adjusted {
		caActions, _ = s.caService.GetActions(ctx, schemeCode)
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
			Date: n.Date,
			Nav:  roundFloat(navValue, 4),
		})
	}

	// Interval sampling
	dataPoints = SampleNavDataPoints(dataPoints, interval)

	return &models.NavDataResponse{
		SchemeCode: fund.SchemeCode,
		SchemeName: fund.SchemeName,
		FundHouse:  fund.FundHouse,
		Category:   fund.SchemeCategory,
		Data:       dataPoints,
	}, nil
}

func (s *NavService) GetReturns(ctx context.Context, schemeCode int, startDate, endDate time.Time) (*models.ReturnsResponse, error) {
	var fund models.Fund
	if err := s.db.WithContext(ctx).Where("scheme_code = ?", schemeCode).First(&fund).Error; err != nil {
		return nil, err
	}

	var navs []models.NavHistory
	if err := s.db.WithContext(ctx).Where("scheme_code = ? AND date >= ? AND date <= ?", schemeCode, startDate.AddDate(0, 0, -10), endDate.AddDate(0, 0, 10)).Order("date asc").Find(&navs).Error; err != nil || len(navs) == 0 {
		return nil, fmt.Errorf("no NAV history available for scheme %d", schemeCode)
	}

	startEntry := FindNavOnOrAfter(navs, startDate, 7, true)
	endEntry := FindNavOnOrBefore(navs, endDate)

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
		StartDate:             FormatDateDDMMYYYY(startEntry.Date),
		EndDate:               FormatDateDDMMYYYY(endEntry.Date),
		StartNav:              roundFloat(startEntry.Nav, 4),
		EndNav:                roundFloat(endEntry.Nav, 4),
		AbsoluteReturnPercent: roundFloat(absReturn, 2),
		Cagr:                  cagrVal,
		Days:                  days,
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
			_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&fetched).Error
		}
	}

	return s.DetectNavGaps(ctx, schemeCode, minGapDays)
}

func (s *NavService) EnsureNavHistories(ctx context.Context, schemeCodes []int, start, end time.Time) (map[int][]models.NavHistory, error) {
	res := make(map[int][]models.NavHistory)
	for _, sc := range schemeCodes {
		var navs []models.NavHistory
		_ = s.db.WithContext(ctx).
			Where("scheme_code = ? AND date >= ? AND date <= ?", sc, start.AddDate(0, 0, -30), end.AddDate(0, 0, 30)).
			Order("date asc").
			Find(&navs).Error

		if len(navs) == 0 {
			fetched, err := s.amfiService.FetchNavHistory(ctx, sc, start, end)
			if err == nil && len(fetched) > 0 {
				_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&fetched).Error
				navs = fetched
			}
		}
		res[sc] = navs
	}
	return res, nil
}
