package importer

import (
	"fmt"
	"math"
	"strings"
)

type ZerodhaMFParser struct{}

func NewZerodhaMFParser() *ZerodhaMFParser {
	return &ZerodhaMFParser{}
}

func (p *ZerodhaMFParser) ID() string {
	return "zerodha_coin_mf"
}

func (p *ZerodhaMFParser) DisplayName() string {
	return "Zerodha Coin (Mutual Funds Tradebook)"
}

func (p *ZerodhaMFParser) Match(rows [][]string) bool {
	// Look for Zerodha MF tradebook headers
	headerIdx, hMap := FindHeaderRow(rows, 25, "symbol", "tradedate", "tradetype", "quantity", "price")
	if headerIdx < 0 {
		return false
	}
	// Check for MF segment or ISIN INF prefix
	if _, ok := hMap["isin"]; ok {
		// Check sample rows below header
		for r := headerIdx + 1; r < len(rows) && r < headerIdx+5; r++ {
			rowStr := strings.Join(rows[r], " ")
			if strings.Contains(rowStr, "INF") || strings.Contains(strings.ToUpper(rowStr), "MUTUAL FUND") || strings.Contains(strings.ToUpper(rowStr), "DIRECT PLAN") {
				return true
			}
		}
	}
	return false
}

func (p *ZerodhaMFParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 25, "symbol", "tradedate", "tradetype", "quantity", "price")
	if headerIdx < 0 {
		return nil, fmt.Errorf("zerodha mf tradebook header row not found")
	}

	var drafts []RawTradeDraft

	for rIdx := headerIdx + 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allCellsEmpty(row) {
			continue
		}

		symbol := GetCell(row, headerMap, "symbol")
		isin := GetCell(row, headerMap, "isin")
		dateStr := GetCell(row, headerMap, "tradedate", "trade_date", "date")
		typeRaw := GetCell(row, headerMap, "tradetype", "trade_type", "type")
		qtyStr := GetCell(row, headerMap, "quantity", "qty", "units")
		priceStr := GetCell(row, headerMap, "price", "nav")
		tradeID := GetCell(row, headerMap, "tradeid", "trade_id", "orderid")
		orderID := GetCell(row, headerMap, "orderid", "order_id")

		txDate, err := ParseFlexibleDate(dateStr)
		if err != nil {
			continue
		}

		units := math.Abs(ParseFlexibleFloat(qtyStr))
		price := ParseFlexibleFloat(priceStr)
		amount := units * price

		if units == 0 {
			continue
		}

		canonicalType := CanonicalTransactionType(typeRaw)

		notes := fmt.Sprintf("Zerodha Coin | Trade ID: %s", tradeID)
		if orderID != "" && orderID != tradeID {
			notes += fmt.Sprintf(" | Order ID: %s", orderID)
		}

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       AssetMutualFund,
			Symbol:          symbol,
			ISIN:            isin,
			SchemeName:      symbol,
			TransactionDate: txDate,
			TransactionType: canonicalType,
			Units:           units,
			Price:           price,
			Amount:          amount,
			TradeID:         tradeID,
			Notes:           notes,
		})
	}

	return drafts, nil
}
