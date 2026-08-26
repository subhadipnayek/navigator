package importer

import (
	"fmt"
	"math"
	"strings"
)

type ZerodhaEquityParser struct{}

func NewZerodhaEquityParser() *ZerodhaEquityParser {
	return &ZerodhaEquityParser{}
}

func (p *ZerodhaEquityParser) ID() string {
	return "zerodha_kite_equity"
}

func (p *ZerodhaEquityParser) DisplayName() string {
	return "Zerodha Kite (Equity & ETF Tradebook)"
}

func (p *ZerodhaEquityParser) Match(rows [][]string) bool {
	headerIdx, hMap := FindHeaderRow(rows, 25, "symbol", "tradedate", "tradetype", "quantity", "price")
	if headerIdx < 0 {
		return false
	}
	// Reject if segment is explicitly MF
	if _, ok := hMap["segment"]; ok {
		for r := headerIdx + 1; r < len(rows) && r < headerIdx+10; r++ {
			seg := strings.ToUpper(GetCell(rows[r], hMap, "segment"))
			if seg == "MF" {
				return false
			}
		}
	}
	// Check for equity signatures across data rows: Exchange is NSE/BSE, Segment is EQ, or ISIN is INE...
	for r := headerIdx + 1; r < len(rows) && r < headerIdx+15; r++ {
		ex := strings.ToUpper(GetCell(rows[r], hMap, "exchange"))
		seg := strings.ToUpper(GetCell(rows[r], hMap, "segment"))
		isin := strings.ToUpper(GetCell(rows[r], hMap, "isin"))
		if ex == "NSE" || ex == "BSE" || seg == "EQ" || strings.HasPrefix(isin, "INE") {
			return true
		}
	}
	return false
}

func (p *ZerodhaEquityParser) Parse(rows [][]string) ([]RawTradeDraft, error) {
	headerIdx, headerMap := FindHeaderRow(rows, 25, "symbol", "tradedate", "tradetype", "quantity", "price")
	if headerIdx < 0 {
		return nil, fmt.Errorf("zerodha equity tradebook header row not found")
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
		qtyStr := GetCell(row, headerMap, "quantity", "qty")
		priceStr := GetCell(row, headerMap, "price")
		tradeID := GetCell(row, headerMap, "tradeid", "trade_id")
		orderID := GetCell(row, headerMap, "orderid", "order_id")
		exchange := GetCell(row, headerMap, "exchange")

		txDate, err := ParseFlexibleDate(dateStr)
		if err != nil {
			continue
		}

		qty := math.Abs(ParseFlexibleFloat(qtyStr))
		price := ParseFlexibleFloat(priceStr)
		amount := qty * price

		if qty == 0 {
			continue
		}

		canonicalType := CanonicalTransactionType(typeRaw)

		assetType := AssetStock
		upperSym := strings.ToUpper(symbol)
		if strings.HasSuffix(upperSym, "BEES") || strings.HasSuffix(upperSym, "ETF") || strings.Contains(upperSym, "GOLD") || strings.Contains(upperSym, "SILVER") || strings.Contains(upperSym, "LIQUID") {
			assetType = AssetETF
		} else if strings.HasPrefix(isin, "INF") && (strings.Contains(upperSym, "DIRECT") || strings.Contains(upperSym, "GROWTH") || strings.Contains(upperSym, "PLAN") || strings.Contains(upperSym, "FUND")) {
			assetType = AssetMutualFund
		}

		notes := fmt.Sprintf("Symbol: %s | Zerodha Kite | %s | Trade ID: %s", symbol, exchange, tradeID)
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
			Units:           qty,
			Price:           price,
			Amount:          amount,
			TradeID:         tradeID,
			Notes:           notes,
		})
	}

	return drafts, nil
}
