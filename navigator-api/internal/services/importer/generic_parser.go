package importer

import (
	"fmt"
	"math"
)

type GenericParser struct{}

func NewGenericParser() *GenericParser {
	return &GenericParser{}
}

func (p *GenericParser) ID() string {
	return "generic_spreadsheet"
}

func (p *GenericParser) DisplayName() string {
	return "Generic CSV / Excel Trade Ledger"
}

func (p *GenericParser) Match(rows [][]string) bool {
	headerIdx, _ := FindHeaderRow(rows, 10, "date", "units")
	if headerIdx >= 0 {
		return true
	}
	headerIdx2, _ := FindHeaderRow(rows, 10, "tradedate", "quantity")
	return headerIdx2 >= 0
}

func (p *GenericParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 10, "date")
	if headerIdx < 0 {
		headerIdx, headerMap = FindHeaderRow(rows, 10, "tradedate")
	}
	if headerIdx < 0 {
		return nil, fmt.Errorf("generic header row with date column not found")
	}

	var drafts []RawTradeDraft

	for rIdx := headerIdx + 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allCellsEmpty(row) {
			continue
		}

		dateStr := GetCell(row, headerMap, "date", "tradedate", "transactiondate")
		typeRaw := GetCell(row, headerMap, "type", "tradetype", "transactiontype")
		unitsStr := GetCell(row, headerMap, "units", "quantity", "qty")
		priceStr := GetCell(row, headerMap, "price", "nav", "purchasenav")
		amountStr := GetCell(row, headerMap, "amount", "value", "totalamount")
		codeStr := GetCell(row, headerMap, "code", "schemecode", "scheme_code", "symbol")
		isin := GetCell(row, headerMap, "isin", "isinno")
		name := GetCell(row, headerMap, "name", "schemename", "fundname", "description")
		notes := GetCell(row, headerMap, "notes", "remarks", "memo")

		txDate, err := ParseFlexibleDate(dateStr)
		if err != nil {
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

		canonicalType := CanonicalTransactionType(typeRaw)

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			Symbol:          codeStr,
			ISIN:            isin,
			SchemeName:      name,
			TransactionDate: txDate,
			TransactionType: canonicalType,
			Units:           units,
			Price:           price,
			Amount:          amount,
			Notes:           notes,
		})
	}

	return drafts, nil
}
