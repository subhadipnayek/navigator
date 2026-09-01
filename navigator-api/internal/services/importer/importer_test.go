package importer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"navigator-api/internal/models"
	"navigator-api/internal/services"
	"navigator-api/internal/services/importer"
)

func TestTabularReaderAndParsersOnRealSamples(t *testing.T) {
	dir := "/home/subhadipnayek/git_repos/navigator/sample transaction files"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Skipf("Sample transaction files directory not found at %s", dir)
	}

	reader := importer.NewTabularReader()
	registry := importer.NewBrokerRegistry()

	tests := []struct {
		filename        string
		expectedParser  string
		minParsedDrafts int
	}{
		{
			filename:        "cams transaction file.xls",
			expectedParser:  "cams_cas",
			minParsedDrafts: 15,
		},
		{
			filename:        "kfintech transaction file.xls",
			expectedParser:  "kfintech_cas",
			minParsedDrafts: 20,
		},
		{
			filename:        "DEMAT MF Trade History FY 2022-2023.xlsx",
			expectedParser:  "zerodha_coin_mf",
			minParsedDrafts: 20,
		},
		{
			filename:        "Trade History for FY 2021-2022.xlsx",
			expectedParser:  "zerodha_kite_equity",
			minParsedDrafts: 50,
		},
		{
			filename:        "Trade History for FY 2022-2023.xlsx",
			expectedParser:  "zerodha_kite_equity",
			minParsedDrafts: 50,
		},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			path := filepath.Join(dir, tc.filename)
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("Failed to open test file %s: %v", tc.filename, err)
			}
			defer f.Close()

			rows, err := reader.Read(f, tc.filename)
			if err != nil {
				t.Fatalf("Reader failed on %s: %v", tc.filename, err)
			}

			if len(rows) < 2 {
				t.Fatalf("Expected multiple rows, got %d", len(rows))
			}

			parser, drafts, err := registry.DetectAndParse(rows)
			if err != nil {
				t.Fatalf("Parser detection failed on %s: %v", tc.filename, err)
			}

			if parser.ID() != tc.expectedParser {
				t.Errorf("Expected parser '%s', got '%s'", tc.expectedParser, parser.ID())
			}

			if len(drafts) < tc.minParsedDrafts {
				t.Errorf("Expected at least %d drafts, got %d", tc.minParsedDrafts, len(drafts))
			}

			t.Logf("Success: %s parsed by [%s] with %d drafts", tc.filename, parser.DisplayName(), len(drafts))
		})
	}
}

func TestDeduplicationFingerprintsAndOccurrences(t *testing.T) {
	dedup := importer.NewDeduplicationEngine()

	// 1. Same TradeID yields identical fingerprint regardless of casing
	fpTrade1 := dedup.ComputeFingerprint(101, 0, "INFY", "2023-05-15", "Buy", 10, 1500, "TR12345", "", 1)
	fpTrade2 := dedup.ComputeFingerprint(101, 0, "infy", "2023-05-15", "BUY", 10, 1500, "TR12345", "", 1)
	if fpTrade1 != fpTrade2 {
		t.Errorf("Expected identical fingerprints for TradeID, got %s != %s", fpTrade1, fpTrade2)
	}

	// 2. Different TradeIDs on same second at same price yield distinct fingerprints (partial fill protection)
	fpTrade3 := dedup.ComputeFingerprint(101, 0, "INFY", "2023-05-15", "Buy", 10, 1500, "TR12346", "", 1)
	if fpTrade1 == fpTrade3 {
		t.Errorf("Expected different fingerprints for distinct TradeIDs, got identical: %s", fpTrade1)
	}

	// 3. Mutual Fund identical intra-day purchases differentiated by occurrence index
	fpOcc1 := dedup.ComputeFingerprint(101, 120503, "", "2023-05-15", "Buy", 50.0, 100.0, "", "FOLIO1", 1)
	fpOcc2 := dedup.ComputeFingerprint(101, 120503, "", "2023-05-15", "Buy", 50.0, 100.0, "", "FOLIO1", 2)
	if fpOcc1 == fpOcc2 {
		t.Errorf("Expected different fingerprints for distinct intra-day occurrences, got identical: %s", fpOcc1)
	}

	// 4. Test ClassifyDrafts with 2 identical same-day trades when DB has 1
	draftDate, _ := time.Parse("2006-01-02", "2023-05-15")
	drafts := []importer.RawTradeDraft{
		{SchemeCode: 120503, TransactionDate: draftDate, TransactionType: "Buy", Units: 50.0, Price: 100.0, FolioNumber: "FOLIO1"},
		{SchemeCode: 120503, TransactionDate: draftDate, TransactionType: "Buy", Units: 50.0, Price: 100.0, FolioNumber: "FOLIO1"},
	}

	existingInDb := []models.PortfolioTransaction{
		{PortfolioId: 101, SchemeCode: 120503, TransactionDate: draftDate, TransactionType: "Buy", Units: 50.0, Nav: 100.0},
	}

	classified := dedup.ClassifyDrafts(101, drafts, existingInDb)
	if len(classified) != 2 {
		t.Fatalf("Expected 2 classified drafts, got %d", len(classified))
	}

	if classified[0].Status != importer.StatusDuplicate {
		t.Errorf("Expected 1st occurrence to be Duplicate (already in DB), got %s", classified[0].Status)
	}
	if classified[1].Status != importer.StatusNew {
		t.Errorf("Expected 2nd occurrence to be New (genuine 2nd order), got %s", classified[1].Status)
	}

	// 5. Cross-broker deduplication: SIP and Buy on same date for same fund at same units/nav produce identical base signature
	sigSIP := dedup.ComputeSignature(101, 120503, "2023-05-15", "SIP", 50.0, 100.0)
	sigBuy := dedup.ComputeSignature(101, 120503, "2023-05-15", "Buy", 50.0, 100.0)
	if sigSIP != sigBuy {
		t.Errorf("Expected matching signatures for SIP and Buy, got %s != %s", sigSIP, sigBuy)
	}
}

