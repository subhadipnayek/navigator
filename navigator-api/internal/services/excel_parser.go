package services

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ExcelTransactionDraft struct {
	RowNumber       int
	TransactionDate string
	TransactionType string
	Nav             string
	Units           string
	SchemeCode      string
	Isin            string
	SchemeName      string
	Notes           string
}

func ParseTransactionExcel(r io.Reader) ([]ExcelTransactionDraft, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel workbook: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("the workbook does not contain a readable worksheet")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil || len(rows) < 2 {
		return nil, nil
	}

	headerMap := make(map[string]int)
	for colIdx, cell := range rows[0] {
		normalized := normalizeHeader(cell)
		if normalized != "" {
			headerMap[normalized] = colIdx
		}
	}

	var drafts []ExcelTransactionDraft
	for rIdx := 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		if allEmpty(row) {
			continue
		}

		drafts = append(drafts, ExcelTransactionDraft{
			RowNumber:       rIdx + 1,
			TransactionDate: getCell(row, headerMap, "tradedate", "transactiondate", "date"),
			TransactionType: getCell(row, headerMap, "tradetype", "transactiontype", "type"),
			Nav:             getCell(row, headerMap, "price", "nav", "purchasenav"),
			Units:           getCell(row, headerMap, "quantity", "units"),
			SchemeCode:      getCell(row, headerMap, "schemecode", "code"),
			Isin:            getCell(row, headerMap, "isin", "isinno"),
			SchemeName:      getCell(row, headerMap, "schemename", "fundname", "scheme"),
			Notes:           getCell(row, headerMap, "notes", "remarks", "description"),
		})
	}

	return drafts, nil
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func getCell(row []string, headers map[string]int, possibleNames ...string) string {
	for _, name := range possibleNames {
		if idx, ok := headers[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
	}
	return ""
}

func allEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func ParseFloatOrZero(s string) float64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}
