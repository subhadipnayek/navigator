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
	headerIdx, hMap := FindHeaderRow(rows, 25, "symbol", "tradedate", "tradetype", "quantity", "price")
	if headerIdx < 0 {
		return false
	}
	// Check if segment is explicitly MF
	if _, ok := hMap["segment"]; ok {
		for r := headerIdx + 1; r < len(rows) && r < headerIdx+10; r++ {
			seg := strings.ToUpper(GetCell(rows[r], hMap, "segment"))
			if seg == "MF" {
				return true
			}
		}
	}
	// Check for Mutual Fund tradebook characteristics in Symbol/Name column
	for r := headerIdx + 1; r < len(rows) && r < headerIdx+15; r++ {
		sym := strings.ToUpper(GetCell(rows[r], hMap, "symbol"))
		isin := strings.ToUpper(GetCell(rows[r], hMap, "isin"))
		if strings.Contains(sym, "DIRECT PLAN") || strings.Contains(sym, "REGULAR PLAN") || strings.Contains(sym, "MUTUAL FUND") ||
			(strings.HasPrefix(isin, "INF") && !strings.HasSuffix(sym, "BEES") && !strings.HasSuffix(sym, "ETF") && !strings.Contains(sym, "GOLD") && !strings.Contains(sym, "SILVER")) {
			return true
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

		assetType := AssetMutualFund
		upperSym := strings.ToUpper(symbol)
		if strings.HasPrefix(isin, "INE") {
			assetType = AssetStock
		} else if strings.HasSuffix(upperSym, "BEES") || strings.HasSuffix(upperSym, "ETF") {
			assetType = AssetETF
		}

		notes := fmt.Sprintf("Symbol: %s | Zerodha Coin | Trade ID: %s", symbol, tradeID)
		if orderID != "" && orderID != tradeID {
			notes += fmt.Sprintf(" | Order ID: %s", orderID)
		}

		drafts = append(drafts, RawTradeDraft{
			SourceRowNumber: rIdx + 1,
			BrokerName:      p.DisplayName(),
			AssetType:       assetType,
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
