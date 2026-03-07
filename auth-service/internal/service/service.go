package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/food-platform/auth-service/internal/config"
	"github.com/food-platform/auth-service/internal/events"
	"github.com/food-platform/auth-service/internal/models"
	"github.com/food-platform/auth-service/internal/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// ─── Errors ───────────────────────────────────────────────────────────────────

var (
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrPhoneAlreadyExists = errors.New("user with this phone already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrInvalidOTP         = errors.New("invalid or expired OTP")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidRole        = errors.New("invalid role")
	ErrAccountDeleted     = errors.New("account has been deleted")
)

// ─── Claims ───────────────────────────────────────────────────────────────────

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"` // USER | RESTAURANT_OWNER | DRIVER | ADMIN
	jwt.RegisteredClaims
}

// ─── Service interface ────────────────────────────────────────────────────────

type AuthService interface {
	Register(req *models.RegisterRequest) (*models.AuthResponse, error)
	Login(req *models.LoginRequest) (*models.AuthResponse, error)
	RefreshToken(refreshToken string) (*models.AuthResponse, error)
	Logout(userID uint, refreshToken string) error
	SendOTP(phone string) error
	VerifyOTP(phone, code string) (*models.AuthResponse, error)
	DeleteAccount(userID uint) error
	BanUser(userID uint) error
}

type authService struct {
	repo      repository.AuthRepository
	redis     *redis.Client
	cfg       *config.Config
	publisher events.Publisher
}

func New(repo repository.AuthRepository, rdb *redis.Client, cfg *config.Config, publisher events.Publisher) AuthService {
	return &authService{repo: repo, redis: rdb, cfg: cfg, publisher: publisher}
}

// ─── Register ─────────────────────────────────────────────────────────────────

func (s *authService) Register(req *models.RegisterRequest) (*models.AuthResponse, error) {
	// Check email uniqueness.
	existing, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// Check phone uniqueness.
	existingPhone, err := s.repo.FindUserByPhone(req.Phone)
	if err != nil {
		return nil, err
	}
	if existingPhone != nil {
		return nil, ErrPhoneAlreadyExists
	}

	// Hash password with bcrypt (cost=12 is a reasonable default).
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, err
	}

	role := req.Role
	if role == "" {
		role = "USER"
	}

	allowedRoles := map[string]bool{
		"USER": true, "RESTAURANT_OWNER": true, "DRIVER": true,
	}
	if !allowedRoles[role] {
		return nil, ErrInvalidRole
	}

	user := &models.User{
		Email:    req.Email,
		Password: string(hashed),
		Phone:    req.Phone,
		Role:     role,
	}
	if err := s.repo.CreateUser(user); err != nil {
		return nil, err
	}

	// Publish USER_CREATED event
	event := &models.UserCreatedEvent{
		Event:     "USER_CREATED",
		UserID:    user.ID,
		Name:      req.Name,
		Email:     user.Email,
		Phone:     user.Phone,
		Role:      user.Role,
		Timestamp: time.Now(),
	}
	if err := s.publisher.PublishUserCreated(event); err != nil {
		// Log the error but don't fail the registration
		// In production, you might want to handle this differently
		fmt.Printf("Failed to publish USER_CREATED event: %v\n", err)
	}

	return s.buildAuthResponse(user)
}

// ─── Login ────────────────────────────────────────────────────────────────────

