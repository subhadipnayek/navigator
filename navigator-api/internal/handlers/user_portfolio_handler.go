package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"navigator-api/internal/middleware"
	"navigator-api/internal/models"
	"navigator-api/internal/services"
	"navigator-api/internal/services/importer"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserPortfolioHandler struct {
	portService *services.UserPortfolioService
	importerSvc *importer.ImporterService
	db          *gorm.DB
}

func NewUserPortfolioHandler(portSvc *services.UserPortfolioService, db *gorm.DB) *UserPortfolioHandler {
	return &UserPortfolioHandler{
		portService: portSvc,
		importerSvc: importer.NewImporterService(db),
		db:          db,
	}
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
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
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
	transactions, summary, err := h.portService.GetTransactions(c.Request.Context(), userId, portId, fy)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
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

// ImportPreview handles dry-run parsing across all supported broker formats.
func (h *UserPortfolioHandler) ImportPreview(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Upload a valid file (.xlsx, .xls, or .csv)."})
		return
	}
	defer file.Close()

	res, err := h.importerSvc.Preview(c.Request.Context(), userId, portId, file, header.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

// ImportCommit atomically commits the previewed batch of transactions with the selected strategy.
func (h *UserPortfolioHandler) ImportCommit(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	idStr := c.Param("portfolioId")
	portId, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid portfolio ID"})
		return
	}

	var req importer.ImportCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	insertedCount, err := h.importerSvc.Commit(c.Request.Context(), userId, portId, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// Fetch updated transactions & summary
	fy := c.Query("financialYear")
	transactions, summary, _ := h.portService.GetTransactions(c.Request.Context(), userId, portId, fy)

	c.JSON(http.StatusOK, gin.H{
		"insertedCount": insertedCount,
		"strategy":      req.Strategy,
		"transactions":  transactions,
		"summary":       summary,
	})
}

// Legacy fallback endpoint
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

func (h *UserPortfolioHandler) MatchFunds(c *gin.Context) {
	var req models.MatchPortfolioFundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var matches []models.MatchedPortfolioFundDto
	for _, row := range req.Rows {
		var fund models.Fund
		var err error

		if row.Isin != nil && *row.Isin != "" {
			err = h.db.WithContext(c.Request.Context()).
				Where("isin_growth = ? OR isin_div_reinvestment = ?", *row.Isin, *row.Isin).
				First(&fund).Error
		}

		if err != nil && row.SchemeCode != nil && *row.SchemeCode != "" {
			if code, parseErr := strconv.Atoi(*row.SchemeCode); parseErr == nil {
				err = h.db.WithContext(c.Request.Context()).Where("scheme_code = ?", code).First(&fund).Error
			}
		}

		if err != nil && row.SchemeName != nil && *row.SchemeName != "" {
			err = h.db.WithContext(c.Request.Context()).Where("LOWER(scheme_name) LIKE LOWER(?)", "%"+*row.SchemeName+"%").First(&fund).Error
		}

		if err == nil {
			matches = append(matches, models.MatchedPortfolioFundDto{
				RowNumber:  row.RowNumber,
				SchemeCode: &fund.SchemeCode,
				SchemeName: &fund.SchemeName,
			})
		} else {
			errMsg := "Fund not found"
			matches = append(matches, models.MatchedPortfolioFundDto{
				RowNumber: row.RowNumber,
				Error:     &errMsg,
			})
		}
	}

	c.JSON(http.StatusOK, models.MatchPortfolioFundResponse{Matches: matches})
}

func (h *UserPortfolioHandler) GetGridPreference(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	gridKey := c.Param("gridKey")

	var pref models.UserGridPreference
	err := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND grid_key = ?", userId, gridKey).
		First(&pref).Error

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Preference not found"})
		return
	}

	c.JSON(http.StatusOK, models.UserGridPreferenceDto{
		GridKey:   pref.GridKey,
		StateJson: &pref.StateJson,
	})
}

func (h *UserPortfolioHandler) SaveGridPreference(c *gin.Context) {
	userId := middleware.GetCurrentUserID(c)
	gridKey := c.Param("gridKey")

	var req models.SaveUserGridPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	var pref models.UserGridPreference
	err := h.db.WithContext(c.Request.Context()).
		Where("user_id = ? AND grid_key = ?", userId, gridKey).
		First(&pref).Error

	if err != nil {
		pref = models.UserGridPreference{
			UserId:    userId,
			GridKey:   gridKey,
			StateJson: req.StateJson,
		}
		_ = h.db.WithContext(c.Request.Context()).Create(&pref).Error
	} else {
		pref.StateJson = req.StateJson
		_ = h.db.WithContext(c.Request.Context()).Save(&pref).Error
	}

	c.JSON(http.StatusOK, models.UserGridPreferenceDto{
		GridKey:   pref.GridKey,
		StateJson: &pref.StateJson,
	})
}