func TestCamsParserSchemeEnrichmentAndNoiseStripping(t *testing.T) {
	parser := importer.NewCamsParser()
	rows := [][]string{
		{"MF_NAME", "FOLIO_NUMBER", "PRODUCT_CODE", "SCHEME_NAME", "TRADE_DATE", "TRANSACTION_TYPE", "AMOUNT", "UNITS", "PRICE"},
		{
			"PPFAS Mutual Fund", "13513468", "PP001ZGU",
			"Parag Parikh Flexi Cap - Dir Plan Growth-(formerly Parag Parikh Long Term Value Fund)",
			"02-JUN-2023", "Purchase", "5000", "78.12", "64.004",
		},
		{
			"DSP Mutual Fund", "8419486", "D887",
			"Nifty Midcap 150 Q50 Index Fund - Dir - G",
			"01-JUN-2023", "Purchase", "3000", "200.0", "15.0",
		},
	}

	drafts, err := parser.Parse(rows)
	if err != nil {
		t.Fatalf("Parser failed: %v", err)
	}

	if len(drafts) != 2 {
		t.Fatalf("Expected 2 drafts, got %d", len(drafts))
	}

	// 1. PPFAS should have former name stripped and AMC prepended
	if drafts[0].SchemeName != "PPFAS Mutual Fund - Parag Parikh Flexi Cap - Dir Plan Growth" {
		t.Errorf("Expected cleaned and AMC-prepended scheme name, got '%s'", drafts[0].SchemeName)
	}

	// 2. DSP should have AMC prepended
	if drafts[1].SchemeName != "DSP Mutual Fund - Nifty Midcap 150 Q50 Index Fund - Dir - G" {
		t.Errorf("Expected DSP AMC prepended scheme name, got '%s'", drafts[1].SchemeName)
	}
}

func TestAuditAllSampleFiles(t *testing.T) {
	dir := "/home/subhadipnayek/git_repos/navigator/sample transaction files"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// Also check relative to workspace
		relDir := "../../../../sample transaction files"
		if _, errRel := os.Stat(relDir); errRel == nil {
			dir = relDir
		} else {
			t.Skipf("Sample transaction files directory not found at %s", dir)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	reader := importer.NewTabularReader()
	registry := importer.NewBrokerRegistry()

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			t.Errorf("[%s] Open error: %v", e.Name(), err)
			continue
		}

		rows, err := reader.Read(f, e.Name())
		f.Close()
		if err != nil {
			t.Errorf("[%s] Read error: %v", e.Name(), err)
			continue
		}

		parser, drafts, err := registry.DetectAndParse(rows)
		if err != nil {
			t.Errorf("[%s] Detect/Parse error: %v", e.Name(), err)
			continue
		}

		validCount := 0
		ignoredCount := 0
		typeCounts := make(map[string]int)
		var minDate, maxDate time.Time
		totalAmount := 0.0

		for _, d := range drafts {
			if d.IsIgnored {
				ignoredCount++
			} else {
				validCount++
				totalAmount += d.Amount
				if !d.TransactionDate.IsZero() {
					if minDate.IsZero() || d.TransactionDate.Before(minDate) {
						minDate = d.TransactionDate
					}
					if maxDate.IsZero() || d.TransactionDate.After(maxDate) {
						maxDate = d.TransactionDate
					}
				}
			}
			typeCounts[d.TransactionType]++
		}

		t.Logf("=== AUDIT FILE: %s ===", e.Name())
		t.Logf("  Rows: %d | Detected Parser: [%s] %s", len(rows), parser.ID(), parser.DisplayName())
		t.Logf("  Drafts: Total=%d | Valid=%d | Ignored=%d", len(drafts), validCount, ignoredCount)
		t.Logf("  Date Span: %s to %s | Total Ingestion Value: %.2f", minDate.Format("2006-01-02"), maxDate.Format("2006-01-02"), totalAmount)
		t.Logf("  Transaction Types Breakdown: %v", typeCounts)
		if len(drafts) > 0 {
			t.Logf("  First Draft: Scheme='%s' | ISIN='%s' | Code=%d | Date=%s | Type=%s | Units=%.4f | Price=%.4f | TradeID='%s'",
				drafts[0].SchemeName, drafts[0].ISIN, drafts[0].SchemeCode, drafts[0].TransactionDate.Format("2006-01-02"), drafts[0].TransactionType, drafts[0].Units, drafts[0].Price, drafts[0].TradeID)
		}
	}
}

