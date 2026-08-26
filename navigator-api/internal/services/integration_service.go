package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type IntegrationService struct {
	db         *gorm.DB
	httpClient *http.Client
}

func NewIntegrationService(db *gorm.DB) *IntegrationService {
	return &IntegrationService{
		db: db,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *IntegrationService) GetEndpoints(ctx context.Context) (*models.ApiEndpointsSummaryDto, error) {
	var endpoints []models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Order("key asc").Find(&endpoints).Error; err != nil {
		return nil, err
	}

	healthy := 0
	degraded := 0
	down := 0
	var totalLatency int64

	dtos := make([]models.ApiEndpointConfigDto, len(endpoints))
	for i, e := range endpoints {
		var lastCheck *string
		if e.LastHealthCheckAtUtc != nil {
			str := e.LastHealthCheckAtUtc.Format(time.RFC3339)
			lastCheck = &str
		}

		dtos[i] = models.ApiEndpointConfigDto{
			Key:                  e.Key,
			Name:                 e.Name,
			Description:          e.Description,
			Url:                  e.Url,
			DefaultUrl:           e.DefaultUrl,
			Method:               e.Method,
			HeadersJson:          e.HeadersJson,
			PayloadTemplate:      e.PayloadTemplate,
			IsActive:             e.IsActive,
			LastHealthCheckAtUtc: lastCheck,
			LastHealthStatus:     e.LastHealthStatus,
			LastStatusCode:       e.LastStatusCode,
			LastLatencyMs:        e.LastLatencyMs,
			LastErrorMessage:     e.LastErrorMessage,
			UpdatedAtUtc:         e.UpdatedAtUtc.Format(time.RFC3339),
		}

		switch e.LastHealthStatus {
		case "Healthy":
			healthy++
		case "Degraded":
			degraded++
		case "Down":
			down++
		}
		totalLatency += e.LastLatencyMs
	}

	var avgLatency int64
	if len(endpoints) > 0 {
		avgLatency = totalLatency / int64(len(endpoints))
	}

	return &models.ApiEndpointsSummaryDto{
		TotalEndpoints: len(endpoints),
		HealthyCount:   healthy,
		DegradedCount:  degraded,
		DownCount:      down,
		AvgLatencyMs:   avgLatency,
		Endpoints:      dtos,
	}, nil
}

func (s *IntegrationService) GetActiveUrl(ctx context.Context, key string, fallback string) string {
	var ep models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Where("key = ? AND is_active = true", key).First(&ep).Error; err == nil {
		if strings.TrimSpace(ep.Url) != "" {
			return strings.TrimSpace(ep.Url)
		}
	}
	return fallback
}

func (s *IntegrationService) UpdateEndpoint(ctx context.Context, key string, req *models.UpdateApiEndpointConfigRequest) (*models.ApiEndpointConfigDto, error) {
	var ep models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&ep).Error; err != nil {
		return nil, fmt.Errorf("endpoint '%s' not found", key)
	}

	ep.Url = strings.TrimSpace(req.Url)
	if req.Method != "" {
		ep.Method = strings.ToUpper(strings.TrimSpace(req.Method))
	}
	if req.HeadersJson != "" {
		ep.HeadersJson = strings.TrimSpace(req.HeadersJson)
	}
	if req.PayloadTemplate != "" {
		ep.PayloadTemplate = strings.TrimSpace(req.PayloadTemplate)
	}
	if req.IsActive != nil {
		ep.IsActive = *req.IsActive
	}
	ep.UpdatedAtUtc = time.Now().UTC()

	if err := s.db.WithContext(ctx).Save(&ep).Error; err != nil {
		return nil, err
	}

	var lastCheck *string
	if ep.LastHealthCheckAtUtc != nil {
		str := ep.LastHealthCheckAtUtc.Format(time.RFC3339)
		lastCheck = &str
	}

	return &models.ApiEndpointConfigDto{
		Key:                  ep.Key,
		Name:                 ep.Name,
		Description:          ep.Description,
		Url:                  ep.Url,
		DefaultUrl:           ep.DefaultUrl,
		Method:               ep.Method,
		HeadersJson:          ep.HeadersJson,
		PayloadTemplate:      ep.PayloadTemplate,
		IsActive:             ep.IsActive,
		LastHealthCheckAtUtc: lastCheck,
		LastHealthStatus:     ep.LastHealthStatus,
		LastStatusCode:       ep.LastStatusCode,
		LastLatencyMs:        ep.LastLatencyMs,
		LastErrorMessage:     ep.LastErrorMessage,
		UpdatedAtUtc:         ep.UpdatedAtUtc.Format(time.RFC3339),
	}, nil
}

