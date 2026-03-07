package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/food-platform/restaurant-service/internal/models"
	"github.com/food-platform/restaurant-service/internal/service"
	"github.com/gin-gonic/gin"
)

// ── helpers ────────────────────────────────────────────────────

func callerInfo(c *gin.Context) (uint, string) {
	uid, _ := c.Get("user_id")
	role, _ := c.Get("role")
	return uid.(uint), role.(string)
}

func paramUint(c *gin.Context, key string) (uint, error) {
	v, err := strconv.ParseUint(c.Param(key), 10, 64)
	return uint(v), err
}

func handleSvcErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: err.Error()})
	case errors.Is(err, service.ErrBadStatus):
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
	}
}

// ── Restaurant handlers ────────────────────────────────────────

type RestaurantHandler struct {
	svc service.RestaurantService
}

func NewRestaurantHandler(svc service.RestaurantService) *RestaurantHandler {
	return &RestaurantHandler{svc: svc}
}

// POST /api/v1/restaurants  [Bearer+Owner]
func (h *RestaurantHandler) Create(c *gin.Context) {
	var req models.CreateRestaurantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ownerID, _ := callerInfo(c)
	r, err := h.svc.Create(ownerID, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"restaurant": r})
}

// GET /api/v1/restaurants  [None] — list all approved, open
func (h *RestaurantHandler) List(c *gin.Context) {
	restaurants, err := h.svc.ListApprovedOpen()
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurants": restaurants})
}

// GET /api/v1/restaurants/:id  [None] — details + full menu
func (h *RestaurantHandler) Get(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	r, err := h.svc.GetByIDWithMenu(id)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurant": r})
}

// GET /api/v1/restaurants/search?q=  [None]
func (h *RestaurantHandler) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "query parameter q is required"})
		return
	}
	restaurants, err := h.svc.Search(q)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurants": restaurants})
}

// GET /api/v1/restaurants/nearby?lat=&lng=&radius=  [None]
func (h *RestaurantHandler) Nearby(c *gin.Context) {
	lat, err1 := strconv.ParseFloat(c.Query("lat"), 64)
	lng, err2 := strconv.ParseFloat(c.Query("lng"), 64)
	radius, err3 := strconv.ParseFloat(c.Query("radius"), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "lat, lng, and radius are required"})
		return
	}
	restaurants, err := h.svc.Nearby(lat, lng, radius)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurants": restaurants})
}

// PUT /api/v1/restaurants/:id  [Bearer+Owner]
func (h *RestaurantHandler) Update(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.UpdateRestaurantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	r, err := h.svc.Update(id, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurant": r})
}

// DELETE /api/v1/restaurants/:id  [Bearer+Owner]
func (h *RestaurantHandler) Delete(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	if err := h.svc.Delete(id, callerID, callerRole); err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, models.MessageResponse{Message: "restaurant deactivated"})
}

// PATCH /api/v1/restaurants/:id/status  [Bearer+Owner] — toggle open/closed
func (h *RestaurantHandler) ToggleStatus(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	r, err := h.svc.ToggleStatus(id, callerID, callerRole)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurant": r})
}

// PATCH /api/v1/restaurants/:id/order-status  [Bearer+Owner]
func (h *RestaurantHandler) UpdateOrderStatus(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.OrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	if err := h.svc.UpdateOrderStatus(restaurantID, callerID, callerRole, &req); err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, models.MessageResponse{Message: "order status updated to " + req.Status})
}

// PATCH /api/v1/restaurants/:id/approve  [ADMIN]
func (h *RestaurantHandler) Approve(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	r, err := h.svc.Approve(id)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"restaurant": r})
}

// ── Menu item handlers ─────────────────────────────────────────

type MenuItemHandler struct {
	svc service.MenuItemService
}

func NewMenuItemHandler(svc service.MenuItemService) *MenuItemHandler {
	return &MenuItemHandler{svc: svc}
}

// GET /api/v1/restaurants/:id/menu  [None]
func (h *MenuItemHandler) List(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid restaurant id"})
		return
	}
	items, err := h.svc.ListByRestaurant(restaurantID)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"menu_items": items})
}

// POST /api/v1/restaurants/:id/menu  [Bearer+Owner]
func (h *MenuItemHandler) Create(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid restaurant id"})
		return
	}
	var req models.CreateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	item, err := h.svc.Create(restaurantID, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"menu_item": item})
}

// PUT /api/v1/restaurants/:id/menu/:itemId  [Bearer+Owner]
func (h *MenuItemHandler) Update(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid restaurant id"})
		return
	}
	itemID, err := paramUint(c, "itemId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid item id"})
		return
	}
	var req models.UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	item, err := h.svc.Update(restaurantID, itemID, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"menu_item": item})
}

// DELETE /api/v1/restaurants/:id/menu/:itemId  [Bearer+Owner]
func (h *MenuItemHandler) Delete(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid restaurant id"})
		return
	}
	itemID, err := paramUint(c, "itemId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid item id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	if err := h.svc.Delete(restaurantID, itemID, callerID, callerRole); err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, models.MessageResponse{Message: "menu item removed"})
}

// PATCH /api/v1/restaurants/:id/menu/:itemId/toggle  [Bearer+Owner]
func (h *MenuItemHandler) ToggleAvailability(c *gin.Context) {
	restaurantID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid restaurant id"})
		return
	}
	itemID, err := paramUint(c, "itemId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid item id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	item, err := h.svc.ToggleAvailability(restaurantID, itemID, callerID, callerRole)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"menu_item": item})
}

// ── Internal handlers (no JWT — Docker-network only) ───────────

type InternalHandler struct {
	restSvc service.RestaurantService
	menuSvc service.MenuItemService
}

func NewInternalHandler(rs service.RestaurantService, ms service.MenuItemService) *InternalHandler {
	return &InternalHandler{restSvc: rs, menuSvc: ms}
}

// GET /api/v1/internal/restaurants/:id  [Internal] — restaurant status
func (h *InternalHandler) GetRestaurant(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	r, err := h.restSvc.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "restaurant not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          r.ID,
		"name":        r.Name,
		"is_open":     r.IsOpen,
		"is_approved": r.IsApproved,
	})
}

// GET /api/v1/internal/restaurants/:id/menu/:itemId  [Internal] — single menu item
func (h *InternalHandler) GetMenuItem(c *gin.Context) {
	itemID, err := paramUint(c, "itemId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid item id"})
		return
	}
	item, err := h.menuSvc.GetByID(itemID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "menu item not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           item.ID,
		"name":         item.Name,
		"price":        item.Price,
		"is_available": item.IsAvailable,
	})
}
