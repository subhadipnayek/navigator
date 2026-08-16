package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db          *gorm.DB
	fundService *services.FundService
	navService  *services.NavService
}

func NewAdminHandler(db *gorm.DB, fundSvc *services.FundService, navSvc *services.NavService) *AdminHandler {
	return &AdminHandler{db: db, fundService: fundSvc, navService: navSvc}
}

func (h *AdminHandler) GetFundsSummary(c *gin.Context) {
	ctx := c.Request.Context()
	var totalFunds int64
	var navRecords int64
	var corporateActions int64

	_ = h.db.WithContext(ctx).Model(&models.Fund{}).Count(&totalFunds).Error
	_ = h.db.WithContext(ctx).Model(&models.NavHistory{}).Count(&navRecords).Error
	_ = h.db.WithContext(ctx).Model(&models.CorporateAction{}).Count(&corporateActions).Error

	var fundsWithNav int64
	_ = h.db.WithContext(ctx).Model(&models.NavHistory{}).Distinct("scheme_code").Count(&fundsWithNav).Error

	var oldestNav, latestNav *time.Time
	var oldestRow, latestRow models.NavHistory
	if err := h.db.WithContext(ctx).Order("date asc").First(&oldestRow).Error; err == nil {
		oldestNav = &oldestRow.Date
	}
	if err := h.db.WithContext(ctx).Order("date desc").First(&latestRow).Error; err == nil {
		latestNav = &latestRow.Date
	}

	c.JSON(http.StatusOK, models.AdminDatabaseSummaryDto{
		TotalFunds:             int(totalFunds),
		FundsWithNavHistory:    int(fundsWithNav),
		FundsWithoutNavHistory: int(totalFunds - fundsWithNav),
		NavHistoryRecords:      navRecords,
		CorporateActions:       int(corporateActions),
		OldestNavDate:          services.FormatDatePtr(oldestNav),
		LatestNavDate:          services.FormatDatePtr(latestNav),
	})
}

func (h *AdminHandler) GetFunds(c *gin.Context) {
	ctx := c.Request.Context()
	q := strings.TrimSpace(c.Query("q"))
	navStatus := strings.ToLower(c.DefaultQuery("navStatus", "all"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	query := h.db.WithContext(ctx).Model(&models.Fund{}).Preload("NavSyncTracker")

	if q != "" {
		if code, err := strconv.Atoi(q); err == nil {
			query = query.Where("scheme_code = ? OR scheme_name ILIKE ? OR fund_house ILIKE ?", code, "%"+q+"%", "%"+q+"%")
		} else {
			query = query.Where("scheme_name ILIKE ? OR fund_house ILIKE ?", "%"+q+"%", "%"+q+"%")
		}
	}

	if navStatus == "with-nav" {
		query = query.Joins("INNER JOIN \"NavSyncTrackers\" ON \"NavSyncTrackers\".scheme_code = \"Funds\".scheme_code AND \"NavSyncTrackers\".nav_history_count > 0")
	} else if navStatus == "without-nav" {
		query = query.Where("scheme_code NOT IN (SELECT scheme_code FROM \"NavSyncTrackers\" WHERE nav_history_count > 0)")
	}

	var totalItems int64
	_ = query.Count(&totalItems).Error

	var funds []models.Fund
	offset := (page - 1) * pageSize
	_ = query.Order("scheme_name asc").Offset(offset).Limit(pageSize).Find(&funds).Error

	var items []models.AdminFundStorageDto
	for _, f := range funds {
		item := models.AdminFundStorageDto{
			SchemeCode:     f.SchemeCode,
			SchemeName:     f.SchemeName,
			FundHouse:      f.FundHouse,
			SchemeType:     f.SchemeType,
			SchemeCategory: f.SchemeCategory,
			NavStatus:      "Active",
		}
		if f.NavSyncTracker != nil {
			t := f.NavSyncTracker
			item.LatestNav = t.LatestNav
			item.LatestNavDate = services.FormatDatePtr(t.LatestNavDate)
			item.NavStartDate = services.FormatDatePtr(t.NavStartDate)
			item.NavEndDate = services.FormatDatePtr(t.NavEndDate)
			item.FirstStoredNavDate = services.FormatDatePtr(t.NavStartDate)
			item.LastStoredNavDate = services.FormatDatePtr(t.NavEndDate)
			item.NavHistoryCount = t.NavHistoryCount
			item.CorporateActionCount = t.CorporateActionCount
			if t.LastLatestNavCheckAtIst != nil {
				istStr := t.LastLatestNavCheckAtIst.Format(time.RFC3339)
				item.LastSuccessfulNavCheckUtc = &istStr
			}
		}
		items = append(items, item)
	}

	c.JSON(http.StatusOK, models.PagedResponseDto[models.AdminFundStorageDto]{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: int(totalItems),
		TotalPages: int(math.Ceil(float64(totalItems) / float64(pageSize))),
		Items:      items,
	})
}

func (h *AdminHandler) SyncFundMaster(c *gin.Context) {
	result, err := h.fundService.SyncFundMasterList(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) SyncFundNavHistory(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	res, err := h.navService.GetNavHistory(c.Request.Context(), schemeCode, nil, nil, "daily", false)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) DeleteFundNavHistory(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.NavHistory{}).Error
	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.NavSyncTracker{}).Error
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) DeleteFund(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.NavHistory{}).Error
	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.CorporateAction{}).Error
	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.NavSyncTracker{}).Error
	_ = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", schemeCode).Delete(&models.Fund{}).Error
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) DeleteAllFunds(c *gin.Context) {
	_ = h.db.WithContext(c.Request.Context()).Exec("TRUNCATE TABLE \"NavHistories\", \"CorporateActions\", \"NavSyncTrackers\", \"Funds\" CASCADE").Error
	c.Status(http.StatusNoContent)
}
