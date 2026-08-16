package services

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type UserPortfolioService struct {
	db     *gorm.DB
	navSvc *NavService
}

func NewUserPortfolioService(db *gorm.DB, nav *NavService) *UserPortfolioService {
	return &UserPortfolioService{db: db, navSvc: nav}
}

var buyTypes = map[string]bool{
	"buy":                  true,
	"purchase":             true,
	"sip":                  true,
	"switchin":             true,
	"dividendreinvestment": true,
}

var sellTypes = map[string]bool{
	"sell":       true,
	"redeem":     true,
	"redemption": true,
	"switchout":  true,
}

func isBuyType(t string) bool {
	return buyTypes[strings.ToLower(strings.TrimSpace(t))]
}

func isSellType(t string) bool {
	return sellTypes[strings.ToLower(strings.TrimSpace(t))]
}

func (s *UserPortfolioService) GetPortfolios(ctx context.Context, userId int) ([]models.UserPortfolioSummaryDto, error) {
	var portfolios []models.UserPortfolio
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("name asc").
		Find(&portfolios).Error
	if err != nil {
		return nil, err
	}

	var res []models.UserPortfolioSummaryDto
	for _, p := range portfolios {
		var fundCount int64
		_ = s.db.WithContext(ctx).Model(&models.PortfolioTransaction{}).
			Where("portfolio_id = ?", p.Id).
			Distinct("scheme_code").
			Count(&fundCount).Error

		res = append(res, models.UserPortfolioSummaryDto{
			Id:           p.Id,
			Name:         p.Name,
			FundCount:    int(fundCount),
			UpdatedAtUtc: p.UpdatedAtUtc,
		})
	}

	return res, nil
}

func (s *UserPortfolioService) CreatePortfolio(ctx context.Context, userId int, req *models.CreateUserPortfolioRequest) (*models.UserPortfolioSummaryDto, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "My Portfolio"
	}

	portfolio := models.UserPortfolio{
		UserId:       userId,
		Name:         name,
		CreatedAtUtc: time.Now().UTC(),
		UpdatedAtUtc: time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(&portfolio).Error; err != nil {
		return nil, err
	}

	return &models.UserPortfolioSummaryDto{
		Id:           portfolio.Id,
		Name:         portfolio.Name,
		FundCount:    0,
		UpdatedAtUtc: portfolio.UpdatedAtUtc,
	}, nil
}

