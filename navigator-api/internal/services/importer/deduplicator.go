package importer

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"

	"navigator-api/internal/models"
)

type DeduplicationEngine struct{}

func NewDeduplicationEngine() *DeduplicationEngine {
	return &DeduplicationEngine{}
}

// ComputeFingerprint generates a deterministic unique hash for a trade.
func (d *DeduplicationEngine) ComputeFingerprint(portfolioID int64, schemeCode int, symbol string, dateStr string, txType string, units float64, price float64, tradeID string) string {
	if tradeID != "" {
		raw := fmt.Sprintf("%d|%s|%s|%s", portfolioID, symbol, dateStr, tradeID)
		hash := sha256.Sum256([]byte(raw))
		return fmt.Sprintf("%x", hash)
	}

	normType := strings.ToLower(strings.TrimSpace(txType))
	raw := fmt.Sprintf("%d|%d|%s|%s|%.4f|%.4f",
		portfolioID,
		schemeCode,
		dateStr,
		normType,
		math.Abs(units),
		math.Abs(price),
	)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}

// ClassifyDrafts compares parsed drafts against existing portfolio records to identify new vs duplicate vs ignored rows.
func (d *DeduplicationEngine) ClassifyDrafts(
	portfolioID int64,
	drafts []RawTradeDraft,
	existingTransactions []models.PortfolioTransaction,
) []ProcessedTradeRecord {
	// Build map of existing fingerprints
	existingMap := make(map[string]bool)
	for _, tx := range existingTransactions {
		dateStr := tx.TransactionDate.Format("2006-01-02")
		fp := d.ComputeFingerprint(portfolioID, tx.SchemeCode, "", dateStr, tx.TransactionType, tx.Units, tx.Nav, "")
		existingMap[fp] = true
	}

	// Track within-batch duplicates
	batchSeenMap := make(map[string]bool)

	var results []ProcessedTradeRecord

	for _, draft := range drafts {
		if draft.IsIgnored {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusIgnored,
				StatusMessage: draft.IgnoreReason,
				Fingerprint:   "",
			})
			continue
		}

		if draft.TransactionDate.IsZero() {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusInvalid,
				StatusMessage: "Missing or unparseable transaction date",
				Fingerprint:   "",
			})
			continue
		}

		dateStr := draft.TransactionDate.Format("2006-01-02")
		fp := d.ComputeFingerprint(
			portfolioID,
			draft.SchemeCode,
			draft.Symbol,
			dateStr,
			draft.TransactionType,
			draft.Units,
			draft.Price,
			draft.TradeID,
		)

		// Check if unresolved
		if draft.SchemeCode <= 0 && draft.Symbol == "" {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusUnresolved,
				StatusMessage: fmt.Sprintf("Scheme / ISIN '%s' could not be resolved to a mutual fund or stock", draft.ISIN),
				Fingerprint:   fp,
			})
			continue
		}

		// Check if duplicate in database or duplicate within the same uploaded file
		if existingMap[fp] {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusDuplicate,
				StatusMessage: "Transaction already exists in this portfolio (skipped in Smart Merge)",
				Fingerprint:   fp,
			})
			continue
		}

		if batchSeenMap[fp] {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusDuplicate,
				StatusMessage: "Duplicate row within the same uploaded file (skipped)",
				Fingerprint:   fp,
			})
			continue
		}

		batchSeenMap[fp] = true

		results = append(results, ProcessedTradeRecord{
			RawTradeDraft: draft,
			Status:        StatusNew,
			StatusMessage: "New verified trade (will be imported)",
			Fingerprint:   fp,
		})
	}

	return results
}
