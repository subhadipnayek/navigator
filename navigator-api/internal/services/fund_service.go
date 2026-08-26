package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FundService struct {
	db          *gorm.DB
	amfiService *AmfiService
}

func NewFundService(db *gorm.DB, amfi *AmfiService) *FundService {
	return &FundService{db: db, amfiService: amfi}
}

func (s *FundService) GetAllFunds(ctx context.Context) ([]models.FundDto, error) {
	var funds []models.Fund
	err := s.db.WithContext(ctx).
		Preload("NavSyncTracker").
		Order("scheme_name asc").
		Find(&funds).Error
	if err != nil {
		return nil, err
	}

	return s.mapFundsToDtos(funds), nil
}

func (s *FundService) GetFund(ctx context.Context, schemeCode int) (*models.FundDto, error) {
	var fund models.Fund
	err := s.db.WithContext(ctx).
		Preload("NavSyncTracker").
		Where("scheme_code = ?", schemeCode).
		First(&fund).Error
	if err != nil {
		return nil, err
	}

	dtos := s.mapFundsToDtos([]models.Fund{fund})
	if len(dtos) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &dtos[0], nil
}

func (s *FundService) SearchFunds(ctx context.Context, query string) ([]models.FundDto, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	var funds []models.Fund
	dbQuery := s.db.WithContext(ctx).Preload("NavSyncTracker")

	if code, err := strconv.Atoi(query); err == nil {
		dbQuery = dbQuery.Where("scheme_code = ? OR scheme_name ILIKE ? OR fund_house ILIKE ?", code, "%"+query+"%", "%"+query+"%")
	} else {
		dbQuery = dbQuery.Where("scheme_name ILIKE ? OR fund_house ILIKE ?", "%"+query+"%", "%"+query+"%")
	}

	err := dbQuery.Order("scheme_name asc").Limit(100).Find(&funds).Error
	if err != nil {
		return nil, err
	}

	return s.mapFundsToDtos(funds), nil
}

func (s *FundService) GetFundCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.Fund{}).Count(&count).Error
	return count, err
}

func (s *FundService) SyncFundMasterList(ctx context.Context) (*models.FundSyncResultDto, error) {
	startTime := time.Now()
	log.Println("Starting AMFI Fund Master sync...")
	funds, err := s.amfiService.FetchFundMasterList(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch AMFI fund master: %w", err)
	}

	if len(funds) == 0 {
		return &models.FundSyncResultDto{
			TotalParsed: 0,
			Total:       0,
			Duration:    time.Since(startTime).String(),
		}, nil
	}

	batchSize := 1000
	inserted := 0
	updated := 0

	for i := 0; i < len(funds); i += batchSize {
		end := i + batchSize
		if end > len(funds) {
			end = len(funds)
		}
		batch := funds[i:end]

		// Bulk upsert into Postgres
		err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "scheme_code"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"scheme_name",
				"fund_house",
				"scheme_type",
				"scheme_category",
				"isin_growth",
				"isin_div_reinvestment",
				"updated_at",
			}),
		}).Create(&batch).Error

		if err != nil {
			log.Printf("Error upserting fund batch %d-%d: %v", i, end, err)
			continue
		}
		inserted += len(batch)
	}

	// Also fetch and upsert latest published NAV snapshots from NAVAll.txt
	navSnapshots, navErr := s.amfiService.FetchLatestNavSnapshots(ctx)
	if navErr == nil && len(navSnapshots) > 0 {
		nowIst := IndiaNow()
		var trackers []models.NavSyncTracker
		for _, snap := range navSnapshots {
			navVal := snap.Nav
			navDate := snap.Date
			trackers = append(trackers, models.NavSyncTracker{
				SchemeCode:              snap.SchemeCode,
				LatestNav:               &navVal,
				LatestNavDate:           &navDate,
				LastLatestNavCheckAtIst: &nowIst,
			})
		}

		for i := 0; i < len(trackers); i += batchSize {
			end := i + batchSize
			if end > len(trackers) {
				end = len(trackers)
			}
			batch := trackers[i:end]
			_ = s.db.WithContext(ctx).Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "scheme_code"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"latest_nav",
					"latest_nav_date",
					"last_latest_nav_check_at_ist",
					"updated_at",
				}),
			}).Create(&batch).Error
		}
	}

	durationStr := time.Since(startTime).String()
	log.Printf("AMFI Fund Master sync complete: processed %d funds in %s", len(funds), durationStr)
	return &models.FundSyncResultDto{
		TotalParsed: len(funds),
		Added:       inserted,
		Inserted:    inserted,
		Updated:     updated,
		Unchanged:   len(funds) - inserted - updated,
		Total:       len(funds),
		Duration:    durationStr,
	}, nil
}

func (s *FundService) mapFundsToDtos(funds []models.Fund) []models.FundDto {
	res := make([]models.FundDto, len(funds))
	for i, f := range funds {
		dto := models.FundDto{
			SchemeCode:          f.SchemeCode,
			SchemeName:          f.SchemeName,
			FundHouse:           f.FundHouse,
			SchemeType:          f.SchemeType,
			SchemeCategory:      f.SchemeCategory,
			IsinGrowth:          f.IsinGrowth,
			IsinDivReinvestment: f.IsinDivReinvestment,
		}
		if f.NavSyncTracker != nil {
			dto.LatestNav = f.NavSyncTracker.LatestNav
			dto.LatestNavDate = FormatDatePtr(f.NavSyncTracker.LatestNavDate)
			dto.NavStartDate = FormatDatePtr(f.NavSyncTracker.NavStartDate)
			dto.NavEndDate = FormatDatePtr(f.NavSyncTracker.NavEndDate)
			dto.NavHistoryCount = f.NavSyncTracker.NavHistoryCount
		}
		res[i] = dto
	}
	return res
}
