package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/food-platform/auth-service/internal/middleware"
	"github.com/food-platform/auth-service/internal/models"
	"github.com/food-platform/auth-service/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthHandler holds dependencies for all auth routes.
type AuthHandler struct {
	svc       service.AuthService
	startTime time.Time
	jwtSecret string
}

func New(svc service.AuthService, jwtSecret string) *AuthHandler {
	return &AuthHandler{svc: svc, startTime: time.Now(), jwtSecret: jwtSecret}
}

// RegisterRoutes wires all auth endpoints onto the given router group.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, jwtSecret string) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
	rg.POST("/logout", middleware.JWTAuth(jwtSecret), h.Logout)           // JWT applied here
	rg.DELETE("/account", middleware.JWTAuth(jwtSecret), h.DeleteAccount) // JWT applied here
	rg.POST("/otp/send", h.SendOTP)
	rg.POST("/otp/verify", h.VerifyOTP)
	rg.GET("/health", h.Health)
}

// ─── POST /api/v1/auth/register ───────────────────────────────────────────────

func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.svc.Register(&req)
	if err != nil {
		switch err {
		case service.ErrUserAlreadyExists, service.ErrPhoneAlreadyExists:
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: err.Error()})
		case service.ErrInvalidRole:
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ─── POST /api/v1/auth/login ──────────────────────────────────────────────────

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.svc.Login(&req)
	if err != nil {
		switch err {
		case service.ErrInvalidCredentials:
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_credentials"})
		case service.ErrAccountDeleted:
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "account_deleted"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/auth/refresh ────────────────────────────────────────────────

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.svc.RefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_or_expired_token"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/auth/logout ─────────────────────────────────────────────────

func (h *AuthHandler) Logout(c *gin.Context) {
	// user_id is injected into context by the JWT middleware.
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req models.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	if err := h.svc.Logout(userID.(uint), req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "logged out successfully"})
}

// ─── DELETE /api/v1/auth/account ──────────────────────────────────────────────

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	// user_id is injected into context by the JWT middleware.
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	if err := h.svc.DeleteAccount(userID.(uint)); err != nil {
		switch err {
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user_not_found"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "account deleted successfully"})
}

// ─── POST /api/v1/auth/otp/send ───────────────────────────────────────────────

func (h *AuthHandler) SendOTP(c *gin.Context) {
	var req models.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	if err := h.svc.SendOTP(req.Phone); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed_to_send_otp"})
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "OTP sent successfully"})
}

// ─── POST /api/v1/auth/otp/verify ─────────────────────────────────────────────

func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req models.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	resp, err := h.svc.VerifyOTP(req.Phone, req.Code)
	if err != nil {
		switch err {
		case service.ErrInvalidOTP:
			c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_or_expired_otp"})
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user_not_found"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ─── POST /api/v1/internal/ban/:userId ─────────────────────────────────────────
// Internal endpoint — no JWT, Docker-network only.
// Soft-deletes the user and revokes all refresh tokens.

func (h *AuthHandler) BanUser(c *gin.Context) {
	userID := c.Param("userId")
	id, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid user id"})
		return
	}

	if err := h.svc.BanUser(uint(id)); err != nil {
		switch err {
		case service.ErrUserNotFound:
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "user_not_found"})
		default:
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal_server_error"})
		}
		return
	}

	c.JSON(http.StatusOK, models.SuccessResponse{Message: "user banned successfully"})
}

// ─── GET /api/v1/auth/health ──────────────────────────────────────────────────

func (h *AuthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, models.HealthResponse{
		Service: "auth-service",
		Status:  "ok",
		Uptime:  time.Since(h.startTime).String(),
	})
}