func (s *UserPortfolioService) GetDashboard(
	ctx context.Context,
	userId int,
	portfolioId int64,
	showFullyExitedFunds bool,
) (*models.PortfolioDashboardDto, error) {
	var portfolio models.UserPortfolio
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", portfolioId, userId).First(&portfolio).Error; err != nil {
		return nil, err
	}

	var transactions []models.PortfolioTransaction
	err := s.db.WithContext(ctx).
		Preload("Fund").
		Where("portfolio_id = ?", portfolioId).
		Order("transaction_date asc, id asc").
		Find(&transactions).Error
	if err != nil {
		return nil, err
	}

	var emptyStates []string
	if len(transactions) == 0 {
		emptyStates = append(emptyStates, "No transactions have been added yet.")
	}

	// Group transactions by fund
	fundTxMap := make(map[int][]models.PortfolioTransaction)
	fundMap := make(map[int]*models.Fund)
	for _, tx := range transactions {
		fundTxMap[tx.SchemeCode] = append(fundTxMap[tx.SchemeCode], tx)
		if tx.Fund != nil {
			fundMap[tx.SchemeCode] = tx.Fund
		}
	}

	// Fetch latest NAVs
	var schemeCodes []int
	for sc := range fundTxMap {
		schemeCodes = append(schemeCodes, sc)
	}

	var trackers []models.NavSyncTracker
	_ = s.db.WithContext(ctx).Where("scheme_code IN ?", schemeCodes).Find(&trackers).Error
	latestNavMap := make(map[int]models.NavSyncTracker)
	for _, t := range trackers {
		latestNavMap[t.SchemeCode] = t
	}

	var fundInsights []models.FundInsightDto
	var allCashFlows []CashFlow
	totalInvested := 0.0
	totalCurrentVal := 0.0
	totalRealized := 0.0
	activeCount := 0
	exitedCount := 0

	for sc, txList := range fundTxMap {
		var fundName, fundHouse, category, assetClass string
		if f, ok := fundMap[sc]; ok && f != nil {
			fundName = f.SchemeName
			if f.FundHouse != nil {
				fundHouse = *f.FundHouse
			}
			if f.SchemeCategory != nil {
				category = *f.SchemeCategory
			}
			if f.SchemeType != nil {
				assetClass = *f.SchemeType
			}
		}
		if fundName == "" {
			fundName = fmt.Sprintf("Fund %d", sc)
		}
		if assetClass == "" {
			assetClass = "Equity"
		}

		unitsHeld := 0.0
		investedAmt := 0.0
		realizedGain := 0.0
		var fundCashFlows []CashFlow

		for _, tx := range txList {
			amt := tx.Amount
			if amt <= 0 {
				amt = tx.Units * tx.Nav
			}

			if isBuyType(tx.TransactionType) {
				unitsHeld += tx.Units
				investedAmt += amt
				fundCashFlows = append(fundCashFlows, CashFlow{Date: tx.TransactionDate, Amount: -amt})
				allCashFlows = append(allCashFlows, CashFlow{Date: tx.TransactionDate, Amount: -amt})
			} else if isSellType(tx.TransactionType) {
				// Weighted average cost basis
				avgCost := 0.0
				if unitsHeld > 0 {
					avgCost = investedAmt / unitsHeld
				}
				costSold := tx.Units * avgCost
				gain := amt - costSold
				realizedGain += gain

				unitsHeld -= tx.Units
				investedAmt -= costSold
				if investedAmt < 0 {
					investedAmt = 0
				}
				if unitsHeld < 0 {
					unitsHeld = 0
				}

				fundCashFlows = append(fundCashFlows, CashFlow{Date: tx.TransactionDate, Amount: amt})
				allCashFlows = append(allCashFlows, CashFlow{Date: tx.TransactionDate, Amount: amt})
			}
		}

		latestNav := 0.0
		latestNavDateStr := ""
		if t, ok := latestNavMap[sc]; ok {
			if t.LatestNav != nil {
				latestNav = *t.LatestNav
			}
			if t.LatestNavDate != nil {
				latestNavDateStr = FormatDateDDMMYYYY(*t.LatestNavDate)
			}
		}

		currentVal := unitsHeld * latestNav
		unrealizedGain := currentVal - investedAmt
		totalGain := realizedGain + unrealizedGain
		gainPct := 0.0
		if investedAmt > 0 {
			gainPct = roundFloat(unrealizedGain/investedAmt*100.0, 2)
		}

		isExited := unitsHeld < 0.0001
		if isExited {
			exitedCount++
		} else {
			activeCount++
		}

		if currentVal > 0 {
			fundCashFlows = append(fundCashFlows, CashFlow{Date: IndiaToday(), Amount: currentVal})
		}
		fundXirr := CalculateXIRR(fundCashFlows)

		avgNav := 0.0
		if unitsHeld > 0 && investedAmt > 0 {
			avgNav = roundFloat(investedAmt/unitsHeld, 4)
		}

		insight := models.FundInsightDto{
			SchemeCode:     sc,
			SchemeName:     fundName,
			FundHouse:      fundHouse,
			AssetClass:     assetClass,
			Category:       category,
			UnitsHeld:      roundFloat(unitsHeld, 4),
			InvestedAmount: roundFloat(investedAmt, 2),
			CurrentValue:   roundFloat(currentVal, 2),
			TotalGain:      roundFloat(totalGain, 2),
			GainPercentage: gainPct,
			RealizedGain:   roundFloat(realizedGain, 2),
			UnrealizedGain: roundFloat(unrealizedGain, 2),
			Xirr:           fundXirr,
			AverageNav:     avgNav,
			LatestNav:      roundFloat(latestNav, 4),
			LatestNavDate:  latestNavDateStr,
			IsFullyExited:  isExited,
		}

		fundInsights = append(fundInsights, insight)
		totalInvested += investedAmt
		totalCurrentVal += currentVal
		totalRealized += realizedGain
	}

	sort.Slice(fundInsights, func(i, j int) bool {
		return fundInsights[i].CurrentValue > fundInsights[j].CurrentValue
	})

	var visibleFunds []models.FundInsightDto
	for _, fi := range fundInsights {
		if showFullyExitedFunds || !fi.IsFullyExited {
			visibleFunds = append(visibleFunds, fi)
		}
	}

	totalUnrealized := totalCurrentVal - totalInvested
	totalGain := totalRealized + totalUnrealized
	portGainPct := 0.0
	if totalInvested > 0 {
		portGainPct = roundFloat(totalUnrealized/totalInvested*100.0, 2)
	}

	if totalCurrentVal > 0 {
		allCashFlows = append(allCashFlows, CashFlow{Date: IndiaToday(), Amount: totalCurrentVal})
	}
	portXirr := CalculateXIRR(allCashFlows)

	metrics := models.PortfolioMetricsDto{
		InvestedAmount:        roundFloat(totalInvested, 2),
		CurrentValue:          roundFloat(totalCurrentVal, 2),
		TotalGain:             roundFloat(totalGain, 2),
		GainPercentage:        portGainPct,
		RealizedGain:          roundFloat(totalRealized, 2),
		UnrealizedGain:        roundFloat(totalUnrealized, 2),
		Xirr:                  portXirr,
		ActiveFundsCount:      activeCount,
		FullyExitedFundsCount: exitedCount,
	}

	// Allocations
	assetAlloc := buildAllocationBreakdown(visibleFunds, func(f models.FundInsightDto) string { return f.AssetClass })
	fundAlloc := buildAllocationBreakdown(visibleFunds, func(f models.FundInsightDto) string { return f.SchemeName })

	fyInflows := buildContributionBreakdown(transactions, true)
	cyInflows := buildContributionBreakdown(transactions, false)
	sipConsistency := buildSipConsistency(transactions)

	return &models.PortfolioDashboardDto{
		Id:                   portfolio.Id,
		Name:                 portfolio.Name,
		Metrics:              metrics,
		Funds:                visibleFunds,
		AssetAllocation:      assetAlloc,
		FundAllocation:       fundAlloc,
		FinancialYearInflows: fyInflows,
		CalendarYearInflows:  cyInflows,
		SipConsistency:       sipConsistency,
		EmptyStates:          emptyStates,
	}, nil
}

