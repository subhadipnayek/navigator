package handlers

import (
	"net/http"
	"strconv"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
)

type CorporateActionHandler struct {
	caService *services.CorporateActionService
}

func NewCorporateActionHandler(caSvc *services.CorporateActionService) *CorporateActionHandler {
	return &CorporateActionHandler{caService: caSvc}
}

func (h *CorporateActionHandler) DetectAnomalies(c *gin.Context) {
	threshold, _ := strconv.ParseFloat(c.DefaultQuery("threshold", "30"), 64)
	minDays, _ := strconv.Atoi(c.DefaultQuery("minDaysSincePrevious", "1"))

	var schemeCodePtr *int
	if scStr := c.Query("schemeCode"); scStr != "" {
		if sc, err := strconv.Atoi(scStr); err == nil {
			schemeCodePtr = &sc
		}
	}

	res, err := h.caService.DetectAnomalies(c.Request.Context(), threshold, minDays, schemeCodePtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CorporateActionHandler) GetReviewQueue(c *gin.Context) {
	threshold, _ := strconv.ParseFloat(c.DefaultQuery("threshold", "30"), 64)
	minGapDays, _ := strconv.Atoi(c.DefaultQuery("minGapDays", "1"))
	filter := c.DefaultQuery("filter", "all")
	priority := c.DefaultQuery("priority", "all")
	includeRecorded := c.DefaultQuery("includeRecorded", "true") == "true"
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "500"))

	var schemeCodePtr *int
	if scStr := c.Query("schemeCode"); scStr != "" {
		if sc, err := strconv.Atoi(scStr); err == nil {
			schemeCodePtr = &sc
		}
	}

	var qPtr, actionTypePtr *string
	if q := c.Query("q"); q != "" {
		qPtr = &q
	}
	if at := c.Query("actionType"); at != "" {
		actionTypePtr = &at
	}

	var minConfPtr *float64
	if mcStr := c.Query("minConfidence"); mcStr != "" {
		if mc, err := strconv.ParseFloat(mcStr, 64); err == nil {
			minConfPtr = &mc
		}
	}

	res, err := h.caService.GetReviewQueue(c.Request.Context(), threshold, minGapDays, filter, priority, includeRecorded, schemeCodePtr, qPtr, actionTypePtr, minConfPtr, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CorporateActionHandler) AutoRecordAnomalies(c *gin.Context) {
	minConfidence, _ := strconv.ParseFloat(c.DefaultQuery("minAutoRecordConfidence", "0.90"), 64)
	res, err := h.caService.AutoRecordAnomalies(c.Request.Context(), minConfidence)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CorporateActionHandler) GetActions(c *gin.Context) {
	codeStr := c.Param("schemeCode")
	schemeCode, err := strconv.Atoi(codeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid scheme code"})
		return
	}

	actions, err := h.caService.GetActions(c.Request.Context(), schemeCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, actions)
}

func (h *CorporateActionHandler) CreateAction(c *gin.Context) {
	var req models.CreateCorporateActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	res, err := h.caService.CreateAction(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CorporateActionHandler) UpdateAction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid action ID"})
		return
	}

	var req models.UpdateCorporateActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	res, err := h.caService.UpdateAction(c.Request.Context(), id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *CorporateActionHandler) DeleteAction(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid action ID"})
		return
	}

	if err := h.caService.DeleteAction(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