func (s *IntegrationService) ResetEndpoint(ctx context.Context, key string) (*models.ApiEndpointConfigDto, error) {
	var ep models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&ep).Error; err != nil {
		return nil, fmt.Errorf("endpoint '%s' not found", key)
	}

	ep.Url = ep.DefaultUrl
	ep.Method = "GET"
	ep.HeadersJson = `{"User-Agent": "NAVigator/2.0"}`
	ep.IsActive = true
	ep.UpdatedAtUtc = time.Now().UTC()

	if err := s.db.WithContext(ctx).Save(&ep).Error; err != nil {
		return nil, err
	}

	return s.UpdateEndpoint(ctx, key, &models.UpdateApiEndpointConfigRequest{
		Url:             ep.DefaultUrl,
		Method:          "GET",
		HeadersJson:     ep.HeadersJson,
		PayloadTemplate: ep.PayloadTemplate,
		IsActive:        &ep.IsActive,
	})
}

func (s *IntegrationService) TestEndpoint(ctx context.Context, key string) (*models.ApiEndpointTestResultDto, error) {
	var ep models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&ep).Error; err != nil {
		return nil, fmt.Errorf("endpoint '%s' not found", key)
	}

	method := ep.Method
	if method == "" {
		method = "GET"
	}

	// Prepare request
	var reqBody io.Reader
	targetUrl := ep.Url

	if method == "GET" && ep.PayloadTemplate != "" && !strings.Contains(targetUrl, "?") {
		// Substitute sample variables for testing
		sampleQuery := ep.PayloadTemplate
		sampleQuery = strings.ReplaceAll(sampleQuery, "{{schemeCode}}", "119551")
		sampleQuery = strings.ReplaceAll(sampleQuery, "{{fromDate}}", "01-01-2024")
		sampleQuery = strings.ReplaceAll(sampleQuery, "{{toDate}}", "25-08-2026")
		targetUrl = targetUrl + "?" + sampleQuery
	} else if method == "POST" && ep.PayloadTemplate != "" {
		reqBody = bytes.NewBufferString(ep.PayloadTemplate)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, targetUrl, reqBody)
	if err != nil {
		now := time.Now().UTC()
		errMsg := err.Error()
		ep.LastHealthCheckAtUtc = &now
		ep.LastHealthStatus = "Down"
		ep.LastStatusCode = 0
		ep.LastLatencyMs = 0
		ep.LastErrorMessage = &errMsg
		_ = s.db.WithContext(ctx).Save(&ep).Error

		return &models.ApiEndpointTestResultDto{
			Key:          key,
			Url:          targetUrl,
			Method:       method,
			StatusCode:   0,
			LatencyMs:    0,
			HealthStatus: "Down",
			ErrorMessage: errMsg,
			Success:      false,
		}, nil
	}

	// Set headers
	if ep.HeadersJson != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(ep.HeadersJson), &headers); err == nil {
			for k, v := range headers {
				httpReq.Header.Set(k, v)
			}
		}
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "NAVigator/2.0")
	}

	start := time.Now()
	resp, err := s.httpClient.Do(httpReq)
	latency := time.Since(start).Milliseconds()

	now := time.Now().UTC()
	ep.LastHealthCheckAtUtc = &now
	ep.LastLatencyMs = latency

	if err != nil {
		errMsg := err.Error()
		ep.LastHealthStatus = "Down"
		ep.LastStatusCode = 0
		ep.LastErrorMessage = &errMsg
		_ = s.db.WithContext(ctx).Save(&ep).Error

		return &models.ApiEndpointTestResultDto{
			Key:          key,
			Url:          targetUrl,
			Method:       method,
			StatusCode:   0,
			LatencyMs:    latency,
			HealthStatus: "Down",
			ErrorMessage: errMsg,
			Success:      false,
		}, nil
	}
	defer resp.Body.Close()

	ep.LastStatusCode = resp.StatusCode

	// Read preview snippet (first 500 bytes)
	buf := make([]byte, 500)
	n, _ := io.ReadFull(resp.Body, buf)
	snippet := string(buf[:n])

	healthStatus := "Healthy"
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		healthStatus = "Down"
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)
		ep.LastErrorMessage = &errMsg
	} else if latency > 2000 {
		healthStatus = "Degraded"
		ep.LastErrorMessage = nil
	} else {
		ep.LastErrorMessage = nil
	}

	ep.LastHealthStatus = healthStatus
	_ = s.db.WithContext(ctx).Save(&ep).Error

	return &models.ApiEndpointTestResultDto{
		Key:             key,
		Url:             targetUrl,
		Method:          method,
		StatusCode:      resp.StatusCode,
		LatencyMs:       latency,
		HealthStatus:    healthStatus,
		ResponseSnippet: snippet,
		Success:         resp.StatusCode >= 200 && resp.StatusCode < 400,
	}, nil
}

func (s *IntegrationService) HealthCheckAll(ctx context.Context) (*models.ApiEndpointsSummaryDto, error) {
	var endpoints []models.ApiEndpointConfig
	if err := s.db.WithContext(ctx).Find(&endpoints).Error; err != nil {
		return nil, err
	}

	for _, ep := range endpoints {
		_, _ = s.TestEndpoint(ctx, ep.Key)
	}

	return s.GetEndpoints(ctx)
}