func (s *UserPortfolioService) GetTransactions(ctx context.Context, userId int, portfolioId int64, financialYear string) ([]models.PortfolioTransactionDto, *models.PortfolioTransactionsSummaryDto, error) {
	var portfolio models.UserPortfolio
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", portfolioId, userId).First(&portfolio).Error; err != nil {
		return nil, nil, err
	}

	query := s.db.WithContext(ctx).
		Preload("Fund").
		Where("portfolio_id = ?", portfolioId)

	if financialYear != "" {
		// FY 2023-24: 01-04-2023 to 31-03-2024
		parts := strings.Split(financialYear, "-")
		if len(parts) == 2 {
			if startYear, err := strconv.Atoi(parts[0]); err == nil {
				start := time.Date(startYear, 4, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(startYear+1, 3, 31, 23, 59, 59, 0, time.UTC)
				query = query.Where("transaction_date >= ? AND transaction_date <= ?", start, end)
			}
		}
	}

	var rows []models.PortfolioTransaction
	if err := query.Order("transaction_date desc, id desc").Find(&rows).Error; err != nil {
		return nil, nil, err
	}

	var dtos []models.PortfolioTransactionDto
	totalPurchases := 0.0
	totalRedemptions := 0.0

	for _, tx := range rows {
		name := ""
		if tx.Fund != nil {
			name = tx.Fund.SchemeName
		}
		if name == "" {
			name = fmt.Sprintf("Scheme %d", tx.SchemeCode)
		}

		dtos = append(dtos, models.PortfolioTransactionDto{
			Id:              tx.Id,
			SchemeCode:      tx.SchemeCode,
			SchemeName:      name,
			TransactionDate: FormatDateDDMMYYYY(tx.TransactionDate),
			TransactionType: tx.TransactionType,
			Units:           tx.Units,
			Nav:             tx.Nav,
			Amount:          tx.Amount,
			Notes:           tx.Notes,
			Isin:            tx.Isin,
		})

		if isBuyType(tx.TransactionType) {
			totalPurchases += tx.Amount
		} else if isSellType(tx.TransactionType) {
			totalRedemptions += tx.Amount
		}
	}

	summary := &models.PortfolioTransactionsSummaryDto{
		TotalPurchases:    roundFloat(totalPurchases, 2),
		TotalRedemptions:  roundFloat(totalRedemptions, 2),
		NetInvestedAmount: roundFloat(totalPurchases-totalRedemptions, 2),
		TransactionCount:  len(dtos),
	}

	return dtos, summary, nil
}

