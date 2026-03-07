package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/food-platform/order-service/internal/models"
	"github.com/food-platform/order-service/internal/service"
	"github.com/gin-gonic/gin"
)

// OrderHandler bundles service dependency

type OrderHandler struct {
	svc service.OrderService
}

func NewOrderHandler(svc service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

// RegisterRoutes wires handlers onto a router group.
func (h *OrderHandler) RegisterRoutes(rg *gin.RouterGroup, jwtSecret string) {
	rg.POST("/orders", h.PlaceOrder)
	rg.GET("/orders/:id", h.GetOrder)
	rg.GET("/orders/user/:userId", h.ListByUser)
	rg.GET("/orders/restaurant/:restaurantId", h.ListByRestaurant)
	rg.PATCH("/orders/:id/cancel", h.CancelOrder)
	rg.PATCH("/orders/:id/status", h.UpdateStatusByOwner)
}

// RegisterInternalRoutes wires internal-only endpoints (no JWT expected).
func (h *OrderHandler) RegisterInternalRoutes(rg *gin.RouterGroup) {
	rg.PATCH("/internal/orders/:id/status", h.UpdateStatus)
	rg.GET("/internal/orders/stats", h.GetStats)
}

func (h *OrderHandler) PlaceOrder(c *gin.Context) {
	userIDIfc, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	userID := userIDIfc.(uint)

	key := c.GetHeader("Idempotency-Key")
	if key == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "idempotency_key_required"})
		return
	}

	var req models.PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "validation_failed", Message: err.Error()})
		return
	}

	order, err := h.svc.PlaceOrder(userID, key, &req)
	if err != nil {
		switch err {
		case service.ErrIdempotencyExists:
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "duplicate_request"})
		case service.ErrRestaurantUnavail, service.ErrMenuItemUnavail:
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		case service.ErrCircuitOpen:
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "service_unavailable"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error", Message: err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"order": order})
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	order, err := h.svc.GetOrder(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	if order == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "order_not_found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order})
}

func (h *OrderHandler) ListByUser(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("userId"))
	orders, err := h.svc.ListByUser(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (h *OrderHandler) ListByRestaurant(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("restaurantId"))
	orders, err := h.svc.ListByRestaurant(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"orders": orders})
}

func (h *OrderHandler) CancelOrder(c *gin.Context) {
	userIDIfc, _ := c.Get("user_id")
	userID := userIDIfc.(uint)
	id, _ := strconv.Atoi(c.Param("id"))
	if err := h.svc.CancelOrder(userID, uint(id)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "cancelled"})
}

// UpdateStatusByOwner handles PATCH /orders/:id/status (JWT-protected, restaurant owner only)
func (h *OrderHandler) UpdateStatusByOwner(c *gin.Context) {
	ownerID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.OwnerUpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "validation_failed", Message: err.Error()})
		return
	}
	err := h.svc.UpdateStatusByOwner(ownerID.(uint), uint(id), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "order_not_found"})
		case errors.Is(err, service.ErrForbidden):
			c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "not_restaurant_owner"})
		case errors.Is(err, service.ErrInvalidStatus):
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: "invalid_status_transition"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}
	c.JSON(http.StatusOK, models.SuccessResponse{Message: "status_updated"})
}

// UpdateStatus handles PATCH /internal/orders/:id/status (no JWT, internal/delivery system)
func (h *OrderHandler) UpdateStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req models.InternalUpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "validation_failed", Message: err.Error()})
		return
	}
	err := h.svc.UpdateStatus(uint(id), req.Status)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "order_not_found"})
		case errors.Is(err, service.ErrInvalidStatus):
			c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{Error: "invalid_status_transition"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}
	// Return the updated order so callers can read user_id, restaurant_id, etc.
	order, err := h.svc.GetOrder(uint(id))
	if err != nil || order == nil {
		c.JSON(http.StatusOK, models.SuccessResponse{Message: "status_updated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "status_updated", "order": order})
}

func (h *OrderHandler) GetStats(c *gin.Context) {
	stats, err := h.svc.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "stats_query_failed"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *OrderHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{Service: "order-service", Status: "ok"})
}
