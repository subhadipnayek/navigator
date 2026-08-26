package importer

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type ImporterService struct {
	db           *gorm.DB
	reader       *TabularReader
	registry     *BrokerRegistry
	resolver     *AssetResolver
	deduplicator *DeduplicationEngine
}

func NewImporterService(db *gorm.DB) *ImporterService {
	return &ImporterService{
		db:           db,
		reader:       NewTabularReader(),
		registry:     NewBrokerRegistry(),
		resolver:     NewAssetResolver(db),
		deduplicator: NewDeduplicationEngine(),
	}
}

// Preview parses an uploaded file stream and generates a complete dry-run diff breakdown without persisting changes.
func (s *ImporterService) Preview(
	ctx context.Context,
	userId int,
	portfolioId int64,
	fileReader io.Reader,
	filename string,
) (*ImportPreviewResult, error) {
	// 1. Verify portfolio ownership
	var portfolio models.UserPortfolio
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", portfolioId, userId).First(&portfolio).Error; err != nil {
		return nil, fmt.Errorf("portfolio not found or unauthorized")
	}

	// 2. Read raw tabular 2D grid
	rows, err := s.reader.Read(fileReader, filename)
	if err != nil {
		return nil, fmt.Errorf("error reading file '%s': %w", filename, err)
	}

	// 3. Detect broker signature and parse drafts
	parser, drafts, err := s.registry.DetectAndParse(rows)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// 4. Resolve Asset codes (AMFI SchemeCode / Stock Symbol)
	for i := range drafts {
		if !drafts[i].IsIgnored {
			s.resolver.Resolve(ctx, &drafts[i])
		}
	}

	// 5. Fetch existing portfolio transactions to compute duplicates
	var existingTx []models.PortfolioTransaction
	_ = s.db.WithContext(ctx).Where("portfolio_id = ?", portfolioId).Find(&existingTx).Error

	// 6. Deduplicate & classify
	processed := s.deduplicator.ClassifyDrafts(portfolioId, drafts, existingTx)

	// 7. Compute high-level KPI metrics & date ranges
	var validCount, newCount, dupCount, ignoredCount, unresCount int
	var minDate, maxDate time.Time
	var totalInflow, totalOutflow float64

	for _, rec := range processed {
		if rec.Status == StatusIgnored {
			ignoredCount++
			continue
		}
		if rec.Status == StatusUnresolved {
			unresCount++
		}

		validCount++
		if rec.Status == StatusNew {
			newCount++
		} else if rec.Status == StatusDuplicate {
			dupCount++
		}

		if !rec.TransactionDate.IsZero() {
			if minDate.IsZero() || rec.TransactionDate.Before(minDate) {
				minDate = rec.TransactionDate
			}
			if maxDate.IsZero() || rec.TransactionDate.After(maxDate) {
				maxDate = rec.TransactionDate
			}
		}

		amt := rec.Amount
		if amt == 0 {
			amt = rec.Units * rec.Price
		}

		lowerType := strings.ToLower(rec.TransactionType)
		if strings.Contains(lowerType, "sell") || strings.Contains(lowerType, "redemption") || strings.Contains(lowerType, "switch out") {
			totalOutflow += amt
		} else {
			totalInflow += amt
		}
	}

	earliestStr := ""
	latestStr := ""
	if !minDate.IsZero() {
		earliestStr = minDate.Format("02-Jan-2006")
	}
	if !maxDate.IsZero() {
		latestStr = maxDate.Format("02-Jan-2006")
	}

	// Sort processed records with newest first
	sort.Slice(processed, func(i, j int) bool {
		return processed[i].TransactionDate.After(processed[j].TransactionDate)
	})

	strategies := []ImportStrategyOption{
		{
			Key:         StrategySmartMerge,
			Title:       "⚡ Smart Merge & Deduplicate",
			Description: fmt.Sprintf("Recommended: Import %d new trades, safely skip %d duplicate trades already recorded.", newCount, dupCount),
			Recommended: true,
		},
		{
			Key:         StrategyAppend,
			Title:       "➕ Append All Verified Trades",
			Description: fmt.Sprintf("Force-insert all %d valid transactions into portfolio.", validCount),
			Recommended: false,
		},
		{
			Key:         StrategyWindowReplace,
			Title:       "🔄 Replace Date Window",
			Description: fmt.Sprintf("Delete existing trades between %s and %s, then insert all %d trades.", earliestStr, latestStr, validCount),
			Recommended: false,
		},
		{
			Key:         StrategyFullOverwrite,
			Title:       "⚠️ Full Portfolio Overwrite",
			Description: "Reset all existing transactions and replace with this file.",
			Recommended: false,
		},
	}

	return &ImportPreviewResult{
		DetectedFormat:          parser.ID(),
		DetectedFormatLabel:     parser.DisplayName(),
		TotalRowsParsed:         len(rows),
		ValidFinancialTrades:    validCount,
		NewTradesCount:          newCount,
		DuplicateTradesCount:    dupCount,
		IgnoredEventsCount:      ignoredCount,
		UnresolvedCount:         unresCount,
		EarliestTransactionDate: earliestStr,
		LatestTransactionDate:   latestStr,
		TotalInflow:             math.Round(totalInflow*100) / 100,
		TotalOutflow:            math.Round(totalOutflow*100) / 100,
		RecommendedStrategy:     StrategySmartMerge,
		Transactions:            processed,
		AvailableStrategies:     strategies,
	}, nil
}