func (s *UserPortfolioService) SaveTransactions(
	ctx context.Context,
	userId int,
	portfolioId int64,
	req *models.SavePortfolioTransactionsRequest,
) (*models.SavePortfolioTransactionsResponse, error) {
	var portfolio models.UserPortfolio
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", portfolioId, userId).First(&portfolio).Error; err != nil {
		return nil, err
	}

	var errors []models.TransactionValidationIssueDto
	var toInsert []models.PortfolioTransaction

	for idx, tx := range req.Transactions {
		txDate, err := ParseDate(tx.TransactionDate)
		if err != nil {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: idx + 1,
				Field:     "TransactionDate",
				Message:   "Invalid date format. Use dd-MM-yyyy.",
			})
			continue
		}

		if tx.SchemeCode <= 0 {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: idx + 1,
				Field:     "SchemeCode",
				Message:   "Scheme code is required and must be positive.",
			})
			continue
		}

		if tx.Units <= 0 {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: idx + 1,
				Field:     "Units",
				Message:   "Units must be positive.",
			})
			continue
		}

		amt := tx.Amount
		if amt <= 0 {
			amt = tx.Units * tx.Nav
		}

		toInsert = append(toInsert, models.PortfolioTransaction{
			PortfolioId:     portfolioId,
			SchemeCode:      tx.SchemeCode,
			TransactionDate: txDate,
			TransactionType: tx.TransactionType,
			Units:           tx.Units,
			Nav:             tx.Nav,
			Amount:          amt,
			Notes:           tx.Notes,
			Isin:            tx.Isin,
			CreatedAtUtc:    time.Now().UTC(),
			UpdatedAtUtc:    time.Now().UTC(),
		})
	}

	if len(errors) > 0 {
		return &models.SavePortfolioTransactionsResponse{
			IsValid: false,
			Errors:  errors,
		}, nil
	}

	// Persist
	err := s.db.WithContext(ctx).Transaction(func(txDb *gorm.DB) error {
		for _, entity := range toInsert {
			if err := txDb.Create(&entity).Error; err != nil {
				return err
			}
		}
		// Invalidate cache
		_ = txDb.Where("portfolio_id = ?", portfolioId).Delete(&models.UserPortfolioDashboardCache{}).Error
		return nil
	})

	if err != nil {
		return nil, err
	}

	savedDtos, summary, _ := s.GetTransactions(ctx, userId, portfolioId, req.FinancialYear)
	return &models.SavePortfolioTransactionsResponse{
		IsValid:      true,
		Summary:      *summary,
		Transactions: savedDtos,
	}, nil
}

