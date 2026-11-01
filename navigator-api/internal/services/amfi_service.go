package services

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"navigator-api/internal/models"
)

const (
	AmfiMasterURL    = "https://portal.amfiindia.com/DownloadSchemeData_Po.aspx?mf=0"
	AmfiLatestNavURL = "https://portal.amfiindia.com/spages/NAVAll.txt"
	AmfiNavHistoryURL = "https://www.amfiindia.com/api/nav-history"
)

type AmfiService struct {
	httpClient         *http.Client
	integrationService *IntegrationService
}

func NewAmfiService(integrationSvc ...*IntegrationService) *AmfiService {
	var intSvc *IntegrationService
	if len(integrationSvc) > 0 {
		intSvc = integrationSvc[0]
	}
	return &AmfiService{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		integrationService: intSvc,
	}
}

type AmfiNavHistoryRecord struct {
	Date              string      `json:"date"`
	Nav               interface{} `json:"nav"`
	UploadTime        string      `json:"upload_time"`
	UploadTimeDisplay string      `json:"upload_time_display"`
	Repurchase        interface{} `json:"repurchase"`
	Reissue           interface{} `json:"reissue"`
}

type AmfiNavGroup struct {
	NavName           string                 `json:"nav_name"`
	HistoricalRecords []AmfiNavHistoryRecord `json:"historical_records"`
}

type AmfiNavHistoryData struct {
	MfName     string         `json:"mf_name"`
	SchemeName string         `json:"scheme_name"`
	DateRange  string         `json:"date_range"`
	NavGroups  []AmfiNavGroup `json:"nav_groups"`
}

type AmfiNavHistoryApiResponse struct {
	Data    *AmfiNavHistoryData `json:"data"`
	Message string              `json:"message"`
	Error   string              `json:"error"`
}

type LatestNavSnapshot struct {
	SchemeCode int
	Date       time.Time
	Nav        float64
}

func (s *AmfiService) FetchFundMasterList(ctx context.Context) ([]models.Fund, error) {
	url := AmfiMasterURL
	if s.integrationService != nil {
		url = s.integrationService.GetActiveUrl(ctx, "AMFI_MASTER_URL", AmfiMasterURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AMFI returned status %d", resp.StatusCode)
	}

	return ParseAmfiMasterData(resp.Body)
}

func ParseAmfiMasterData(r io.Reader) ([]models.Fund, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil && len(records) == 0 {
		return nil, err
	}

	var funds []models.Fund
	now := time.Now().UTC()

	for i, parts := range records {
		if len(parts) < 2 {
			continue
		}

		// Skip header line
		if i == 0 && (strings.EqualFold(parts[0], "amc") || strings.EqualFold(parts[0], "scheme code") || strings.Contains(strings.ToLower(parts[1]), "code")) {
			continue
		}

		schemeCode := 0
		var fundHouse, schemeType, schemeCategory *string
		schemeName := ""
		var isinGrowth, isinDivReinvest *string

		if code, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && code > 0 {
			// AMFI Standard: AMC,Code,Scheme Name,Scheme Type,Scheme Category,Scheme NAV Name,Scheme Minimum Amount,Launch Date,Closure Date,ISIN
			schemeCode = code
			if strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[0]) != "-" {
				v := strings.TrimSpace(parts[0])
				fundHouse = &v
			}
			// Prefer Scheme NAV Name (column 5) if available, else Scheme Name (column 2)
			if len(parts) > 5 && strings.TrimSpace(parts[5]) != "" {
				schemeName = strings.TrimSpace(parts[5])
			} else if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
				schemeName = strings.TrimSpace(parts[2])
			}
			if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" && strings.TrimSpace(parts[3]) != "-" {
				v := strings.TrimSpace(parts[3])
				schemeType = &v
			}
			if len(parts) > 4 && strings.TrimSpace(parts[4]) != "" && strings.TrimSpace(parts[4]) != "-" {
				v := strings.TrimSpace(parts[4])
				schemeCategory = &v
			}
			if len(parts) > 9 {
				isinRaw := strings.TrimSpace(parts[9])
				if len(isinRaw) == 12 {
					isinGrowth = &isinRaw
				} else if len(isinRaw) >= 24 {
					g := isinRaw[0:12]
					d := isinRaw[12:24]
					isinGrowth = &g
					isinDivReinvest = &d
				}
			}
		} else if code, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil && code > 0 {
			// Legacy fallback: Code,Scheme Name,ISIN Growth,ISIN Div Reinvest,Fund House,Scheme Type,Scheme Category
			schemeCode = code
			if len(parts) > 1 {
				schemeName = strings.TrimSpace(parts[1])
			}
			if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" && strings.TrimSpace(parts[2]) != "-" {
				v := strings.TrimSpace(parts[2])
				isinGrowth = &v
			}
			if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" && strings.TrimSpace(parts[3]) != "-" {
				v := strings.TrimSpace(parts[3])
				isinDivReinvest = &v
			}
			if len(parts) > 4 && strings.TrimSpace(parts[4]) != "" && strings.TrimSpace(parts[4]) != "-" {
				v := strings.TrimSpace(parts[4])
				fundHouse = &v
			}
			if len(parts) > 5 && strings.TrimSpace(parts[5]) != "" && strings.TrimSpace(parts[5]) != "-" {
				v := strings.TrimSpace(parts[5])
				schemeType = &v
			}
			if len(parts) > 6 && strings.TrimSpace(parts[6]) != "" && strings.TrimSpace(parts[6]) != "-" {
				v := strings.TrimSpace(parts[6])
				schemeCategory = &v
			}
		} else {
			continue
		}

		if schemeCode <= 0 || schemeName == "" {
			continue
		}

		funds = append(funds, models.Fund{
			SchemeCode:          schemeCode,
			SchemeName:          schemeName,
			FundHouse:           fundHouse,
			SchemeType:          schemeType,
			SchemeCategory:      schemeCategory,
			IsinGrowth:          isinGrowth,
			IsinDivReinvestment: isinDivReinvest,
			CreatedAtUtc:        now,
			UpdatedAtUtc:        now,
		})
	}

	return funds, nil
}

