package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/food-platform/user-service/internal/config"
	"github.com/food-platform/user-service/internal/handlers"
	"github.com/food-platform/user-service/internal/middleware"
	"github.com/food-platform/user-service/internal/models"
	"github.com/food-platform/user-service/internal/repository"
	"github.com/food-platform/user-service/internal/service"
)

func main() {
	// ── Logger ────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ────────────────────────────────────────────────
	cfg := config.Load()

	// ── DB ────────────────────────────────────────────────────
	db := config.ConnectDB(cfg.DBUrl)
	// AutoMigrate for Phase 1 — replace with golang-migrate for production
	if err := db.AutoMigrate(&models.Profile{}, &models.Address{}); err != nil {
		log.Fatal("automigrate failed", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────
	rdb := config.ConnectRedis(cfg.RedisAddr)

	// ── Repos ─────────────────────────────────────────────────
	profileRepo := repository.NewProfileRepository(db)
	addressRepo := repository.NewAddressRepository(db)

	// ── Services ──────────────────────────────────────────────
	profileSvc := service.NewProfileService(profileRepo)
	addressSvc := service.NewAddressService(addressRepo, profileRepo)

	// ── Handlers ──────────────────────────────────────────────
	profileH := handlers.NewProfileHandler(profileSvc)
	addressH := handlers.NewAddressHandler(addressSvc)

	// ── Router ────────────────────────────────────────────────
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))
	r.Use(cors.Default())
	r.Use(middleware.RateLimit(rdb, 100, time.Minute))

	v1 := r.Group("/api/v1")

	// ── Health (public) ───────────────────────────────────────
	v1.GET("/users/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "user-service", "status": "ok"})
	})

	// ── Internal route — no JWT, Docker-network only ──────────
	// Called by auth-service immediately after registration to
	// create the profile row in user_db.
	v1.POST("/internal/users/ensure", profileH.EnsureProfile)

	// ── Protected routes ──────────────────────────────────────
	auth := middleware.Auth(cfg.JWTSecret)

	users := v1.Group("/users", auth)
	{
		users.GET("/:id", profileH.GetProfile)
		users.PUT("/:id", profileH.UpdateProfile)
		users.DELETE("/:id", profileH.DeleteProfile)

		users.GET("/:id/addresses", addressH.ListAddresses)
		users.POST("/:id/addresses", addressH.AddAddress)
	}

	// Address routes where addressId (not userId) is the param
	v1.PUT("/users/addresses/:addressId", auth, addressH.UpdateAddress)
	v1.DELETE("/users/addresses/:addressId", auth, addressH.DeleteAddress)

	// ── Graceful shutdown ─────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Info("user-service starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down user-service...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()
	rdb.Close()
	log.Info("user-service stopped")
}
