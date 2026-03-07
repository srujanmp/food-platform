package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/food-platform/delivery-service/internal/middleware"
	"github.com/food-platform/delivery-service/internal/models"
	"github.com/food-platform/delivery-service/internal/service"
	"github.com/gin-gonic/gin"
)

// DeliveryHandler bundles the service dependency.
type DeliveryHandler struct {
	svc service.DeliveryService
}

func NewDeliveryHandler(svc service.DeliveryService) *DeliveryHandler {
	return &DeliveryHandler{svc: svc}
}

// RegisterRoutes wires handlers onto a router group (JWT-protected).
func (h *DeliveryHandler) RegisterRoutes(rg *gin.RouterGroup, jwtSecret string) {
	delivery := rg.Group("/delivery")

	// Public health check
	delivery.GET("/health", h.Health)

	// Auth-required routes
	auth := middleware.JWTAuth(jwtSecret)

	// Anyone authenticated can track
	delivery.GET("/track/:orderId", auth, h.TrackOrder)

	// Driver-only routes
	driver := delivery.Group("")
	driver.Use(auth, middleware.RequireRole("DRIVER"))
	{
		driver.GET("/driver/by-auth/:authId", h.GetDriverByAuth)
		driver.GET("/driver/:driverId", h.GetDriver)
		driver.GET("/driver/:driverId/orders", h.GetDriverOrders)
		driver.PATCH("/location", h.UpdateLocation)
		driver.PATCH("/:orderId/status", h.UpdateStatus)
	}
}

func (h *DeliveryHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{Service: "delivery-service", Status: "ok"})
}

func (h *DeliveryHandler) GetDriver(c *gin.Context) {
	driverID, err := strconv.Atoi(c.Param("driverId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_driver_id"})
		return
	}

	driver, err := h.svc.GetDriver(uint(driverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	if driver == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "driver_not_found"})
		return
	}

	// Get active order for this driver
	activeOrder, _ := h.svc.GetActiveOrderForDriver(driver.ID)

	c.JSON(http.StatusOK, gin.H{
		"driver":       driver,
		"active_order": activeOrder,
	})
}

func (h *DeliveryHandler) GetDriverByAuth(c *gin.Context) {
	authID, err := strconv.Atoi(c.Param("authId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_auth_id"})
		return
	}

	driver, err := h.svc.GetDriverByAuthID(uint(authID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	if driver == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "driver_not_found"})
		return
	}

	activeOrder, _ := h.svc.GetActiveOrderForDriver(driver.ID)

	c.JSON(http.StatusOK, gin.H{
		"driver":       driver,
		"active_order": activeOrder,
	})
}

func (h *DeliveryHandler) GetDriverOrders(c *gin.Context) {
	driverID, err := strconv.Atoi(c.Param("driverId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_driver_id"})
		return
	}

	orders, err := h.svc.ListDriverOrders(uint(driverID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (h *DeliveryHandler) UpdateLocation(c *gin.Context) {
	authID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req models.UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "validation_failed", Message: err.Error()})
		return
	}

	if err := h.svc.UpdateLocation(authID.(uint), req.Latitude, req.Longitude); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "driver_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "location_updated"})
}

func (h *DeliveryHandler) UpdateStatus(c *gin.Context) {
	authID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	orderID, err := strconv.Atoi(c.Param("orderId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_order_id"})
		return
	}

	var req models.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "validation_failed", Message: err.Error()})
		return
	}

	err = h.svc.UpdateDeliveryStatus(authID.(uint), uint(orderID), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "delivery_not_found"})
		case errors.Is(err, service.ErrForbidden):
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not_assigned_driver"})
		case errors.Is(err, service.ErrInvalidTransition):
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: "invalid_status_transition"})
		case errors.Is(err, service.ErrCircuitOpen):
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "service_unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error", Message: err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "status_updated"})
}

func (h *DeliveryHandler) TrackOrder(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("orderId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid_order_id"})
		return
	}

	tracking, err := h.svc.GetTracking(uint(orderID))
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "delivery_not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	c.JSON(http.StatusOK, tracking)
}
