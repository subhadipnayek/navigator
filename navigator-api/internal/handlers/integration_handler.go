package handlers

import (
	"net/http"

	"navigator-api/internal/models"
	"navigator-api/internal/services"

	"github.com/gin-gonic/gin"
)

type IntegrationHandler struct {
	integrationService *services.IntegrationService
}

func NewIntegrationHandler(intSvc *services.IntegrationService) *IntegrationHandler {
	return &IntegrationHandler{integrationService: intSvc}
}

func (h *IntegrationHandler) GetEndpoints(c *gin.Context) {
	summary, err := h.integrationService.GetEndpoints(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *IntegrationHandler) UpdateEndpoint(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Endpoint key is required"})
		return
	}

	var req models.UpdateApiEndpointConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request body: " + err.Error()})
		return
	}

	updated, err := h.integrationService.UpdateEndpoint(c.Request.Context(), key, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updated)
}

func (h *IntegrationHandler) ResetEndpoint(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Endpoint key is required"})
		return
	}

	res, err := h.integrationService.ResetEndpoint(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *IntegrationHandler) TestEndpoint(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Endpoint key is required"})
		return
	}

	res, err := h.integrationService.TestEndpoint(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *IntegrationHandler) HealthCheckAll(c *gin.Context) {
	summary, err := h.integrationService.HealthCheckAll(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
