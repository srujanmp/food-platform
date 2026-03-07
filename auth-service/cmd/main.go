package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/food-platform/auth-service/internal/config"
	"github.com/food-platform/auth-service/internal/events"
	"github.com/food-platform/auth-service/internal/handlers"
	"github.com/food-platform/auth-service/internal/middleware"
	"github.com/food-platform/auth-service/internal/models"
	"github.com/food-platform/auth-service/internal/repository"
	"github.com/food-platform/auth-service/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	// ── 1. Config ─────────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── 2. Logger ─────────────────────────────────────────────────────────────
	var logger *zap.Logger
	var err error
	if cfg.AppEnv == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}
	defer logger.Sync() //nolint:errcheck

	// ── 3. Infrastructure ─────────────────────────────────────────────────────
	db := config.ConnectDB(cfg)
	rdb := config.ConnectRedis(cfg)

	// RabbitMQ connection and channel for event publishing
	rbConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("failed to connect to RabbitMQ", zap.Error(err))
	}
	rbCh, err := rbConn.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ channel", zap.Error(err))
	}

	// Create event publisher
	publisher, err := events.NewPublisher(rbCh)
	if err != nil {
		logger.Fatal("failed to create event publisher", zap.Error(err))
	}

	// ── 4. Auto-migrate (Phase 1 only — replace with golang-migrate in prod) ──
	if err := db.AutoMigrate(&models.User{}, &models.OTP{}); err != nil {
		logger.Fatal("auto-migrate failed", zap.Error(err))
	}

	// ── 4b. Seed default admin ────────────────────────────────────────────────
	seedAdmin(db, publisher, logger, cfg)

	// ── 5. Wire layers ────────────────────────────────────────────────────────
	repo := repository.New(db)
	svc := service.New(repo, rdb, cfg, publisher)
	handler := handlers.New(svc, cfg.JWTSecret)

	// ── 6. Gin setup ──────────────────────────────────────────────────────────
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery()) // catches panics, returns 500

	// CORS — allow all origins in dev; restrict in prod via env.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	r.Use(middleware.RequestLogger(logger))

	// ── 7. Routes ─────────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")
	{
		auth := v1.Group("/auth")

		// Apply stricter rate limit to auth endpoints (30 req/min).
		auth.Use(middleware.RateLimit(rdb, cfg.RateLimitAuthRPM, time.Minute))

		handler.RegisterRoutes(auth, cfg.JWTSecret)

		// Internal routes — no JWT, Docker-network only
		v1.POST("/internal/ban/:userId", handler.BanUser)
	}

	// ── 8. HTTP server with graceful shutdown ─────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so we can listen for OS signals.
	go func() {
		logger.Info("auth-service starting", zap.String("port", cfg.AppPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// Block until SIGINT or SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down gracefully…")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}

	// Close DB connections.
	sqlDB, _ := db.DB()
	sqlDB.Close()
	rdb.Close()

	// Close RabbitMQ connections.
	rbCh.Close()
	rbConn.Close()

	logger.Info("auth-service stopped")
}

// seedAdmin ensures the admin user exists in the users table on every startup.
func seedAdmin(db *gorm.DB, publisher events.Publisher, logger *zap.Logger, cfg *config.Config) {
	if cfg.AdminPassword == "" {
		logger.Info("seedAdmin: ADMIN_PASSWORD not set, skipping admin seed")
		return
	}

	var user models.User
	err := db.Where("email = ?", cfg.AdminEmail).First(&user).Error
	if err == nil {
		return
	}
	if err != gorm.ErrRecordNotFound {
		logger.Error("seedAdmin: failed to query admin user", zap.Error(err))
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 12)
	if err != nil {
		logger.Error("seedAdmin: failed to hash password", zap.Error(err))
		return
	}

	user = models.User{
		Email:      cfg.AdminEmail,
		Password:   string(hashed),
		Phone:      "+10000000000",
		Role:       "ADMIN",
		IsVerified: true,
	}
	if err := db.Create(&user).Error; err != nil {
		logger.Error("seedAdmin: failed to create admin user", zap.Error(err))
		return
	}

	// Publish USER_CREATED so user-service creates the admin profile.
	event := &models.UserCreatedEvent{
		Event:     "USER_CREATED",
		UserID:    user.ID,
		Name:      "Admin",
		Email:     user.Email,
		Phone:     user.Phone,
		Role:      user.Role,
		Timestamp: time.Now(),
	}
	if err := publisher.PublishUserCreated(event); err != nil {
		logger.Warn("seedAdmin: failed to publish USER_CREATED event", zap.Error(err))
	}

	fmt.Printf("✓ Default admin seeded (id=%d, email=%s)\n", user.ID, cfg.AdminEmail)
}