func (s *AmfiService) FetchLatestNavSnapshots(ctx context.Context) ([]LatestNavSnapshot, error) {
	url := AmfiLatestNavURL
	if s.integrationService != nil {
		url = s.integrationService.GetActiveUrl(ctx, "AMFI_LATEST_NAV_URL", AmfiLatestNavURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AMFI returned status %d", resp.StatusCode)
	}

	return ParseLatestNavAll(resp.Body)
}

func ParseLatestNavAll(r io.Reader) ([]LatestNavSnapshot, error) {
	var snapshots []LatestNavSnapshot
	scanner := bufio.NewScanner(r)

	// Format 1 (8 columns): Scheme Code;ISIN Div Payout/ ISIN Growth;ISIN Div Reinvestment;Scheme Name;Plan;Option;Net Asset Value;Date
	// Format 2 (6 columns): Scheme Code;ISIN Div Payout/ ISIN Growth;ISIN Div Reinvestment;Scheme Name;Net Asset Value;Date
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Split(line, ";")
		if len(parts) < 6 {
			continue
		}

		schemeCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || schemeCode <= 0 {
			continue
		}

		navIdx := 4
		dateIdx := 5
		if len(parts) >= 8 {
			navIdx = 6
			dateIdx = 7
		}

		navStr := strings.TrimSpace(parts[navIdx])
		nav, err := strconv.ParseFloat(navStr, 64)
		if err != nil || nav <= 0 {
			continue
		}

		dateStr := strings.TrimSpace(parts[dateIdx])
		date, err := ParseDate(dateStr)
		if err != nil {
			continue
		}

		snapshots = append(snapshots, LatestNavSnapshot{
			SchemeCode: schemeCode,
			Date:       date,
			Nav:        nav,
		})
	}

	return snapshots, scanner.Err()
}

type navChunk struct {
	Index int
	From  time.Time
	To    time.Time
}