func (s *authService) Login(req *models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.repo.FindUserByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if user.IsDeleted {
		return nil, ErrAccountDeleted
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.buildAuthResponse(user)
}

// ─── Refresh Token ────────────────────────────────────────────────────────────

func (s *authService) RefreshToken(refreshToken string) (*models.AuthResponse, error) {
	claims, err := s.parseToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Validate the refresh token still exists in Redis (not revoked).
	ctx := context.Background()
	storedToken, err := s.redis.Get(ctx, s.refreshKey(claims.UserID)).Result()
	if err != nil || storedToken != refreshToken {
		return nil, ErrInvalidToken
	}

	user, err := s.repo.FindUserByID(claims.UserID)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	if user.IsDeleted {
		return nil, ErrAccountDeleted
	}

	return s.buildAuthResponse(user)
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func (s *authService) Logout(userID uint, refreshToken string) error {
	ctx := context.Background()
	// Delete the refresh token from Redis — this revokes it immediately.
	return s.redis.Del(ctx, s.refreshKey(userID)).Err()
}

// ─── Delete Account ───────────────────────────────────────────────────────────

func (s *authService) DeleteAccount(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	// Soft delete the user
	now := time.Now()
	user.IsDeleted = true
	user.DeletedAt = &now

	if err := s.repo.UpdateUser(user); err != nil {
		return err
	}

	// Publish USER_DELETED event
	event := &models.UserDeletedEvent{
		Event:     "USER_DELETED",
		UserID:    user.ID,
		Email:     user.Email,
		Timestamp: time.Now(),
	}
	if err := s.publisher.PublishUserDeleted(event); err != nil {
		// Log the error but don't fail the deletion
		fmt.Printf("Failed to publish USER_DELETED event: %v\n", err)
	}

	// Revoke the user's refresh tokens
	ctx := context.Background()
	s.redis.Del(ctx, s.refreshKey(userID)) //nolint

	return nil
}

// ─── Ban User ─────────────────────────────────────────────────────────────────

func (s *authService) BanUser(userID uint) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrUserNotFound
	}

	now := time.Now()
	user.IsDeleted = true
	user.DeletedAt = &now

	if err := s.repo.UpdateUser(user); err != nil {
		return err
	}

	// Revoke the user's refresh tokens
	ctx := context.Background()
	s.redis.Del(ctx, s.refreshKey(userID)) //nolint

	return nil
}

// ─── OTP ──────────────────────────────────────────────────────────────────────

func (s *authService) SendOTP(phone string) error {
	code := fmt.Sprintf("%06d", rand.Intn(1000000))
	otp := &models.OTP{
		Phone:     phone,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := s.repo.CreateOTP(otp); err != nil {
		return err
	}
	// TODO: integrate with SMS provider (Twilio / AWS SNS).
	// For now, log the OTP in development only.
	fmt.Printf("[DEV ONLY] OTP for %s: %s\n", phone, code)
	return nil
}

func (s *authService) VerifyOTP(phone, code string) (*models.AuthResponse, error) {
	otp, err := s.repo.FindValidOTP(phone, code)
	if err != nil {
		return nil, err
	}
	if otp == nil {
		return nil, ErrInvalidOTP
	}

	if err := s.repo.MarkOTPUsed(otp.ID); err != nil {
		return nil, err
	}

	// Find or auto-create a user for this phone.
	user, err := s.repo.FindUserByPhone(phone)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if user.IsDeleted {
		return nil, ErrAccountDeleted
	}

	// Mark the user as verified and persist.
	user.IsVerified = true
	if err := s.repo.UpdateUser(user); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(user)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// buildAuthResponse generates a fresh access + refresh token pair and persists
// the refresh token to Redis.
func (s *authService) buildAuthResponse(user *models.User) (*models.AuthResponse, error) {
	accessToken, err := s.generateToken(user, s.cfg.AccessTokenTTL)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.generateToken(user, s.cfg.RefreshTokenTTL)
	if err != nil {
		return nil, err
	}

	// Persist refresh token in Redis. Key: refresh:<userID>
	ctx := context.Background()
	if err := s.redis.Set(ctx, s.refreshKey(user.ID), refreshToken, s.cfg.RefreshTokenTTL).Err(); err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: models.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Phone: user.Phone,
			Role:  user.Role,
		},
	}, nil
}

func (s *authService) generateToken(user *models.User, ttl time.Duration) (string, error) {
	claims := &Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "food-platform/auth-service",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *authService) parseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

func (s *authService) refreshKey(userID uint) string {
	return "refresh:" + strconv.Itoa(int(userID))
}
