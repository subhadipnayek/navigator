package importer_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"navigator-api/internal/models"
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
}
