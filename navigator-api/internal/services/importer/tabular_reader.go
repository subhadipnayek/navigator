package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

// TabularReader abstracts loading 2D string matrices from any tabular file stream.
type TabularReader struct{}

func NewTabularReader() *TabularReader {
	return &TabularReader{}
}

// Read extracts all non-empty rows from an uploaded file stream into a generic [][]string grid.
func (r *TabularReader) Read(reader io.Reader, filename string) ([][]string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file stream: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// 1. Check for CSV / TSV
	if ext == ".csv" || ext == ".tsv" || ext == ".txt" || isPlainText(data) {
		rows, csvErr := r.readCSV(data)
		if csvErr == nil && len(rows) > 0 {
			return rows, nil
		}
	}

	// 2. Try XLSX (Office Open XML)
	if ext == ".xlsx" || ext == ".xlsm" || len(data) > 4 && string(data[:4]) == "PK\x03\x04" {
		rows, xlsxErr := r.readXLSX(data)
		if xlsxErr == nil && len(rows) > 0 {
			return rows, nil
		}
	}

	// 3. Try legacy BIFF8 XLS (Excel 97-2004)
	rows, xlsErr := r.readXLS(data)
	if xlsErr == nil && len(rows) > 0 {
		return rows, nil
	}

	// 4. Fallback: retry CSV delimiter detection
	rows, csvFallbackErr := r.readCSV(data)
	if csvFallbackErr == nil && len(rows) > 0 {
		return rows, nil
	}

	return nil, fmt.Errorf("unsupported or unreadable tabular file format: %s", filename)
}

func (r *TabularReader) readXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("workbook contains no sheets")
	}

	// Pick the first sheet with actual data
	for _, sheet := range sheets {
		rows, err := f.GetRows(sheet)
		if err == nil && len(rows) > 0 {
			return rows, nil
		}
	}

	return nil, fmt.Errorf("no data found in any worksheet")
}

func (r *TabularReader) readXLS(data []byte) ([][]string, error) {
	// Write temporary file for extrame/xls since it requires file path or io.ReadSeeker
	tmpFile, err := os.CreateTemp("", "cams_xls_*.xls")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	xlFile, err := xls.Open(tmpFile.Name(), "utf-8")
	if err != nil {
		return nil, err
	}

	sheet := xlFile.GetSheet(0)
	if sheet == nil {
		return nil, fmt.Errorf("no sheets in xls workbook")
	}

	var rows [][]string
	for i := 0; i <= int(sheet.MaxRow); i++ {
		row := sheet.Row(i)
		if row == nil {
			continue
		}
		var cells []string
		for c := 0; c < row.LastCol(); c++ {
			cells = append(cells, strings.TrimSpace(row.Col(c)))
		}
		if !allCellsEmpty(cells) {
			rows = append(rows, cells)
		}
	}

	return rows, nil
}

func (r *TabularReader) readCSV(data []byte) ([][]string, error) {
	delimiters := []rune{',', '\t', ';', '|'}
	sample := string(data[:min(len(data), 4096)])

	var bestDelim rune = ','
	var maxCols int = 0

	for _, d := range delimiters {
		reader := csv.NewReader(strings.NewReader(sample))
		reader.Comma = d
		reader.FieldsPerRecord = -1
		if records, err := reader.ReadAll(); err == nil && len(records) > 0 {
			if len(records[0]) > maxCols {
				maxCols = len(records[0])
				bestDelim = d
			}
		}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = bestDelim
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var cleaned [][]string
	for _, row := range records {
		if !allCellsEmpty(row) {
			cleaned = append(cleaned, row)
		}
	}

	return cleaned, nil
}

func isPlainText(data []byte) bool {
	if len(data) == 0 {
		return true
	}
	checkLen := min(len(data), 1024)
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			return false
		}
	}
	return utf8.Valid(data[:checkLen])
}

func allCellsEmpty(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
