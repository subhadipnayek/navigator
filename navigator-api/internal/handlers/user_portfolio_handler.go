package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/middleware"
	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserPortfolioHandler struct {
	portService *services.UserPortfolioService
	db          *gorm.DB
}

func NewUserPortfolioHandler(portSvc *services.UserPortfolioService, db *gorm.DB) *UserPortfolioHandler {
	return &UserPortfolioHandler{portService: portSvc, db: db}
}

func (h *UserPortfolioHandler) GetPortfolios(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	portfolios, err := h.portService.GetPortfolios(c.Request.Context(), userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, portfolios)
}

func (h *UserPortfolioHandler) CreatePortfolio(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	var req models.CreateUserPortfolioRequest
	_ = c.ShouldBindJSON(&req)

	res, err := h.portService.CreatePortfolio(c.Request.Context(), userId, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *UserPortfolioHandler) GetDashboard(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	showFullyExited := strings.EqualFold(c.Query("showFullyExitedFunds"), "true")
	res, err := h.portService.GetDashboard(c.Request.Context(), userId, portId, showFullyExited)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Portfolio not found"})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *UserPortfolioHandler) GetTransactions(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	fy := c.Query("financialYear")
	txs, summary, err := h.portService.GetTransactions(c.Request.Context(), userId, portId, fy)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Portfolio not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": txs,
		"summary":      summary,
	})
}

func (h *UserPortfolioHandler) SaveTransactions(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	var req models.SavePortfolioTransactionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.portService.SaveTransactions(c.Request.Context(), userId, portId, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if !res.IsValid {
		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *UserPortfolioHandler) ImportTransactions(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Upload a valid Excel file."})
		return
	}
	defer file.Close()

	fy := c.Query("financialYear")
	mode := c.DefaultQuery("importMode", "Append")
	startDate, _ := services.ParseDate(c.Query("startDate"))
	endDate, _ := services.ParseDate(c.Query("endDate"))

	res, err := h.portService.ImportTransactions(c.Request.Context(), userId, portId, fy, mode, startDate, endDate, file, header.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if !res.IsValid {
		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *UserPortfolioHandler) GetImportDeletePreview(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	start, err := services.ParseDate(c.Query("startDate"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid startDate"})
		return
	}
	end, err := services.ParseDate(c.Query("endDate"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid endDate"})
		return
	}

	count, err := h.portService.GetImportDeletePreview(c.Request.Context(), userId, portId, start, end)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, models.ImportDeletePreviewResponse{MatchingTransactionsToDelete: count})
}

func (h *UserPortfolioHandler) CompareBenchmark(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	var req models.PortfolioBenchmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	res, err := h.portService.CompareBenchmark(c.Request.Context(), userId, portId, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *UserPortfolioHandler) GetGridPreference(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	gridKey := c.Param("gridKey")

	var pref models.UserGridPreference
	err := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND grid_key = ?", userId, gridKey).
		First(&pref).Error

	var stateJson *string
	if err == nil {
		stateJson = &pref.StateJson
	}

	c.JSON(http.StatusOK, models.UserGridPreferenceDto{
		GridKey:   gridKey,
		StateJson: stateJson,
	})
}

func (h *UserPortfolioHandler) SaveGridPreference(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	gridKey := c.Param("gridKey")

	var req models.SaveUserGridPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.StateJson) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Grid state cannot be empty."})
		return
	}

	var pref models.UserGridPreference
	err := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND grid_key = ?", userId, gridKey).
		First(&pref).Error

	if err != nil {
		pref = models.UserGridPreference{
			UserId:       userId,
			GridKey:      gridKey,
			StateJson:    req.StateJson,
			CreatedAtUtc: time.Now().UTC(),
			UpdatedAtUtc: time.Now().UTC(),
		}
		_ = h.db.WithContext(c.Request.Context()).Create(&pref).Error
	} else {
		pref.StateJson = req.StateJson
		pref.UpdatedAtUtc = time.Now().UTC()
		_ = h.db.WithContext(c.Request.Context()).Save(&pref).Error
	}

	c.JSON(http.StatusOK, models.UserGridPreferenceDto{
		GridKey:   gridKey,
		StateJson: &pref.StateJson,
	})
}

func (h *UserPortfolioHandler) MatchFunds(c *gin.Context) {
	var req models.MatchPortfolioFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var results []models.MatchedPortfolioFundDto
	for _, row := range req.Rows {
		var isin, schemeCodeText, schemeName string
		if row.Isin != nil {
			isin = strings.TrimSpace(*row.Isin)
		}
		if row.SchemeCode != nil {
			schemeCodeText = strings.TrimSpace(*row.SchemeCode)
		}
		if row.SchemeName != nil {
			schemeName = strings.TrimSpace(*row.SchemeName)
		}

		schemeCode, err := strconv.Atoi(schemeCodeText)
		hasSchemeCode := (err == nil && schemeCode > 0)

		if isin == "" && !hasSchemeCode {
			msg := "SchemeCode and ISIN both missing."
			results = append(results, models.MatchedPortfolioFundDto{
				RowNumber: row.RowNumber,
				Error:     &msg,
			})
			continue
		}

		var candidates []models.Fund
		q := h.db.WithContext(c.Request.Context())
		if hasSchemeCode && isin != "" {
			q = q.Where("scheme_code = ? AND (isin_growth = ? OR isin_div_reinvestment = ?)", schemeCode, isin, isin)
		} else if hasSchemeCode {
			q = q.Where("scheme_code = ?", schemeCode)
		} else {
			q = q.Where("isin_growth = ? OR isin_div_reinvestment = ?", isin, isin)
		}

		_ = q.Limit(5).Find(&candidates).Error
		if len(candidates) == 0 {
			msg := "SchemeCode and ISIN not found in database."
			results = append(results, models.MatchedPortfolioFundDto{
				RowNumber: row.RowNumber,
				Error:     &msg,
			})
			continue
		}

		match := candidates[0]
		if len(candidates) > 1 && schemeName != "" {
			for _, f := range candidates {
				if strings.EqualFold(f.SchemeName, schemeName) {
					match = f
					break
				}
			}
		}

		results = append(results, models.MatchedPortfolioFundDto{
			RowNumber:  row.RowNumber,
			SchemeCode: &match.SchemeCode,
			SchemeName: &match.SchemeName,
		})
	}

	c.JSON(http.StatusOK, models.MatchPortfolioFundResponse{Matches: results})
}
