package importer_test

import (
	"os"
	"path/filepath"
	"testing"

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

func TestDeduplicationFingerprints(t *testing.T) {
	dedup := importer.NewDeduplicationEngine()

	fp1 := dedup.ComputeFingerprint(101, 120503, "HDFC TOP 100", "2023-05-15", "Buy", 100.5, 45.2, "")
	fp2 := dedup.ComputeFingerprint(101, 120503, "HDFC TOP 100", "2023-05-15", "buy", 100.5000, 45.2000, "")
	fp3 := dedup.ComputeFingerprint(101, 120503, "HDFC TOP 100", "2023-05-16", "Buy", 100.5, 45.2, "")

	if fp1 != fp2 {
		t.Errorf("Expected identical fingerprints for normalized casing/decimals, got %s != %s", fp1, fp2)
	}

	if fp1 == fp3 {
		t.Errorf("Expected different fingerprints for different dates, got identical: %s", fp1)
	}
}
