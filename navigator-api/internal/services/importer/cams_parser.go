package importer

import (
	"fmt"
	"math"
	"regexp"
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
	if headerIdx >= 0 {
		return true
	}
	headerIdx2, _ := FindHeaderRow(rows, 10, "folionumber", "productcode", "schemename")
	return headerIdx2 >= 0
}

var camsNoiseRegex = regexp.MustCompile(`(?i)[-\s]*[\(\[]?formerly\s+[^\]\)]+[\)\]]?`)

func (p *CamsParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 10, "mfname", "tradedate", "transactiontype")
	if headerIdx < 0 {
		headerIdx, headerMap = FindHeaderRow(rows, 10, "folionumber", "productcode", "schemename")
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("cams signature header row not found")
	}

	var drafts []RawTradeDraft

	for rIdx := headerIdx + 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allCellsEmpty(row) {
			continue
		}

		mfName := GetCell(row, headerMap, "mfname", "mf_name", "amc_name", "amc", "amcname", "mutualfund")
		txTypeRaw := GetCell(row, headerMap, "transactiontype", "transaction_type", "type", "natureoftransaction", "description")
		dateStr := GetCell(row, headerMap, "tradedate", "trade_date", "date", "trxndate", "transactiondate")
		schemeName := GetCell(row, headerMap, "schemename", "scheme_name", "scheme", "schemedescription", "product_name")
		prodCode := GetCell(row, headerMap, "productcode", "product_code", "prodcode")
		isin := GetCell(row, headerMap, "isin", "isinno", "isin_no", "schemeisin", "isin_code")
		folioNo := GetCell(row, headerMap, "folionumber", "folio_number", "folio", "foliono")
		amountStr := GetCell(row, headerMap, "amount", "trxnamount", "netamount", "grossamount")
		unitsStr := GetCell(row, headerMap, "units", "quantity", "qty", "trxnunits")
		priceStr := GetCell(row, headerMap, "price", "nav", "purchasenav", "rate")

		// Identify non-financial administrative entries
		if isNonFinancialEvent(txTypeRaw) {
			drafts = append(drafts, RawTradeDraft{
				SourceRowNumber: rIdx + 1,
				BrokerName:      p.DisplayName(),
				AssetType:       AssetMutualFund,
				SchemeName:      schemeName,
				Symbol:          prodCode,
				ISIN:            isin,
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
				Symbol:          prodCode,
				ISIN:            isin,
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
				Symbol:          prodCode,
				ISIN:            isin,
				TransactionDate: txDate,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    "Zero units and zero amount trade",
			})
			continue
		}

		canonicalType := CanonicalTransactionType(txTypeRaw)

		// Clean scheme description
		cleanScheme := camsNoiseRegex.ReplaceAllString(schemeName, "")
		cleanScheme = strings.TrimSpace(cleanScheme)

		// Combine AMC name if not present in scheme description
		if mfName != "" && !strings.Contains(strings.ToLower(cleanScheme), strings.ToLower(strings.TrimSuffix(mfName, " Mutual Fund"))) {
			cleanScheme = mfName + " - " + cleanScheme
		}

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			SchemeName:      cleanScheme,
			Symbol:          prodCode,
			ISIN:            isin,
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
		"deletion of nominee",
		"change / regn of nominee",
		"consolidation of folios",
		"consolidation of accounts",
		"nominee opt-out",
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
