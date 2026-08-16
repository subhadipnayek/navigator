package services

import (
	"context"
	"math"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
)

type CorporateActionService struct {
	db *gorm.DB
}

func NewCorporateActionService(db *gorm.DB) *CorporateActionService {
	return &CorporateActionService{db: db}
}

func (s *CorporateActionService) GetActions(ctx context.Context, schemeCode int) ([]models.CorporateActionDto, error) {
	var actions []models.CorporateAction
	err := s.db.WithContext(ctx).
		Where("scheme_code = ?", schemeCode).
		Order("action_date asc").
		Find(&actions).Error
	if err != nil {
		return nil, err
	}

	res := make([]models.CorporateActionDto, len(actions))
	for i, a := range actions {
		res[i] = models.CorporateActionDto{
			Id:               a.Id,
			SchemeCode:       a.SchemeCode,
			ActionDate:       FormatDateDDMMYYYY(a.ActionDate),
			ActionType:       a.ActionType,
			AdjustmentFactor: a.AdjustmentFactor,
			OldUnits:         a.OldUnits,
			NewUnits:         a.NewUnits,
			Ratio:            a.Ratio,
			ConfidenceScore:  a.ConfidenceScore,
			Notes:            a.Notes,
			IsAutoDetected:   a.IsAutoDetected,
			IsVerified:       a.IsVerified,
		}
	}
	return res, nil
}

func (s *CorporateActionService) DetectAnomalies(ctx context.Context, threshold float64, minDaysSincePrevious int, schemeCodeFilter *int) ([]models.NavAnomalyDto, error) {
	if threshold < 5 {
		threshold = 30
	}
	if minDaysSincePrevious < 1 {
		minDaysSincePrevious = 1
	}

	// Fetch funds to process
	var funds []models.Fund
	query := s.db.WithContext(ctx).Order("scheme_code asc")
	if schemeCodeFilter != nil {
		query = query.Where("scheme_code = ?", *schemeCodeFilter)
	}
	if err := query.Find(&funds).Error; err != nil {
		return nil, err
	}

	var anomalies []models.NavAnomalyDto

	for _, f := range funds {
		var navs []models.NavHistory
		if err := s.db.WithContext(ctx).Where("scheme_code = ?", f.SchemeCode).Order("date asc").Find(&navs).Error; err != nil || len(navs) < 2 {
			continue
		}

		// Load existing corporate actions for this fund
		var existingActions []models.CorporateAction
		_ = s.db.WithContext(ctx).Where("scheme_code = ?", f.SchemeCode).Find(&existingActions).Error
		existingMap := make(map[string]models.CorporateAction)
		for _, ea := range existingActions {
			existingMap[FormatDateDDMMYYYY(ea.ActionDate)] = ea
		}

		for i := 1; i < len(navs); i++ {
			prev := navs[i-1]
			curr := navs[i]

			if prev.Nav <= 0 || curr.Nav <= 0 {
				continue
			}

			daysDiff := DaysBetween(prev.Date, curr.Date)
			if daysDiff < minDaysSincePrevious {
				continue
			}

			pctChange := (curr.Nav - prev.Nav) / prev.Nav * 100.0
			if math.Abs(pctChange) >= threshold {
				suggestedAction := "Split"
				suggestedFactor := prev.Nav / curr.Nav
				confidence := 0.85
				explanation := "Large day-over-day price decrease suggests stock/fund split."

				if pctChange > 0 {
					suggestedAction = "Merger"
					suggestedFactor = curr.Nav / prev.Nav
					explanation = "Large price increase suggests fund merger or consolidation."
				}

				// Check nearest integer ratio
				nearestInt := math.Round(suggestedFactor)
				if math.Abs(suggestedFactor-nearestInt) < 0.08 && nearestInt >= 2 {
					suggestedFactor = nearestInt
					confidence = 0.95
				}

				anomalyDateStr := FormatDateDDMMYYYY(curr.Date)
				existing, hasExisting := existingMap[anomalyDateStr]

				dto := models.NavAnomalyDto{
					SchemeCode:                 f.SchemeCode,
					SchemeName:                 f.SchemeName,
					PreviousDate:               FormatDateDDMMYYYY(prev.Date),
					PreviousNav:                prev.Nav,
					AnomalyDate:                anomalyDateStr,
					AnomalyNav:                 curr.Nav,
					PercentChange:              roundFloat(pctChange, 2),
					DaysSincePrevious:          daysDiff,
					SuggestedAction:            suggestedAction,
					SuggestedFactor:            roundFloat(suggestedFactor, 4),
					Confidence:                 confidence,
					Explanation:                explanation,
					HasExistingCorporateAction: hasExisting,
				}

				if hasExisting {
					dto.ExistingActionId = &existing.Id
					dto.ExistingActionType = &existing.ActionType
					dto.ExistingAdjustmentFactor = &existing.AdjustmentFactor
				}

				anomalies = append(anomalies, dto)
			}
		}
	}

	return anomalies, nil
}

