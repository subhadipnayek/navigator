package services

import (
	"bufio"
	"context"
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
	httpClient *http.Client
}

func NewAmfiService() *AmfiService {
	return &AmfiService{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AmfiMasterURL, nil)
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

	return parseAmfiMasterData(resp.Body)
}

func parseAmfiMasterData(r io.Reader) ([]models.Fund, error) {
	var funds []models.Fund
	scanner := bufio.NewScanner(r)

	// Format: Scheme Code,Scheme Name,ISIN Div Payout/ ISIN Growth,ISIN Div Reinvestment,Fund House,Scheme Type,Scheme Category
	isFirstLine := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if isFirstLine {
			isFirstLine = false
			if strings.HasPrefix(strings.ToLower(line), "scheme code") {
				continue
			}
		}

		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		schemeCode, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || schemeCode <= 0 {
			continue
		}

		schemeName := strings.TrimSpace(parts[1])
		if schemeName == "" {
			continue
		}

		var isinGrowth, isinDivReinvest, fundHouse, schemeType, schemeCategory *string

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

		now := time.Now().UTC()
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

	return funds, scanner.Err()
}

func (s *AmfiService) FetchLatestNavSnapshots(ctx context.Context) ([]LatestNavSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AmfiLatestNavURL, nil)
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

	return parseLatestNavAll(resp.Body)
}

func parseLatestNavAll(r io.Reader) ([]LatestNavSnapshot, error) {
	var snapshots []LatestNavSnapshot
	scanner := bufio.NewScanner(r)

	// Format: Scheme Code;ISIN Div Payout/ ISIN Growth;ISIN Div Reinvestment;Scheme Name;Net Asset Value;Date
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

		navStr := strings.TrimSpace(parts[4])
		nav, err := strconv.ParseFloat(navStr, 64)
		if err != nil || nav <= 0 {
			continue
		}

		dateStr := strings.TrimSpace(parts[5])
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
	// Query AMFI history chunk
	url := fmt.Sprintf("%s?schemeCode=%d&fromDate=%s&toDate=%s",
		AmfiNavHistoryURL, schemeCode, FormatDateDDMMYYYY(from), FormatDateDDMMYYYY(to))

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