// FetchNavHistory fetches historical NAV data for a scheme across any arbitrary time window (from days to 30+ years)
// by automatically partitioning the window into <= 2-year chunks compliant with AMFI upstream limits.
// It iterates backwards from 'to' down to 'from', executing chunks in parallel (bounded to max 3 concurrent workers).
// It applies a 10-day inception cutoff limit: if any chunk returns 0 records or its earliest recorded NAV is more
// than 10 calendar days after the chunk start date, that date is recognized as the scheme inception point,
// immediately terminating/canceling all older chunks.
// If an optional onChunk callback is provided, it is invoked immediately as each individual chunk completes,
// ensuring incremental "store-as-we-go" persistence so already fetched dates are never lost or re-queried.
func (s *AmfiService) FetchNavHistory(
	ctx context.Context,
	schemeCode int,
	from, to time.Time,
	onChunk ...func(records []models.NavHistory),
) ([]models.NavHistory, error) {
	if schemeCode <= 0 {
		return nil, fmt.Errorf("invalid scheme code %d", schemeCode)
	}

	if to.IsZero() {
		to = IndiaToday()
	}
	if from.IsZero() {
		from = to.AddDate(-30, 0, 0)
	}
	if from.After(to) {
		from, to = to, from
	}

	// Partition date range into <= 2-year sliding intervals backwards from 'to' to 'from'.
	// Benchmark measurements confirm AMFI latency scales super-linearly:
	// - 1-Year chunk: ~4.3s
	// - 2-Year chunk: ~7.7s
	// - 4-Year chunk: ~17.8s - 50+s (frequently hits upstream timeouts)
	// 2-Year chunks provide optimal latency and throughput while bounded parallel concurrency streams chunks.
	var chunks []navChunk
	currTo := to
	idx := 0
	for !currTo.Before(from) {
		currFrom := currTo.AddDate(-2, 0, 1)
		if currFrom.Before(from) {
			currFrom = from
		}
		chunks = append(chunks, navChunk{Index: idx, From: currFrom, To: currTo})
		idx++
		currTo = currFrom.AddDate(0, 0, -1)
	}

	if len(chunks) == 0 {
		return nil, nil
	}

	// Fast-path for single chunk (common for <= 2 year requests)
	if len(chunks) == 1 {
		records, err := s.fetchNavHistoryChunk(ctx, schemeCode, chunks[0].From, chunks[0].To)
		if err != nil {
			return nil, err
		}
		sort.Slice(records, func(i, j int) bool {
			return records[i].Date.Before(records[j].Date)
		})
		if len(onChunk) > 0 && onChunk[0] != nil && len(records) > 0 {
			onChunk[0](records)
		}
		return records, nil
	}

	// Multi-chunk parallel execution with 10-day inception cutoff
	const maxConcurrency = 3
	sem := make(chan struct{}, maxConcurrency)

	childCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	chunkCtxs := make([]context.Context, len(chunks))
	chunkCancels := make([]context.CancelFunc, len(chunks))
	for i := range chunks {
		chunkCtxs[i], chunkCancels[i] = context.WithCancel(childCtx)
		defer chunkCancels[i]()
	}

	var cutoffMutex sync.Mutex
	cutoffChunkIndex := len(chunks)

	setCutoff := func(chunkIdx int) {
		cutoffMutex.Lock()
		defer cutoffMutex.Unlock()
		if chunkIdx < cutoffChunkIndex {
			cutoffChunkIndex = chunkIdx
			// Cancel all older chunks (j > chunkIdx)
			for j := chunkIdx + 1; j < len(chunks); j++ {
				chunkCancels[j]()
			}
		}
	}

	type chunkResult struct {
		index   int
		records []models.NavHistory
	}
	resultsChan := make(chan chunkResult, len(chunks))
	var wg sync.WaitGroup

	for _, ch := range chunks {
		cutoffMutex.Lock()
		shouldSkip := ch.Index >= cutoffChunkIndex
		cutoffMutex.Unlock()
		if shouldSkip {
			continue
		}

		wg.Add(1)
		go func(chunk navChunk) {
			defer wg.Done()

			select {
			case <-chunkCtxs[chunk.Index].Done():
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			cutoffMutex.Lock()
			if chunk.Index >= cutoffChunkIndex {
				cutoffMutex.Unlock()
				return
			}
			cutoffMutex.Unlock()

			records, err := s.fetchNavHistoryChunk(chunkCtxs[chunk.Index], schemeCode, chunk.From, chunk.To)
			if err != nil {
				return
			}

			// If no records in this chunk, older chunks have reached pre-inception
			if len(records) == 0 {
				setCutoff(chunk.Index)
				return
			}

			sort.Slice(records, func(i, j int) bool {
				return records[i].Date.Before(records[j].Date)
			})

			// 10-day inception cutoff rule:
			// If more than 10 calendar days of NAV data are missing between chunk.From and earliest record,
			// the scheme was not operational before this earliest record. We hit the beginning point!
			earliest := records[0].Date
			if earliest.Sub(chunk.From).Hours() > 10*24 {
				setCutoff(chunk.Index)
			}

			// Store-as-we-go: notify chunk saver immediately
			if len(onChunk) > 0 && onChunk[0] != nil && len(records) > 0 {
				onChunk[0](records)
			}

			resultsChan <- chunkResult{index: chunk.Index, records: records}
		}(ch)
	}

	wg.Wait()
	close(resultsChan)

	recordMap := make(map[string]models.NavHistory)
	for res := range resultsChan {
		cutoffMutex.Lock()
		isPastCutoff := res.index > cutoffChunkIndex
		cutoffMutex.Unlock()
		if isPastCutoff {
			continue
		}
		for _, r := range res.records {
			dateKey := r.Date.Format(DateFormatYYYYMMDD)
			recordMap[dateKey] = r
		}
	}

	if len(recordMap) == 0 {
		return nil, nil
	}

	result := make([]models.NavHistory, 0, len(recordMap))
	for _, r := range recordMap {
		result = append(result, r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.Before(result[j].Date)
	})

	return result, nil
}

func (s *AmfiService) fetchNavHistoryChunk(ctx context.Context, schemeCode int, from, to time.Time) ([]models.NavHistory, error) {
	baseUrl := AmfiNavHistoryURL
	if s.integrationService != nil {
		baseUrl = s.integrationService.GetActiveUrl(ctx, "AMFI_NAV_HISTORY_URL", AmfiNavHistoryURL)
	}

	// Query AMFI modern history endpoint
	url := fmt.Sprintf("%s?query_type=historical_period&from_date=%s&to_date=%s&sd_id=%d",
		baseUrl, FormatDateYYYYMMDD(from), FormatDateYYYYMMDD(to), schemeCode)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(500 * time.Millisecond):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "NAVigator/2.0")

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("AMFI history API returned status %d", resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		return ParseAmfiNavHistoryJson(body, schemeCode)
	}

	return nil, lastErr
}

