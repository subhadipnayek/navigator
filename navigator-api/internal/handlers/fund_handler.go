package handlers

import (
	"net/http"
	"strconv"

	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
)

type FundHandler struct {
	fundService *services.FundService
}

func NewFundHandler(fundSvc *services.FundService) *FundHandler {
	return &FundHandler{fundService: fundSvc}
}

func (h *FundHandler) GetAll(c *gin.Context) {
	funds, err := h.fundService.GetAllFunds(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, funds)
}

func (h *FundHandler) GetBySchemeCode(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	fund, err := h.fundService.GetFund(c.Request.Context(), schemeCode)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Fund not found"})
		return
	}

	c.JSON(http.StatusOK, fund)
}

func (h *FundHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Query parameter 'q' is required."})
		return
	}

	funds, err := h.fundService.SearchFunds(c.Request.Context(), q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, funds)
}

func (h *FundHandler) Sync(c *gin.Context) {
	result, err := h.fundService.SyncFundMasterList(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *FundHandler) GetCount(c *gin.Context) {
	count, err := h.fundService.GetFundCount(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}
