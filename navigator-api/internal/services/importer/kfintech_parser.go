package importer

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type KFintechParser struct{}

func NewKFintechParser() *KFintechParser {
	return &KFintechParser{}
}

func (p *KFintechParser) ID() string {
	return "kfintech_cas"
}

func (p *KFintechParser) DisplayName() string {
	return "KFintech Consolidated Account Statement (CAS)"
}

func (p *KFintechParser) Match(rows [][]string) bool {
	headerIdx, _ := FindHeaderRow(rows, 10, "fundname", "transactiondate", "schemeisin")
	if headerIdx >= 0 {
		return true
	}
	headerIdx2, _ := FindHeaderRow(rows, 10, "schemedescription", "transactiondescription", "nav")
	return headerIdx2 >= 0
}

var kfintechNoiseRegex = regexp.MustCompile(`(?i)[-\s]*[\(\[]?formerly\s+[^\]\)]+[\)\]]?`)

func (p *KFintechParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 10, "transactiondate", "transactiondescription")
	if headerIdx < 0 {
		headerIdx, headerMap = FindHeaderRow(rows, 10, "fundname", "schemedescription")
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("kfintech signature header row not found")
	}

	var drafts []RawTradeDraft

	for rIdx := headerIdx + 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allCellsEmpty(row) {
			continue
		}

		fundName := GetCell(row, headerMap, "fundname", "fund_name", "amc_name", "amc")
		txTypeRaw := GetCell(row, headerMap, "transactiondescription", "transaction_description", "type", "transactiontype")
		dateStr := GetCell(row, headerMap, "transactiondate", "transaction_date", "date", "tradedate")
		schemeName := GetCell(row, headerMap, "schemedescription", "scheme_description", "schemename", "scheme_name", "fundname")
		isin := GetCell(row, headerMap, "schemeisin", "isin", "isinno", "isin_no")
		accountNo := GetCell(row, headerMap, "accountnumber", "account_number", "folio", "folionumber", "foliono")
		amountStr := GetCell(row, headerMap, "amount", "trxnamount", "netamount")
		unitsStr := GetCell(row, headerMap, "units", "quantity", "qty")
		priceStr := GetCell(row, headerMap, "nav", "price", "purchasenav")

		if isNonFinancialEvent(txTypeRaw) {
			drafts = append(drafts, RawTradeDraft{
				SourceRowNumber: rIdx + 1,
				BrokerName:      p.DisplayName(),
				AssetType:       AssetMutualFund,
				SchemeName:      schemeName,
				ISIN:            isin,
				FolioNumber:     accountNo,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    fmt.Sprintf("Non-financial administrative event: %s", txTypeRaw),
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
				ISIN:            isin,
				FolioNumber:     accountNo,
				TransactionType: txTypeRaw,
				IsIgnored:       true,
				IgnoreReason:    fmt.Sprintf("Invalid date '%s'", dateStr),
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
			continue
		}

		canonicalType := CanonicalTransactionType(txTypeRaw)

		cleanScheme := kfintechNoiseRegex.ReplaceAllString(schemeName, "")
		cleanScheme = strings.TrimSpace(cleanScheme)

		if fundName != "" && !strings.Contains(strings.ToLower(cleanScheme), strings.ToLower(strings.TrimSuffix(fundName, " Mutual Fund"))) {
			cleanScheme = fundName + " - " + cleanScheme
		}

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			SchemeName:      cleanScheme,
			ISIN:            isin,
			FolioNumber:     accountNo,
			TransactionDate: txDate,
			TransactionType: canonicalType,
			Units:           units,
			Price:           price,
			Amount:          amount,
			Notes:           fmt.Sprintf("Folio: %s | KFintech %s", accountNo, txTypeRaw),
		})
	}

	return drafts, nil
}
