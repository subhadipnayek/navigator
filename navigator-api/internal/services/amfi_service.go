package services

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
			Timeout: 60 * time.Second,
		},
		integrationService: intSvc,
	}
}

type AmfiNavHistoryApiResponse struct {
	Data []struct {
		Date string `json:"date"`
		Nav  string `json:"nav"`
	} `json:"data"`
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

func (s *AmfiService) FetchNavHistory(ctx context.Context, schemeCode int, from, to time.Time) ([]models.NavHistory, error) {
	baseUrl := AmfiNavHistoryURL
	if s.integrationService != nil {
		baseUrl = s.integrationService.GetActiveUrl(ctx, "AMFI_NAV_HISTORY_URL", AmfiNavHistoryURL)
	}

	// Query AMFI history chunk
	url := fmt.Sprintf("%s?schemeCode=%d&fromDate=%s&toDate=%s",
		baseUrl, schemeCode, FormatDateDDMMYYYY(from), FormatDateDDMMYYYY(to))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "NAVigator/2.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		// Fallback or empty
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AMFI history API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp AmfiNavHistoryApiResponse
	if err := json.Unmarshal(body, &apiResp); err == nil && len(apiResp.Data) > 0 {
		var list []models.NavHistory
		for _, item := range apiResp.Data {
			d, err := ParseDate(item.Date)
			if err != nil {
				continue
			}
			val, err := strconv.ParseFloat(item.Nav, 64)
			if err != nil || val <= 0 {
				continue
			}
			list = append(list, models.NavHistory{
				SchemeCode: schemeCode,
				Date:       d,
				Nav:        val,
			})
		}
		return list, nil
	}

	return nil, nil
}
