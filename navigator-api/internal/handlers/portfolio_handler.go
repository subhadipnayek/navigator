package handlers

import (
	"net/http"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
)

type PortfolioHandler struct {
	simService *services.PortfolioSimulationService
}

func NewPortfolioHandler(simSvc *services.PortfolioSimulationService) *PortfolioHandler {
	return &PortfolioHandler{simService: simSvc}
}

func (h *PortfolioHandler) Simulate(c *gin.Context) {
	var req models.SimulationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	if req.SipAmount < 0 || req.LumpSumAmount < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Amounts cannot be negative"})
		return
	}
	if req.SipAmount == 0 && req.LumpSumAmount == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "At least one of SIP amount or lump sum must be greater than zero."})
		return
	}

	res, err := h.simService.Simulate(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *PortfolioHandler) Compare(c *gin.Context) {
	var req models.ComparisonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	if len(req.Entries) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "At least 2 entries are required for comparison."})
		return
	}

	res, err := h.simService.Compare(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *PortfolioHandler) WhatIf(c *gin.Context) {
	var req models.WhatIfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	if len(req.Scenarios) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "At least 2 scenarios are required."})
		return
	}

	res, err := h.simService.WhatIf(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *PortfolioHandler) RollingComparison(c *gin.Context) {
	var req models.RollingComparisonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	if len(req.Entries) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "At least 2 entries are required."})
		return
	}

	res, err := h.simService.RollingComparison(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
