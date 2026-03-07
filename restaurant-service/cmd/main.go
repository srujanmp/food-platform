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

	"github.com/food-platform/restaurant-service/internal/config"
	"github.com/food-platform/restaurant-service/internal/handlers"
	"github.com/food-platform/restaurant-service/internal/middleware"
	"github.com/food-platform/restaurant-service/internal/models"
	"github.com/food-platform/restaurant-service/internal/repository"
	"github.com/food-platform/restaurant-service/internal/service"
)

func main() {
	// ── Logger ────────────────────────────────────────────────
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ────────────────────────────────────────────────
	cfg := config.Load()

	// ── DB ────────────────────────────────────────────────────
	db := config.ConnectDB(cfg.DBUrl)
	if err := db.AutoMigrate(&models.Restaurant{}, &models.MenuItem{}, &models.OutboxEvent{}); err != nil {
		log.Fatal("automigrate failed", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────
	rdb := config.ConnectRedis(cfg.RedisAddr)

	// ── Repos ─────────────────────────────────────────────────
	restaurantRepo := repository.NewRestaurantRepository(db)
	menuItemRepo := repository.NewMenuItemRepository(db)
	outboxRepo := repository.NewOutboxRepository()

	// ── Services ──────────────────────────────────────────────
	restaurantSvc := service.NewRestaurantService(restaurantRepo, outboxRepo, db, cfg.OrderServiceURL)
	menuItemSvc := service.NewMenuItemService(menuItemRepo, restaurantRepo)

	// ── Handlers ──────────────────────────────────────────────
	restaurantH := handlers.NewRestaurantHandler(restaurantSvc)
	menuItemH := handlers.NewMenuItemHandler(menuItemSvc)
	internalH := handlers.NewInternalHandler(restaurantSvc, menuItemSvc)

	// ── Router ────────────────────────────────────────────────
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(log))
	r.Use(cors.Default())
	r.Use(middleware.RateLimit(rdb, 100, time.Minute))

	v1 := r.Group("/api/v1")

	// ── Health (public) ───────────────────────────────────────
	v1.GET("/restaurants/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"service": "restaurant-service", "status": "ok"})
	})

	// ── Public routes (no JWT) ───────────────────────────────
	v1.GET("/restaurants", restaurantH.List)
	v1.GET("/restaurants/search", restaurantH.Search)
	v1.GET("/restaurants/nearby", restaurantH.Nearby)
	v1.GET("/restaurants/:id", restaurantH.Get)
	v1.GET("/restaurants/:id/menu", menuItemH.List)

	// ── Internal routes (no JWT — Docker-network only) ───────
	v1.GET("/internal/restaurants/:id", internalH.GetRestaurant)
	v1.GET("/internal/restaurants/:id/menu/:itemId", internalH.GetMenuItem)

	// ── Auth middleware ──────────────────────────────────────
	auth := middleware.Auth(cfg.JWTSecret)

	// ── Owner routes (RESTAURANT_OWNER + ADMIN) ─────────────
	owner := v1.Group("/restaurants", auth, middleware.RequireRole("RESTAURANT_OWNER", "ADMIN"))
	{
		owner.POST("", restaurantH.Create)
		owner.PUT("/:id", restaurantH.Update)
		owner.DELETE("/:id", restaurantH.Delete)
		owner.PATCH("/:id/status", restaurantH.ToggleStatus)
		owner.PATCH("/:id/order-status", restaurantH.UpdateOrderStatus)

		// Menu management
		owner.POST("/:id/menu", menuItemH.Create)
		owner.PUT("/:id/menu/:itemId", menuItemH.Update)
		owner.DELETE("/:id/menu/:itemId", menuItemH.Delete)
		owner.PATCH("/:id/menu/:itemId/toggle", menuItemH.ToggleAvailability)
	}

	// ── Admin routes ─────────────────────────────────────────
	admin := v1.Group("/restaurants", auth, middleware.RequireRole("ADMIN"))
	{
		admin.PATCH("/:id/approve", restaurantH.Approve)
	}

	// ── Graceful shutdown ────────────────────────────────────
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Info("restaurant-service starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("shutting down restaurant-service...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("forced shutdown", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()
	rdb.Close()
	log.Info("restaurant-service stopped")
}
