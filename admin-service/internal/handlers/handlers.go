package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/food-platform/admin-service/internal/clients"
	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	userClient       *clients.UserClient
	authClient       *clients.AuthClient
	restaurantClient *clients.RestaurantClient
	orderClient      *clients.OrderClient
	startTime        time.Time
}

func NewAdminHandler(uc *clients.UserClient, ac *clients.AuthClient, rc *clients.RestaurantClient, oc *clients.OrderClient) *AdminHandler {
	return &AdminHandler{
		userClient:       uc,
		authClient:       ac,
		restaurantClient: rc,
		orderClient:      oc,
		startTime:        time.Now(),
	}
}

// RegisterRoutes installs admin routes onto the given router group.
// All routes (except health) require ADMIN role via middleware applied in main.go.
func (h *AdminHandler) RegisterRoutes(admin *gin.RouterGroup) {
	admin.GET("/users", h.ListUsers)
	admin.PATCH("/users/:id/ban", h.BanUser)
	admin.GET("/restaurants", h.ListRestaurants)
	admin.PATCH("/restaurants/:id/approve", h.ApproveRestaurant)
	admin.GET("/analytics/dashboard", h.Dashboard)
}

// ─── GET /api/v1/admin/users ──────────────────────────────────────────────────

func (h *AdminHandler) ListUsers(c *gin.Context) {
	body, status, err := h.userClient.ListUsers()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "upstream service unavailable"})
		return
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		c.Data(status, "application/json", body)
		return
	}
	c.Data(status, "application/json", body)
}

// ─── PATCH /api/v1/admin/users/:id/ban ────────────────────────────────────────

func (h *AdminHandler) BanUser(c *gin.Context) {
	userID := c.Param("id")

	// Call auth-service to revoke tokens
	authBody, authStatus, err := h.authClient.BanUser(userID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth-service unavailable"})
		return
	}

	// Call user-service to mark user as banned
	userBody, userStatus, err := h.userClient.BanUser(userID)
	if err != nil {
		// Auth succeeded but user-service failed — still return partial success
		c.Data(authStatus, "application/json", authBody)
		return
	}

	// Prefer user-service response (has the full user info)
	if userStatus >= 200 && userStatus < 300 {
		c.Data(userStatus, "application/json", userBody)
		return
	}
	c.Data(authStatus, "application/json", authBody)
}

// ─── GET /api/v1/admin/restaurants ────────────────────────────────────────────

func (h *AdminHandler) ListRestaurants(c *gin.Context) {
	body, status, err := h.restaurantClient.ListRestaurants()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "upstream service unavailable"})
		return
	}
	c.Data(status, "application/json", body)
}

// ─── PATCH /api/v1/admin/restaurants/:id/approve ──────────────────────────────

func (h *AdminHandler) ApproveRestaurant(c *gin.Context) {
	id := c.Param("id")
	token := c.GetHeader("Authorization")
	if len(token) > 7 {
		token = token[7:] // strip "Bearer "
	}

	body, status, err := h.restaurantClient.ApproveRestaurant(id, token)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "upstream service unavailable"})
		return
	}
	c.Data(status, "application/json", body)
}

// ─── GET /api/v1/admin/analytics/dashboard ────────────────────────────────────

func (h *AdminHandler) Dashboard(c *gin.Context) {
	dashboard := gin.H{
		"total_orders":      0,
		"total_revenue":     0,
		"total_delivered":   0,
		"total_cancelled":   0,
		"total_users":       0,
		"total_restaurants": 0,
	}

	// Users count
	userBody, userStatus, err := h.userClient.ListUsers()
	if err == nil && userStatus == http.StatusOK {
		var users []json.RawMessage
		if json.Unmarshal(userBody, &users) == nil {
			dashboard["total_users"] = len(users)
		}
	}

	// Restaurants count
	restBody, restStatus, err := h.restaurantClient.ListRestaurants()
	if err == nil && restStatus == http.StatusOK {
		var restResp struct {
			Restaurants []json.RawMessage `json:"restaurants"`
		}
		if json.Unmarshal(restBody, &restResp) == nil {
			dashboard["total_restaurants"] = len(restResp.Restaurants)
		}
	}

	// Order stats
	orderBody, orderStatus, err := h.orderClient.GetStats()
	if err == nil && orderStatus == http.StatusOK {
		var stats struct {
			TotalOrders    int64   `json:"total_orders"`
			TotalRevenue   float64 `json:"total_revenue"`
			TotalDelivered int64   `json:"total_delivered"`
			TotalCancelled int64   `json:"total_cancelled"`
		}
		if json.Unmarshal(orderBody, &stats) == nil {
			dashboard["total_orders"] = stats.TotalOrders
			dashboard["total_revenue"] = stats.TotalRevenue
			dashboard["total_delivered"] = stats.TotalDelivered
			dashboard["total_cancelled"] = stats.TotalCancelled
		}
	}

	c.JSON(http.StatusOK, dashboard)
}

// ─── GET /api/v1/admin/health ─────────────────────────────────────────────────

func (h *AdminHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service": "admin-service",
		"status":  "ok",
		"uptime":  time.Since(h.startTime).String(),
	})
}
