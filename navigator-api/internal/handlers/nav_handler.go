package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
)

type NavHandler struct {
	navService *services.NavService
}

func NewNavHandler(navSvc *services.NavService) *NavHandler {
	return &NavHandler{navService: navSvc}
}

func (h *NavHandler) GetNavHistory(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	var fromPtr, toPtr *time.Time
	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := services.ParseDate(fromStr); err == nil {
			fromPtr = &t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := services.ParseDate(toStr); err == nil {
			toPtr = &t
		}
	}

	interval := c.Query("interval")
	adjustedStr := c.DefaultQuery("adjusted", "true")
	adjusted := !strings.EqualFold(adjustedStr, "false")

	res, err := h.navService.GetNavHistory(c.Request.Context(), schemeCode, fromPtr, toPtr, interval, adjusted)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *NavHandler) GetReturns(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	startStr := c.Query("startDate")
	endStr := c.Query("endDate")
	if startStr == "" || endStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "startDate and endDate are required"})
		return
	}

	start, err := services.ParseDate(startStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid startDate"})
		return
	}

	end, err := services.ParseDate(endStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid endDate"})
		return
	}

	if !end.After(start) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "startDate must be before endDate."})
		return
	}

	res, err := h.navService.GetReturns(c.Request.Context(), schemeCode, start, end)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *NavHandler) GetRollingReturns(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	periodYears, _ := strconv.Atoi(c.DefaultQuery("periodInYears", "1"))
	var fromPtr, toPtr *time.Time
	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := services.ParseDate(fromStr); err == nil {
			fromPtr = &t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := services.ParseDate(toStr); err == nil {
			toPtr = &t
		}
	}
	interval := c.Query("interval")

	res, err := h.navService.GetRollingReturns(c.Request.Context(), schemeCode, periodYears, fromPtr, toPtr, interval)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *NavHandler) DetectGaps(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	minGapDays, _ := strconv.Atoi(c.DefaultQuery("minGapDays", "30"))
	gaps, err := h.navService.DetectNavGaps(c.Request.Context(), schemeCode, minGapDays)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gaps)
}

func (h *NavHandler) DetectAllGaps(c *gin.Context) {
	minGapDays, _ := strconv.Atoi(c.DefaultQuery("minGapDays", "30"))
	gaps, err := h.navService.DetectAllNavGaps(c.Request.Context(), minGapDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gaps)
}

func (h *NavHandler) RepairGaps(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	minGapDays, _ := strconv.Atoi(c.DefaultQuery("minGapDays", "30"))
	gaps, err := h.navService.RepairNavGaps(c.Request.Context(), schemeCode, minGapDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gaps)
}