func (s *UserPortfolioService) ImportTransactions(
	ctx context.Context,
	userId int,
	portfolioId int64,
	financialYear, importMode string,
	startDate, endDate time.Time,
	r io.Reader,
	fileName string,
) (*models.SavePortfolioTransactionsResponse, error) {
	drafts, err := ParseTransactionExcel(r)
	if err != nil {
		return nil, err
	}

	var errors []models.TransactionValidationIssueDto
	var toInsert []models.PortfolioTransaction

	for _, d := range drafts {
		txDate, err := ParseDate(d.TransactionDate)
		if err != nil {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: d.RowNumber,
				Field:     "TransactionDate",
				Message:   fmt.Sprintf("Invalid date '%s'.", d.TransactionDate),
			})
			continue
		}

		code, _ := strconv.Atoi(d.SchemeCode)
		if code <= 0 && d.Isin != "" {
			// Try lookup fund by ISIN
			var fund models.Fund
			if err := s.db.WithContext(ctx).Where("isin_growth = ? OR isin_div_reinvestment = ?", d.Isin, d.Isin).First(&fund).Error; err == nil {
				code = fund.SchemeCode
			}
		}

		if code <= 0 {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: d.RowNumber,
				Field:     "SchemeCode",
				Message:   fmt.Sprintf("Unknown SchemeCode '%s' / ISIN '%s'.", d.SchemeCode, d.Isin),
			})
			continue
		}

		units := ParseFloatOrZero(d.Units)
		nav := ParseFloatOrZero(d.Nav)
		if units <= 0 {
			errors = append(errors, models.TransactionValidationIssueDto{
				RowNumber: d.RowNumber,
				Field:     "Units",
				Message:   "Units must be positive.",
			})
			continue
		}

		txType := d.TransactionType
		if txType == "" {
			txType = "Purchase"
		}

		var notesPtr, isinPtr *string
		if d.Notes != "" {
			n := d.Notes
			notesPtr = &n
		}
		if d.Isin != "" {
			i := d.Isin
			isinPtr = &i
		}

		toInsert = append(toInsert, models.PortfolioTransaction{
			PortfolioId:     portfolioId,
			SchemeCode:      code,
			TransactionDate: txDate,
			TransactionType: txType,
			Units:           units,
			Nav:             nav,
			Amount:          units * nav,
			Notes:           notesPtr,
			Isin:            isinPtr,
			CreatedAtUtc:    time.Now().UTC(),
			UpdatedAtUtc:    time.Now().UTC(),
		})
	}

	if len(errors) > 0 {
		return &models.SavePortfolioTransactionsResponse{
			IsValid: false,
			Errors:  errors,
		}, nil
	}

	err = s.db.WithContext(ctx).Transaction(func(txDb *gorm.DB) error {
		if strings.EqualFold(importMode, "Replace") && !startDate.IsZero() && !endDate.IsZero() {
			_ = txDb.Where("portfolio_id = ? AND transaction_date >= ? AND transaction_date <= ?", portfolioId, startDate, endDate).
				Delete(&models.PortfolioTransaction{}).Error
		}

		if len(toInsert) > 0 {
			if err := txDb.Create(&toInsert).Error; err != nil {
				return err
			}
		}

		_ = txDb.Where("portfolio_id = ?", portfolioId).Delete(&models.UserPortfolioDashboardCache{}).Error
		return nil
	})

	if err != nil {
		return nil, err
	}

	savedDtos, summary, _ := s.GetTransactions(ctx, userId, portfolioId, financialYear)
	return &models.SavePortfolioTransactionsResponse{
		IsValid:      true,
		Summary:      *summary,
		Transactions: savedDtos,
	}, nil
}

func (s *UserPortfolioService) GetImportDeletePreview(
	ctx context.Context,
	userId int,
	portfolioId int64,
	startDate, endDate time.Time,
) (int, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.PortfolioTransaction{}).
		Where("portfolio_id = ? AND transaction_date >= ? AND transaction_date <= ?", portfolioId, startDate, endDate).
		Count(&count).Error
	return int(count), err
}

func (s *UserPortfolioService) CompareBenchmark(
	ctx context.Context,
	userId int,
	portfolioId int64,
	req *models.PortfolioBenchmarkRequest,
) (*models.PortfolioBenchmarkComparisonDto, error) {
	dash, err := s.GetDashboard(ctx, userId, portfolioId, true)
	if err != nil {
		return nil, err
	}

	var benchmarks []models.BenchmarkPerformanceDto
	for _, sc := range req.BenchmarkSchemeCodes {
		var fund models.Fund
		if err := s.db.WithContext(ctx).Where("scheme_code = ?", sc).First(&fund).Error; err != nil {
			continue
		}

		// Calculate CAGR / return
		res, err := s.navSvc.GetRollingReturns(ctx, sc, 1, nil, nil, "daily")
		var bXirr *float64
		if err == nil && res.AverageReturn != nil {
			bXirr = res.AverageReturn
		}

		var alpha *float64
		if dash.Metrics.Xirr != nil && bXirr != nil {
			a := roundFloat(*dash.Metrics.Xirr-*bXirr, 2)
			alpha = &a
		}

		benchmarks = append(benchmarks, models.BenchmarkPerformanceDto{
			SchemeCode:    sc,
			SchemeName:    fund.SchemeName,
			BenchmarkXirr: bXirr,
			FinalValue:    dash.Metrics.CurrentValue,
			AlphaVsXirr:   alpha,
		})
	}

	return &models.PortfolioBenchmarkComparisonDto{
		PortfolioName:       dash.Name,
		PortfolioXirr:       dash.Metrics.Xirr,
		PortfolioFinalValue: dash.Metrics.CurrentValue,
		TotalInvested:       dash.Metrics.InvestedAmount,
		EffectiveStartDate:  FormatDateDDMMYYYY(IndiaToday().AddDate(-3, 0, 0)),
		EffectiveEndDate:    FormatDateDDMMYYYY(IndiaToday()),
		Benchmarks:          benchmarks,
		TimeSeries:          []models.AlignedPortfolioSnapshot{},
	}, nil
}

