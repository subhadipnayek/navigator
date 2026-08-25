package importer

import (
	"fmt"
	"math"
	"strings"
)

type CamsParser struct{}

func NewCamsParser() *CamsParser {
	return &CamsParser{}
}

func (p *CamsParser) ID() string {
	return "cams_cas"
}

func (p *CamsParser) DisplayName() string {
	return "CAMS Consolidated Account Statement (CAS)"
}

func (p *CamsParser) Match(rows [][]string) bool {
	headerIdx, _ := FindHeaderRow(rows, 10, "mfname", "tradedate", "transactiontype")
	return headerIdx >= 0
}

func (p *CamsParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 10, "mfname", "tradedate", "transactiontype")
	if headerIdx < 0 {
		return nil, fmt.Errorf("cams signature header row not found")
	}

	var drafts []RawTradeDraft

	for rIdx := headerIdx + 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allCellsEmpty(row) {
			continue
		}

		txTypeRaw := GetCell(row, headerMap, "transactiontype", "transaction_type", "type")
		dateStr := GetCell(row, headerMap, "tradedate", "trade_date", "date")
		schemeName := GetCell(row, headerMap, "schemename", "scheme_name", "product_code")
		folioNo := GetCell(row, headerMap, "folionumber", "folio_number", "folio")
		amountStr := GetCell(row, headerMap, "amount")
		unitsStr := GetCell(row, headerMap, "units")
		priceStr := GetCell(row, headerMap, "price", "nav")

		// Identify non-financial administrative entries
		if isNonFinancialEvent(txTypeRaw) {
			drafts = append(drafts, RawTradeDraft{
				SourceRowNumber: rIdx + 1,
				BrokerName:      p.DisplayName(),
				AssetType:       AssetMutualFund,
				SchemeName:      schemeName,
				FolioNumber:     folioNo,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    fmt.Sprintf("Non-financial administrative log: %s", txTypeRaw),
			})
			continue
		}

		txDate, dateErr := ParseFlexibleDate(dateStr)
		if dateErr != nil {
			drafts = append(drafts, RawTradeDraft{
				SourceRowNumber: rIdx + 1,
				BrokerName:      p.DisplayName(),
				AssetType:       AssetMutualFund,
				SchemeName:      schemeName,
				FolioNumber:     folioNo,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    fmt.Sprintf("Invalid or empty date: '%s'", dateStr),
			})
			continue
		}

		units := math.Abs(ParseFlexibleFloat(unitsStr))
		price := ParseFlexibleFloat(priceStr)
		amount := math.Abs(ParseFlexibleFloat(amountStr))

		if amount == 0 && units > 0 && price > 0 {
			amount = units * price
		}
		if price == 0 && units > 0 && amount > 0 {
			price = amount / units
		}

		if units == 0 && amount == 0 {
			drafts = append(drafts, RawTradeDraft{
				SourceRowNumber: rIdx + 1,
				BrokerName:      p.DisplayName(),
				AssetType:       AssetMutualFund,
				SchemeName:      schemeName,
				TransactionDate: txDate,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    "Zero units and zero amount trade",
			})
			continue
		}

		canonicalType := CanonicalTransactionType(txTypeRaw)

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			SchemeName:      schemeName,
			FolioNumber:     folioNo,
			TransactionDate: txDate,
			TransactionType: canonicalType,
			Units:           units,
			Price:           price,
			Amount:          amount,
			Notes:           fmt.Sprintf("Folio: %s | CAMS %s", folioNo, txTypeRaw),
		})
	}

	return drafts, nil
}

func isNonFinancialEvent(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	nonFinancialKeywords := []string{
		"address updated",
		"kra data",
		"registration of nominee",
		"nominee",
		"change of bank",
		"bank mandate",
		"sip registration",
		"stp registration",
		"swp registration",
		"kyc",
		"fatca",
		"transmission",
		"signature update",
		"mobile update",
		"email update",
	}
	for _, kw := range nonFinancialKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}
