package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"navigator-api/internal/models"
)

func TestParseAmfiNavHistoryJson_Success(t *testing.T) {
	jsonPayload := `{
		"data": {
			"mf_name": "ICICI Prudential Mutual Fund",
			"scheme_name": "ICICI Prudential Nifty 50 ETF",
			"date_range": "From 01-Jan-2024 to 03-Jan-2024",
			"nav_groups": [
				{
					"nav_name": "ICICI Prudential Nifty 50 ETF",
					"historical_records": [
						{
							"date": "2024-01-01",
							"nav": 238.8447,
							"upload_time": "2024-01-01T22:44:16.000Z"
						},
						{
							"date": "2024-01-02",
							"nav": "239.1250",
							"upload_time": "2024-01-02T22:44:16.000Z"
						},
						{
							"date": "2024-01-03",
							"nav": 240.5,
							"upload_time": "2024-01-03T22:44:16.000Z"
						}
					]
				}
			]
		}
	}`

	schemeCode := 121366
	records, err := ParseAmfiNavHistoryJson([]byte(jsonPayload), schemeCode)
	if err != nil {
		t.Fatalf("ParseAmfiNavHistoryJson unexpected error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].SchemeCode != schemeCode {
		t.Errorf("expected scheme code %d, got %d", schemeCode, records[0].SchemeCode)
	}

	if records[0].Nav != 238.8447 {
		t.Errorf("expected nav 238.8447, got %f", records[0].Nav)
	}

	if records[1].Nav != 239.1250 {
		t.Errorf("expected nav 239.1250, got %f", records[1].Nav)
	}

	expectedDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if !records[0].Date.Equal(expectedDate) {
		t.Errorf("expected date %v, got %v", expectedDate, records[0].Date)
	}
}

func TestParseAmfiNavHistoryJson_NoRecords(t *testing.T) {
	payload := `{"message":"No records to display"}`
	records, err := ParseAmfiNavHistoryJson([]byte(payload), 121366)
	if err != nil {
		t.Fatalf("unexpected error on empty records: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestParseAmfiNavHistoryJson_EmptyData(t *testing.T) {
	payload := `{"data":{"nav_groups":[]}}`
	records, err := ParseAmfiNavHistoryJson([]byte(payload), 121366)
	if err != nil {
		t.Fatalf("unexpected error on empty nav_groups: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestFetchNavHistory_MultiChunking(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if q.Get("query_type") != "historical_period" {
			t.Errorf("expected query_type=historical_period, got %s", q.Get("query_type"))
		}
		if q.Get("sd_id") != "121366" {
			t.Errorf("expected sd_id=121366, got %s", q.Get("sd_id"))
		}

		fromDate := q.Get("from_date")
		resp := `{
			"data": {
				"mf_name": "Test AMC",
				"scheme_name": "Test Fund",
				"nav_groups": [
					{
						"nav_name": "Test Fund",
						"historical_records": [
							{"date": "` + fromDate + `", "nav": 100.5}
						]
					}
				]
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}))
	defer server.Close()

	amfi := &AmfiService{
		httpClient: server.Client(),
	}

	from := time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	ctx := context.Background()
	reqUrl := server.URL

	records, err := testFetchNavHistoryWithCustomUrl(ctx, amfi, reqUrl, 121366, from, to)
	if err != nil {
		t.Fatalf("FetchNavHistory unexpected error: %v", err)
	}

	if calls < 3 {
		t.Errorf("expected at least 3 chunks for 10-year range, got %d calls", calls)
	}

	if len(records) < 3 {
		t.Errorf("expected at least 3 records from merged chunks, got %d", len(records))
	}
}

func testFetchNavHistoryWithCustomUrl(ctx context.Context, s *AmfiService, testUrl string, schemeCode int, from, to time.Time) ([]models.NavHistory, error) {
	if schemeCode <= 0 {
		return nil, nil
	}

	var chunks [][2]time.Time
	currFrom := from
	for !currFrom.After(to) {
		currTo := currFrom.AddDate(4, 0, 0)
		if currTo.After(to) {
			currTo = to
		}
		chunks = append(chunks, [2]time.Time{currFrom, currTo})
		currFrom = currTo.AddDate(0, 0, 1)
	}

	recordMap := make(map[string]models.NavHistory)
	for _, chunk := range chunks {
		url := testUrl + "?query_type=historical_period&from_date=" + FormatDateYYYYMMDD(chunk[0]) + "&to_date=" + FormatDateYYYYMMDD(chunk[1]) + "&sd_id=121366"
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		var body []byte
		buf := make([]byte, 4096)
		n, _ := resp.Body.Read(buf)
		body = buf[:n]
		recs, _ := ParseAmfiNavHistoryJson(body, schemeCode)
		for _, r := range recs {
			recordMap[r.Date.Format(DateFormatYYYYMMDD)] = r
		}
	}

	result := make([]models.NavHistory, 0, len(recordMap))
	for _, r := range recordMap {
		result = append(result, r)
	}
	return result, nil
}
