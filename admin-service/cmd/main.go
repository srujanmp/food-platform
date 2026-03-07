package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/food-platform/admin-service/internal/clients"
	"github.com/food-platform/admin-service/internal/config"
	"github.com/food-platform/admin-service/internal/handlers"
	"github.com/food-platform/admin-service/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

	// ── 3. Redis (for rate limiting only) ─────────────────────────────────────
	rdb := config.ConnectRedis(cfg.RedisURL)

	// ── 4. HTTP clients ───────────────────────────────────────────────────────
	userClient := clients.NewUserClient(cfg.UserServiceURL)
	authClient := clients.NewAuthClient(cfg.AuthServiceURL)
	restaurantClient := clients.NewRestaurantClient(cfg.RestaurantServiceURL)
	orderClient := clients.NewOrderClient(cfg.OrderServiceURL)

	// ── 5. Handler ────────────────────────────────────────────────────────────
	handler := handlers.NewAdminHandler(userClient, authClient, restaurantClient, orderClient)

	// ── 6. Gin setup ──────────────────────────────────────────────────────────
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(logger))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(middleware.RateLimit(rdb, 100, time.Minute))

	// ── 7. Routes ─────────────────────────────────────────────────────────────
	v1 := r.Group("/api/v1")
	{
		// Public
		v1.GET("/admin/health", handler.Health)

		// Protected — ADMIN only
		admin := v1.Group("/admin",
			middleware.JWTAuth(cfg.JWTSecret),
			middleware.RequireRole("ADMIN"),
		)
		handler.RegisterRoutes(admin)
	}

	// ── 8. HTTP server with graceful shutdown ─────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("admin-service starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down admin-service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}
	rdb.Close()
	logger.Info("admin-service stopped")
}
