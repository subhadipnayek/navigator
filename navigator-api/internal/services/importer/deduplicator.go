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

// ComputeSignature creates a normalized base key for grouping non-TradeID transactions.
func (d *DeduplicationEngine) ComputeSignature(portfolioID int64, schemeCode int, dateStr string, txType string, units float64, price float64) string {
	normType := strings.ToLower(strings.TrimSpace(txType))
	return fmt.Sprintf("%d|%d|%s|%s|%.4f|%.4f",
		portfolioID,
		schemeCode,
		dateStr,
		normType,
		math.Abs(units),
		math.Abs(price),
	)
}

// ComputeFingerprint generates a deterministic unique hash for a trade.
// If TradeID is provided (e.g. Zerodha exchange fill), TradeID is hashed directly.
// If TradeID is absent (e.g. CAMS/KFintech CAS), OccurrenceIndex ensures multiple real same-day trades are preserved.
func (d *DeduplicationEngine) ComputeFingerprint(
	portfolioID int64,
	schemeCode int,
	symbol string,
	dateStr string,
	txType string,
	units float64,
	price float64,
	tradeID string,
	folioNumber string,
	occurrenceIndex int,
) string {
	if tradeID != "" {
		raw := fmt.Sprintf("%d|%s|%s|%s", portfolioID, strings.ToUpper(strings.TrimSpace(symbol)), dateStr, strings.TrimSpace(tradeID))
		hash := sha256.Sum256([]byte(raw))
		return fmt.Sprintf("%x", hash)
	}

	sig := d.ComputeSignature(portfolioID, schemeCode, dateStr, txType, units, price)
	raw := fmt.Sprintf("%s|#%d", sig, occurrenceIndex)
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", hash)
}

// ClassifyDrafts compares parsed drafts against existing portfolio records with occurrence sequence awareness.
func (d *DeduplicationEngine) ClassifyDrafts(
	portfolioID int64,
	drafts []RawTradeDraft,
	existingTransactions []models.PortfolioTransaction,
) []ProcessedTradeRecord {
	// 1. Map existing DB records by TradeID and by Signature occurrence count
	existingTradeIdMap := make(map[string]bool)
	existingSigOccurrences := make(map[string]int)

	for _, tx := range existingTransactions {
		dateStr := tx.TransactionDate.Format("2006-01-02")
		if tx.TradeId != nil && *tx.TradeId != "" {
			fp := d.ComputeFingerprint(portfolioID, tx.SchemeCode, "", dateStr, tx.TransactionType, tx.Units, tx.Nav, *tx.TradeId, "", 1)
			existingTradeIdMap[fp] = true
			if tx.Fingerprint != "" {
				existingTradeIdMap[tx.Fingerprint] = true
			}
		} else {
			sig := d.ComputeSignature(portfolioID, tx.SchemeCode, dateStr, tx.TransactionType, tx.Units, tx.Nav)
			existingSigOccurrences[sig]++
		}
	}

	// 2. Track intra-batch occurrences
	batchTradeIdSeen := make(map[string]bool)
	batchSigOccurrences := make(map[string]int)

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

		// Handle TradeID based (Exchange / Zerodha) trades
		if draft.TradeID != "" {
			fp := d.ComputeFingerprint(
				portfolioID,
				draft.SchemeCode,
				draft.Symbol,
				dateStr,
				draft.TransactionType,
				draft.Units,
				draft.Price,
				draft.TradeID,
				draft.FolioNumber,
				1,
			)

			if draft.SchemeCode <= 0 && draft.Symbol == "" {
				results = append(results, ProcessedTradeRecord{
					RawTradeDraft: draft,
					Status:        StatusUnresolved,
					StatusMessage: fmt.Sprintf("Symbol / ISIN '%s' could not be resolved", draft.ISIN),
					Fingerprint:   fp,
				})
				continue
			}

			if existingTradeIdMap[fp] {
				results = append(results, ProcessedTradeRecord{
					RawTradeDraft: draft,
					Status:        StatusDuplicate,
					StatusMessage: fmt.Sprintf("Exchange Trade ID '%s' already exists in portfolio", draft.TradeID),
					Fingerprint:   fp,
				})
				continue
			}

			if batchTradeIdSeen[fp] {
				results = append(results, ProcessedTradeRecord{
					RawTradeDraft: draft,
					Status:        StatusDuplicate,
					StatusMessage: fmt.Sprintf("Duplicate Trade ID '%s' in uploaded file", draft.TradeID),
					Fingerprint:   fp,
				})
				continue
			}

			batchTradeIdSeen[fp] = true
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusNew,
				StatusMessage: "New verified trade (will be imported)",
				Fingerprint:   fp,
			})
			continue
		}

		// Handle Mutual Fund CAS (No TradeID) with Occurrence Indexing
		sig := d.ComputeSignature(portfolioID, draft.SchemeCode, dateStr, draft.TransactionType, draft.Units, draft.Price)
		batchSigOccurrences[sig]++
		occurrenceIdx := batchSigOccurrences[sig]

		fp := d.ComputeFingerprint(
			portfolioID,
			draft.SchemeCode,
			draft.Symbol,
			dateStr,
			draft.TransactionType,
			draft.Units,
			draft.Price,
			"",
			draft.FolioNumber,
			occurrenceIdx,
		)

		if draft.SchemeCode <= 0 && draft.Symbol == "" {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusUnresolved,
				StatusMessage: fmt.Sprintf("Scheme / ISIN '%s' could not be resolved to a mutual fund", draft.ISIN),
				Fingerprint:   fp,
			})
			continue
		}

		// Check if DB already contains this occurrence
		existingOccCount := existingSigOccurrences[sig]
		if occurrenceIdx <= existingOccCount {
			results = append(results, ProcessedTradeRecord{
				RawTradeDraft: draft,
				Status:        StatusDuplicate,
				StatusMessage: fmt.Sprintf("Identical transaction occurrence #%d already recorded in portfolio", occurrenceIdx),
				Fingerprint:   fp,
			})
			continue
		}

		results = append(results, ProcessedTradeRecord{
			RawTradeDraft: draft,
			Status:        StatusNew,
			StatusMessage: fmt.Sprintf("Verified trade (occurrence #%d, will be imported)", occurrenceIdx),
			Fingerprint:   fp,
		})
	}

	return results
}