// Commit applies the previewed batch of transactions into PostgreSQL according to the selected strategy.
func (s *ImporterService) Commit(
	ctx context.Context,
	userId int,
	portfolioId int64,
	req *ImportCommitRequest,
) (int, error) {
	var portfolio models.UserPortfolio
	if err := s.db.WithContext(ctx).Where("id = ? AND user_id = ?", portfolioId, userId).First(&portfolio).Error; err != nil {
		return 0, fmt.Errorf("portfolio not found or unauthorized")
	}

	var toInsert []models.PortfolioTransaction
	var minDate, maxDate time.Time

	for _, rec := range req.Transactions {
		if rec.Status == StatusIgnored {
			continue
		}

		// In SmartMerge mode, skip duplicates
		if req.Strategy == StrategySmartMerge && rec.Status == StatusDuplicate {
			continue
		}

		// Re-resolve on the fly if SchemeCode is missing
		if rec.SchemeCode <= 0 {
			s.resolver.Resolve(ctx, &rec.RawTradeDraft)
		}

		// If still missing, check for synthetic code or fallback
		if rec.SchemeCode <= 0 {
			if rec.Symbol != "" || rec.ISIN != "" {
				rec.SchemeCode = generateSyntheticEquityCode(rec.Symbol, rec.ISIN)
			} else if rec.SchemeName != "" {
				rec.SchemeCode = generateSyntheticMutualFundCode(rec.SchemeName, rec.FolioNumber)
			} else {
				continue
			}
		}

		if rec.Fingerprint == "" {
			dateStr := rec.TransactionDate.Format("2006-01-02")
			rec.Fingerprint = s.deduplicator.ComputeFingerprint(
				portfolioId,
				rec.SchemeCode,
				rec.Symbol,
				dateStr,
				rec.TransactionType,
				rec.Units,
				rec.Price,
				rec.TradeID,
				rec.FolioNumber,
				rec.SourceRowNumber,
			)
		}

		if !rec.TransactionDate.IsZero() {
			if minDate.IsZero() || rec.TransactionDate.Before(minDate) {
				minDate = rec.TransactionDate
			}
			if maxDate.IsZero() || rec.TransactionDate.After(maxDate) {
				maxDate = rec.TransactionDate
			}
		}

		amt := rec.Amount
		if amt <= 0 {
			amt = rec.Units * rec.Price
		}

		var notesPtr, isinPtr *string
		if rec.Notes != "" {
			n := rec.Notes
			notesPtr = &n
		}
		if rec.ISIN != "" {
			i := rec.ISIN
			isinPtr = &i
		}

		var tradeIdPtr *string
		if rec.TradeID != "" {
			t := rec.TradeID
			tradeIdPtr = &t
		}

		toInsert = append(toInsert, models.PortfolioTransaction{
			PortfolioId:     portfolioId,
			SchemeCode:      rec.SchemeCode,
			TransactionDate: rec.TransactionDate,
			TransactionType: rec.TransactionType,
			Units:           rec.Units,
			Nav:             rec.Price,
			Amount:          amt,
			Notes:           notesPtr,
			Isin:            isinPtr,
			TradeId:         tradeIdPtr,
			Fingerprint:     rec.Fingerprint,
			CreatedAtUtc:    time.Now().UTC(),
			UpdatedAtUtc:    time.Now().UTC(),
		})
	}

	err := s.db.WithContext(ctx).Transaction(func(txDb *gorm.DB) error {
		switch req.Strategy {
		case StrategyFullOverwrite:
			if err := txDb.Where("portfolio_id = ?", portfolioId).Delete(&models.PortfolioTransaction{}).Error; err != nil {
				return err
			}
		case StrategyWindowReplace:
			if !minDate.IsZero() && !maxDate.IsZero() {
				if err := txDb.Where("portfolio_id = ? AND transaction_date >= ? AND transaction_date <= ?", portfolioId, minDate, maxDate).
					Delete(&models.PortfolioTransaction{}).Error; err != nil {
					return err
				}
			}
		}

		if len(toInsert) > 0 {
			// Insert in batches of 100
			if err := txDb.CreateInBatches(&toInsert, 100).Error; err != nil {
				return err
			}
		}

		// Invalidate portfolio dashboard cache
		_ = txDb.Where("portfolio_id = ?", portfolioId).Delete(&models.UserPortfolioDashboardCache{}).Error
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("transaction commit failed: %w", err)
	}

	return len(toInsert), nil
}