// ─── Helpers ───

func buildAllocationBreakdown(funds []models.FundInsightDto, keySelector func(models.FundInsightDto) string) []models.AllocationSegmentDto {
	totalVal := 0.0
	groupMap := make(map[string]float64)

	for _, f := range funds {
		if f.CurrentValue <= 0 {
			continue
		}
		k := keySelector(f)
		if k == "" {
			k = "Other"
		}
		groupMap[k] += f.CurrentValue
		totalVal += f.CurrentValue
	}

	var segments []models.AllocationSegmentDto
	for label, amt := range groupMap {
		pct := 0.0
		if totalVal > 0 {
			pct = roundFloat(amt/totalVal*100.0, 2)
		}
		segments = append(segments, models.AllocationSegmentDto{
			Label:      label,
			Amount:     roundFloat(amt, 2),
			Percentage: pct,
		})
	}

	sort.Slice(segments, func(i, j int) bool { return segments[i].Amount > segments[j].Amount })
	return segments
}

func buildContributionBreakdown(transactions []models.PortfolioTransaction, financialYear bool) []models.PeriodContributionDto {
	type periodData struct {
		Label      string
		TotalInflow float64
		SipInflow   float64
		LumpInflow  float64
		Outflow     float64
	}

	m := make(map[string]*periodData)
	var keys []string

	for _, tx := range transactions {
		var label string
		if financialYear {
			// e.g. FY 2023-24
			year := tx.TransactionDate.Year()
			if tx.TransactionDate.Month() < 4 {
				year--
			}
			label = fmt.Sprintf("FY %d-%02d", year, (year+1)%100)
		} else {
			label = fmt.Sprintf("%d", tx.TransactionDate.Year())
		}

		if _, exists := m[label]; !exists {
			m[label] = &periodData{Label: label}
			keys = append(keys, label)
		}

		pd := m[label]
		amt := tx.Amount
		if amt <= 0 {
			amt = tx.Units * tx.Nav
		}

		if isBuyType(tx.TransactionType) {
			pd.TotalInflow += amt
			if strings.EqualFold(tx.TransactionType, "SIP") {
				pd.SipInflow += amt
			} else {
				pd.LumpInflow += amt
			}
		} else if isSellType(tx.TransactionType) {
			pd.Outflow += amt
		}
	}

	sort.Strings(keys)
	var res []models.PeriodContributionDto
	for _, k := range keys {
		pd := m[k]
		res = append(res, models.PeriodContributionDto{
			PeriodLabel: pd.Label,
			TotalInflow: roundFloat(pd.TotalInflow, 2),
			SipInflow:   roundFloat(pd.SipInflow, 2),
			LumpInflow:  roundFloat(pd.LumpInflow, 2),
			Outflow:     roundFloat(pd.Outflow, 2),
			NetInflow:   roundFloat(pd.TotalInflow-pd.Outflow, 2),
		})
	}
	return res
}

func buildSipConsistency(transactions []models.PortfolioTransaction) models.SipConsistencyDto {
	var sipTx []models.PortfolioTransaction
	for _, tx := range transactions {
		if strings.EqualFold(tx.TransactionType, "SIP") {
			sipTx = append(sipTx, tx)
		}
	}

	if len(sipTx) == 0 {
		return models.SipConsistencyDto{
			ExpectedMonthlySips: 0,
			CompletedSips:       0,
			MissedSips:          0,
			ConsistencyScore:    100.0,
			MonthlyBreakdown:    []string{},
		}
	}

	sort.Slice(sipTx, func(i, j int) bool { return sipTx[i].TransactionDate.Before(sipTx[j].TransactionDate) })
	start := sipTx[0].TransactionDate
	end := sipTx[len(sipTx)-1].TransactionDate

	months := DaysBetween(start, end)/30 + 1
	completed := len(sipTx)
	missed := 0
	if months > completed {
		missed = months - completed
	}

	score := 100.0
	if months > 0 {
		score = roundFloat(float64(completed)/float64(months)*100.0, 2)
	}

	return models.SipConsistencyDto{
		ExpectedMonthlySips: months,
		CompletedSips:       completed,
		MissedSips:          missed,
		ConsistencyScore:    score,
		MonthlyBreakdown:    []string{},
	}
}
