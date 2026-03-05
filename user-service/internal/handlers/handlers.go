package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/food-platform/user-service/internal/models"
	"github.com/food-platform/user-service/internal/service"
)

// ────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────

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
	default:
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal error"})
	}
}

// ────────────────────────────────────────────────────────────
// Profile handlers
// ────────────────────────────────────────────────────────────

type ProfileHandler struct {
	svc service.ProfileService
}

func NewProfileHandler(svc service.ProfileService) *ProfileHandler {
	return &ProfileHandler{svc: svc}
}

// POST /internal/users/ensure — called by auth-service after registration (no JWT)
func (h *ProfileHandler) EnsureProfile(c *gin.Context) {
	var req struct {
		AuthID uint   `json:"auth_id" binding:"required"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	p, err := h.svc.EnsureProfile(req.AuthID, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "could not ensure profile"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// GET /api/v1/users/:id
func (h *ProfileHandler) GetProfile(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	p, err := h.svc.GetProfile(id, callerID, callerRole)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// PUT /api/v1/users/:id
func (h *ProfileHandler) UpdateProfile(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	p, err := h.svc.UpdateProfile(id, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// DELETE /api/v1/users/:id
func (h *ProfileHandler) DeleteProfile(c *gin.Context) {
	id, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	if err := h.svc.DeleteProfile(id, callerID, callerRole); err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, models.MessageResponse{Message: "account deleted"})
}

// ────────────────────────────────────────────────────────────
// Address handlers
// ────────────────────────────────────────────────────────────

type AddressHandler struct {
	svc service.AddressService
}

func NewAddressHandler(svc service.AddressService) *AddressHandler {
	return &AddressHandler{svc: svc}
}

// GET /api/v1/users/:id/addresses
func (h *AddressHandler) ListAddresses(c *gin.Context) {
	userID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	addrs, err := h.svc.ListAddresses(userID, callerID, callerRole)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, addrs)
}

// POST /api/v1/users/:id/addresses
func (h *AddressHandler) AddAddress(c *gin.Context) {
	userID, err := paramUint(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}
	var req models.AddAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	addr, err := h.svc.AddAddress(userID, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusCreated, addr)
}

// PUT /api/v1/users/addresses/:addressId
func (h *AddressHandler) UpdateAddress(c *gin.Context) {
	addressID, err := paramUint(c, "addressId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid address id"})
		return
	}
	var req models.UpdateAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	callerID, callerRole := callerInfo(c)
	addr, err := h.svc.UpdateAddress(addressID, callerID, callerRole, &req)
	if err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, addr)
}

// DELETE /api/v1/users/addresses/:addressId
func (h *AddressHandler) DeleteAddress(c *gin.Context) {
	addressID, err := paramUint(c, "addressId")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid address id"})
		return
	}
	callerID, callerRole := callerInfo(c)
	if err := h.svc.DeleteAddress(addressID, callerID, callerRole); err != nil {
		handleSvcErr(c, err)
		return
	}
	c.JSON(http.StatusOK, models.MessageResponse{Message: "address deleted"})
}