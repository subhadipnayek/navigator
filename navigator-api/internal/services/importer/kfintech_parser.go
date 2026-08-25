package importer

import (
	"fmt"
	"math"
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

		txTypeRaw := GetCell(row, headerMap, "transactiondescription", "transaction_description", "type")
		dateStr := GetCell(row, headerMap, "transactiondate", "transaction_date", "date")
		schemeName := GetCell(row, headerMap, "schemedescription", "scheme_description", "fundname")
		isin := GetCell(row, headerMap, "schemeisin", "isin", "isinno")
		accountNo := GetCell(row, headerMap, "accountnumber", "account_number", "folio")
		amountStr := GetCell(row, headerMap, "amount")
		unitsStr := GetCell(row, headerMap, "units")
		priceStr := GetCell(row, headerMap, "nav", "price")

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

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			SchemeName:      schemeName,
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