func TestParseAmfiMasterData(t *testing.T) {
	sampleCsv := `AMC,Code,Scheme Name,Scheme Type,Scheme Category,Scheme NAV Name,Scheme Minimum Amount,Launch Date, Closure Date,ISIN Div Payout/ ISIN GrowthISIN Div Reinvestment
Aditya Birla Sun Life AMC Limited,100033,Aditya Birla Sun Life Large & Mid Cap Fund,Open Ended,Equity Scheme - Large & Mid Cap Fund,Aditya Birla Sun Life Large & Mid Cap Fund - Regular Growth,5000,24-Feb-1995,24-Feb-1995,INF209K01165
Aditya Birla Sun Life AMC Limited,100034,Aditya Birla Sun Life Large & Mid Cap Fund,Open Ended,Equity Scheme - Large & Mid Cap Fund,Aditya Birla Sun Life Large & Mid Cap Fund -Regular - IDCW,5000,24-Feb-1995,24-Feb-1995,INF209K01157INF209K01CE5
HDFC Asset Management Company Limited,119436,HDFC Large and Mid Cap Fund,Open Ended,Equity Scheme - Large & Mid Cap Fund,HDFC Large and Mid Cap Fund - Growth - Direct Plan,5000,24-Feb-1995,24-Feb-1995,INF179K01VH8
`

	funds, err := services.ParseAmfiMasterData(strings.NewReader(sampleCsv))
	if err != nil {
		t.Fatalf("Unexpected error parsing AMFI master CSV: %v", err)
	}

	if len(funds) != 3 {
		t.Fatalf("Expected 3 parsed funds, got %d", len(funds))
	}

	f1 := funds[0]
	if f1.SchemeCode != 100033 {
		t.Errorf("Expected SchemeCode 100033, got %d", f1.SchemeCode)
	}
	if f1.SchemeName != "Aditya Birla Sun Life Large & Mid Cap Fund - Regular Growth" {
		t.Errorf("Expected Scheme NAV Name, got %s", f1.SchemeName)
	}
	if f1.FundHouse == nil || *f1.FundHouse != "Aditya Birla Sun Life AMC Limited" {
		t.Errorf("Expected fund house 'Aditya Birla Sun Life AMC Limited', got %v", f1.FundHouse)
	}
	if f1.IsinGrowth == nil || *f1.IsinGrowth != "INF209K01165" {
		t.Errorf("Expected ISIN INF209K01165, got %v", f1.IsinGrowth)
	}

	f2 := funds[1]
	if f2.IsinGrowth == nil || *f2.IsinGrowth != "INF209K01157" {
		t.Errorf("Expected growth ISIN INF209K01157, got %v", f2.IsinGrowth)
	}
	if f2.IsinDivReinvestment == nil || *f2.IsinDivReinvestment != "INF209K01CE5" {
		t.Errorf("Expected div reinvestment ISIN INF209K01CE5, got %v", f2.IsinDivReinvestment)
	}
}

func TestParseLatestNavAll(t *testing.T) {
	sampleNavAll := `Scheme Code;ISIN Div Payout/ ISIN Growth;ISIN Div Reinvestment;Scheme Name;Plan;Option;Net Asset Value;Date
 
Open Ended Schemes(Debt Scheme - Banking and PSU Fund)
 
Aditya Birla Sun Life Mutual Fund
 
119551;INF209KA12Z1;INF209KA13Z9;Aditya Birla Sun Life Banking & PSU Debt Fund;Direct Plan;IDCW-Re-investment;106.9419;25-Aug-2026
119550;INF209K01YN0;-;Aditya Birla Sun Life Banking & PSU Debt Fund;Direct Plan;GROWTH;403.8751;25-Aug-2026
`

	snapshots, err := services.ParseLatestNavAll(strings.NewReader(sampleNavAll))
	if err != nil {
		t.Fatalf("Unexpected error parsing NAVAll: %v", err)
	}

	if len(snapshots) != 2 {
		t.Fatalf("Expected 2 snapshots, got %d", len(snapshots))
	}

	if snapshots[0].SchemeCode != 119551 || snapshots[0].Nav != 106.9419 {
		t.Errorf("Expected SchemeCode 119551 with Nav 106.9419, got code %d nav %f", snapshots[0].SchemeCode, snapshots[0].Nav)
	}
	if snapshots[1].SchemeCode != 119550 || snapshots[1].Nav != 403.8751 {
		t.Errorf("Expected SchemeCode 119550 with Nav 403.8751, got code %d nav %f", snapshots[1].SchemeCode, snapshots[1].Nav)
	}
}