// ParseAmfiNavHistoryJson parses the modern AMFI historical NAV JSON response payload.
func ParseAmfiNavHistoryJson(body []byte, schemeCode int) ([]models.NavHistory, error) {
	var apiResp AmfiNavHistoryApiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	if apiResp.Error != "" {
		return nil, fmt.Errorf("AMFI error: %s", apiResp.Error)
	}

	if apiResp.Data == nil || len(apiResp.Data.NavGroups) == 0 {
		return nil, nil
	}

	var list []models.NavHistory
	for _, grp := range apiResp.Data.NavGroups {
		for _, rec := range grp.HistoricalRecords {
			d, err := ParseDate(rec.Date)
			if err != nil {
				continue
			}

			var val float64
			switch v := rec.Nav.(type) {
			case float64:
				val = v
			case float32:
				val = float64(v)
			case int:
				val = float64(v)
			case int64:
				val = float64(v)
			case string:
				parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
				if err != nil {
					continue
				}
				val = parsed
			case json.Number:
				parsed, err := v.Float64()
				if err != nil {
					continue
				}
				val = parsed
			default:
				continue
			}

			if val <= 0 {
				continue
			}

			list = append(list, models.NavHistory{
				SchemeCode: schemeCode,
				Date:       d,
				Nav:        val,
			})
		}
	}
	return list, nil
}

