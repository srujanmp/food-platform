package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/food-platform/order-service/internal/config"
	"github.com/food-platform/order-service/internal/events"
	"github.com/food-platform/order-service/internal/handlers"
	"github.com/food-platform/order-service/internal/middleware"
	"github.com/food-platform/order-service/internal/models"
	"github.com/food-platform/order-service/internal/repository"
	"github.com/food-platform/order-service/internal/service"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

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

	// infrastructure
	db := config.ConnectDB(cfg.DatabaseURL)
	rdb := config.ConnectRedis(cfg.RedisURL)

	rbConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("failed to connect to RabbitMQ", zap.Error(err))
	}
	rbCh, err := rbConn.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ channel", zap.Error(err))
	}

	publisher, err := events.NewPublisher(rbCh)
	if err != nil {
		logger.Fatal("failed to create publisher", zap.Error(err))
	}

	// auto-migrate for now
	if err := db.AutoMigrate(&models.Order{}, &models.Payment{}, &models.OutboxEvent{}); err != nil {
		logger.Fatal("automigrate failed", zap.Error(err))
	}

	// repositories
	orderRepo := repository.NewOrderRepository(db)
	paymentRepo := repository.NewPaymentRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)

	// services
	razorpayClient := service.NewRazorpayClient(cfg.RazorpayKeyID, cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret, &http.Client{Timeout: 5 * time.Second})
	orderSvc := service.NewOrderService(orderRepo, paymentRepo, outboxRepo, db, cfg.RestaurantSvcURL, razorpayClient)

	// handlers
	orderH := handlers.NewOrderHandler(orderSvc)

	// router
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(logger))
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Idempotency-Key"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	r.Use(middleware.RateLimit(rdb, 100, time.Minute))

	v1 := r.Group("/api/v1")
	{
		// public health
		v1.GET("/orders/health", orderH.Health)
		// internal endpoint (no JWT)
		orderH.RegisterInternalRoutes(v1)

		// protected routes
		auth := middleware.JWTAuth(cfg.JWTSecret)
		protected := v1.Group("")
		protected.Use(auth)
		orderH.RegisterRoutes(protected, cfg.JWTSecret)
	}

	// start outbox relay goroutine
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			events, err := outboxRepo.ListUnpublished(10)
			if err != nil {
				logger.Error("outbox query failed", zap.Error(err))
				continue
			}
			for _, ev := range events {
				var pubErr error
				if ev.EventType == "ORDER_PLACED" {
					pubErr = publisher.PublishOrderPlaced(&ev)
				} else if ev.EventType == "ORDER_CANCELLED" {
					pubErr = publisher.PublishOrderCancelled(&ev)
				} else {
					continue
				}
				if pubErr != nil {
					logger.Error("failed to publish event", zap.Error(pubErr))
					continue
				}
				if err := outboxRepo.MarkPublished(ev.ID); err != nil {
					logger.Error("failed to mark published", zap.Error(err))
				}
			}
		}
	}()

	// server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}
	go func() {
		logger.Info("order-service starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down order-service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()
	rdb.Close()
	publisher.Close()
	logger.Info("order-service stopped")
}