func (s *CorporateActionService) GetReviewQueue(
	ctx context.Context,
	threshold float64,
	minGapDays int,
	filter, priority string,
	includeRecorded bool,
	schemeCode *int,
	q, actionType *string,
	minConfidence *float64,
	limit int,
) (*models.CorporateActionReviewQueueDto, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	items, err := s.DetectAnomalies(ctx, threshold, minGapDays, schemeCode)
	if err != nil {
		return nil, err
	}

	var filtered []models.NavAnomalyDto
	pendingCount := 0
	recordedCount := 0
	eligibleAuto := 0

	for _, item := range items {
		if item.HasExistingCorporateAction {
			recordedCount++
		} else {
			pendingCount++
		}
		if item.Confidence >= 0.90 && !item.HasExistingCorporateAction {
			eligibleAuto++
		}

		if !includeRecorded && item.HasExistingCorporateAction {
			continue
		}

		// Filter matching
		if q != nil && *q != "" {
			query := strings.ToLower(*q)
			if !strings.Contains(strings.ToLower(item.SchemeName), query) {
				continue
			}
		}

		if actionType != nil && *actionType != "" && !strings.EqualFold(*actionType, "all") {
			if !strings.EqualFold(item.SuggestedAction, *actionType) {
				continue
			}
		}

		if minConfidence != nil && item.Confidence < *minConfidence {
			continue
		}

		switch strings.ToLower(filter) {
		case "recorded":
			if !item.HasExistingCorporateAction {
				continue
			}
		case "needs-review":
			if item.HasExistingCorporateAction {
				continue
			}
		case "large-moves":
			if math.Abs(item.PercentChange) < 50 {
				continue
			}
		}

		filtered = append(filtered, item)
		if len(filtered) >= limit {
			break
		}
	}

	return &models.CorporateActionReviewQueueDto{
		MovementThreshold:     threshold,
		MinGapDays:            minGapDays,
		Filter:                filter,
		Priority:              priority,
		TotalItems:            len(filtered),
		PendingReviewCount:    pendingCount,
		RecordedCount:         recordedCount,
		EligibleForAutoRecord: eligibleAuto,
		Items:                 filtered,
	}, nil
}

func (s *CorporateActionService) CreateAction(ctx context.Context, req *models.CreateCorporateActionRequest) (*models.CorporateActionDto, error) {
	actionDate, err := ParseDate(req.ActionDate)
	if err != nil {
		return nil, err
	}

	action := models.CorporateAction{
		SchemeCode:       req.SchemeCode,
		ActionDate:       actionDate,
		ActionType:       req.ActionType,
		AdjustmentFactor: req.AdjustmentFactor,
		OldUnits:         req.OldUnits,
		NewUnits:         req.NewUnits,
		Ratio:            req.Ratio,
		Notes:            req.Notes,
		IsVerified:       req.IsVerified,
		CreatedAtUtc:     time.Now().UTC(),
		UpdatedAtUtc:     time.Now().UTC(),
	}

	if err := s.db.WithContext(ctx).Create(&action).Error; err != nil {
		return nil, err
	}

	return &models.CorporateActionDto{
		Id:               action.Id,
		SchemeCode:       action.SchemeCode,
		ActionDate:       FormatDateDDMMYYYY(action.ActionDate),
		ActionType:       action.ActionType,
		AdjustmentFactor: action.AdjustmentFactor,
		OldUnits:         action.OldUnits,
		NewUnits:         action.NewUnits,
		Ratio:            action.Ratio,
		Notes:            action.Notes,
		IsVerified:       action.IsVerified,
	}, nil
}

func (s *CorporateActionService) UpdateAction(ctx context.Context, id int64, req *models.UpdateCorporateActionRequest) (*models.CorporateActionDto, error) {
	actionDate, err := ParseDate(req.ActionDate)
	if err != nil {
		return nil, err
	}

	var action models.CorporateAction
	if err := s.db.WithContext(ctx).First(&action, id).Error; err != nil {
		return nil, err
	}

	action.ActionDate = actionDate
	action.ActionType = req.ActionType
	action.AdjustmentFactor = req.AdjustmentFactor
	action.OldUnits = req.OldUnits
	action.NewUnits = req.NewUnits
	action.Ratio = req.Ratio
	action.Notes = req.Notes
	action.IsVerified = req.IsVerified
	action.UpdatedAtUtc = time.Now().UTC()

	if err := s.db.WithContext(ctx).Save(&action).Error; err != nil {
		return nil, err
	}

	return &models.CorporateActionDto{
		Id:               action.Id,
		SchemeCode:       action.SchemeCode,
		ActionDate:       FormatDateDDMMYYYY(action.ActionDate),
		ActionType:       action.ActionType,
		AdjustmentFactor: action.AdjustmentFactor,
		OldUnits:         action.OldUnits,
		NewUnits:         action.NewUnits,
		Ratio:            action.Ratio,
		Notes:            action.Notes,
		IsVerified:       action.IsVerified,
	}, nil
}

func (s *CorporateActionService) DeleteAction(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&models.CorporateAction{}, id).Error
}

func (s *CorporateActionService) AutoRecordAnomalies(ctx context.Context, minConfidence float64) (*models.AnomalyAutoRecordResultDto, error) {
	if minConfidence <= 0 {
		minConfidence = 0.90
	}

	anomalies, err := s.DetectAnomalies(ctx, 30, 1, nil)
	if err != nil {
		return nil, err
	}

	created := 0
	skipped := 0

	for _, a := range anomalies {
		if a.HasExistingCorporateAction || a.Confidence < minConfidence {
			skipped++
			continue
		}

		actDate, err := ParseDate(a.AnomalyDate)
		if err != nil {
			skipped++
			continue
		}

		ca := models.CorporateAction{
			SchemeCode:       a.SchemeCode,
			ActionDate:       actDate,
			ActionType:       a.SuggestedAction,
			AdjustmentFactor: a.SuggestedFactor,
			ConfidenceScore:  &a.Confidence,
			Notes:            &a.Explanation,
			IsAutoDetected:   true,
			IsVerified:       false,
			CreatedAtUtc:     time.Now().UTC(),
			UpdatedAtUtc:     time.Now().UTC(),
		}

		if err := s.db.WithContext(ctx).Create(&ca).Error; err == nil {
			created++
		} else {
			skipped++
		}
	}

	return &models.AnomalyAutoRecordResultDto{
		TotalProcessed: len(anomalies),
		CreatedCount:   created,
		SkippedCount:   skipped,
	}, nil
}
