package importer

import (
	"math"
	"strconv"
	"strings"
	"time"
)

var knownDateFormats = []string{
	"02-Jan-2006",
	"02-JAN-2006",
	"02-jan-2006",
	"2006-01-02",
	"2006-01-02T15:04:05",
	"02/01/2006",
	"02-01-2006",
	"02/01/06",
	"2006/01/02",
	"2-Jan-2006",
	"2/1/2006",
}

// ParseFlexibleDate attempts to parse arbitrary date strings into time.Time (UTC).
func ParseFlexibleDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	// Remove trailing time or extra details if present (e.g. "09-Mar-2023 Purchase(NAV Dt : 09/03/2023)")
	if idx := strings.Index(s, "("); idx > 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if len(s) > 10 && strings.Contains(s, "T") {
		s = strings.Split(s, "T")[0]
	}

	for _, fmtStr := range knownDateFormats {
		if t, err := time.Parse(fmtStr, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
		}
	}

	return time.Time{}, &time.ParseError{Value: s, Message: "unrecognized date format"}
}

// ParseFlexibleFloat converts strings like "1,483.40", "(100.78)", or "-9169.12" to float64.
func ParseFlexibleFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "NA" || s == "N/A" {
		return 0
	}

	// Handle accounting parenthetical negative numbers: "(123.45)" -> "-123.45"
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")
	}

	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "₹", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimSpace(s)

	val, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(val) || math.IsInf(val, 0) {
		return 0
	}
	return val
}

// NormalizeHeader normalizes header column names for fuzzy mapping.
func NormalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	return s
}

// FindHeaderRow searches the first maxSearchRows for a row containing any signature keywords.
func FindHeaderRow(rows [][]string, maxSearchRows int, signatureKeywords ...string) (int, map[string]int) {
	limit := len(rows)
	if maxSearchRows > 0 && limit > maxSearchRows {
		limit = maxSearchRows
	}

	for rIdx := 0; rIdx < limit; rIdx++ {
		row := rows[rIdx]
		matched := 0
		hMap := make(map[string]int)

		for cIdx, cell := range row {
			norm := NormalizeHeader(cell)
			if norm == "" {
				continue
			}
			hMap[norm] = cIdx

			for _, kw := range signatureKeywords {
				if strings.Contains(norm, NormalizeHeader(kw)) {
					matched++
					break
				}
			}
		}

		if matched >= len(signatureKeywords) && len(hMap) >= 3 {
			return rIdx, hMap
		}
	}

	return -1, nil
}

// GetCell reads the first existing column from possible column aliases.
func GetCell(row []string, headers map[string]int, aliases ...string) string {
	for _, alias := range aliases {
		norm := NormalizeHeader(alias)
		if idx, ok := headers[norm]; ok && idx < len(row) {
			val := strings.TrimSpace(row[idx])
			if val != "" {
				return val
			}
		}
	}
	return ""
}

// CanonicalTransactionType standardizes transaction labels to "Buy", "Sell", "SIP", "Switch In", "Switch Out", etc.
func CanonicalTransactionType(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.Contains(lower, "sip") || strings.Contains(lower, "systematic investment"):
		return "SIP"
	case strings.Contains(lower, "switch in") || strings.Contains(lower, "switchin"):
		return "Switch In"
	case strings.Contains(lower, "switch out") || strings.Contains(lower, "switchout"):
		return "Switch Out"
	case strings.Contains(lower, "dividend") && strings.Contains(lower, "reinvest"):
		return "Dividend Reinvestment"
	case strings.Contains(lower, "redemption") || strings.Contains(lower, "sell") || strings.Contains(lower, "redeem"):
		return "Sell"
	case strings.Contains(lower, "purchase") || strings.Contains(lower, "buy"):
		return "Buy"
	default:
		if lower == "" {
			return "Buy"
		}
		return strings.Title(lower)
	}
}
