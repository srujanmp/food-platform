package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/food-platform/delivery-service/internal/config"
	"github.com/food-platform/delivery-service/internal/events"
	"github.com/food-platform/delivery-service/internal/handlers"
	"github.com/food-platform/delivery-service/internal/middleware"
	"github.com/food-platform/delivery-service/internal/models"
	"github.com/food-platform/delivery-service/internal/repository"
	"github.com/food-platform/delivery-service/internal/service"
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

	// ── Infrastructure ────────────────────────────────────────────────────────
	db := config.ConnectDB(cfg.DatabaseURL)
	rdb := config.ConnectRedis(cfg.RedisURL)

	rbConn, err := amqp091.Dial(cfg.RabbitMQURL)
	if err != nil {
		logger.Fatal("failed to connect to RabbitMQ", zap.Error(err))
	}

	// Channel for publishing
	pubCh, err := rbConn.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ publish channel", zap.Error(err))
	}

	// Channel for consuming (separate channel recommended)
	consCh, err := rbConn.Channel()
	if err != nil {
		logger.Fatal("failed to open RabbitMQ consume channel", zap.Error(err))
	}

	publisher, err := events.NewPublisher(pubCh)
	if err != nil {
		logger.Fatal("failed to create publisher", zap.Error(err))
	}

	// Auto-migrate
	if err := db.AutoMigrate(&models.Driver{}, &models.Delivery{}, &models.OutboxEvent{}, &models.PendingAssignment{}); err != nil {
		logger.Fatal("automigrate failed", zap.Error(err))
	}

	// ── Wire layers ───────────────────────────────────────────────────────────
	driverRepo := repository.NewDriverRepository(db)
	deliveryRepo := repository.NewDeliveryRepository(db)
	outboxRepo := repository.NewOutboxRepository(db)
	pendingRepo := repository.NewPendingAssignmentRepository(db)

	deliverySvc := service.NewDeliveryService(driverRepo, deliveryRepo, outboxRepo, pendingRepo, db, cfg.OrderServiceURL)
	deliveryH := handlers.NewDeliveryHandler(deliverySvc)

	// ── RabbitMQ Consumers ────────────────────────────────────────────────────
	consumer, err := events.NewConsumer(consCh, deliverySvc, logger)
	if err != nil {
		logger.Fatal("failed to create consumer", zap.Error(err))
	}
	consumer.Start()

	// ── Gin setup ─────────────────────────────────────────────────────────────
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

	v1 := r.Group("/api/v1")
	deliveryH.RegisterRoutes(v1, cfg.JWTSecret)

	// ── Outbox relay goroutine ────────────────────────────────────────────────
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			evts, err := outboxRepo.ListUnpublished(10)
			if err != nil {
				logger.Error("outbox query failed", zap.Error(err))
				continue
			}
			for _, ev := range evts {
				if err := publisher.Publish(&ev); err != nil {
					logger.Error("failed to publish event", zap.Error(err))
					continue
				}
				if err := outboxRepo.MarkPublished(ev.ID); err != nil {
					logger.Error("failed to mark published", zap.Error(err))
				}
			}
		}
	}()

	// ── Pending assignment retry poller ───────────────────────────────────────
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			if err := deliverySvc.RetryPendingAssignments(); err != nil {
				logger.Error("pending assignment retry failed", zap.Error(err))
			}
		}
	}()

	// ── HTTP server with graceful shutdown ────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("delivery-service starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down delivery-service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("forced shutdown", zap.Error(err))
	}

	sqlDB, _ := db.DB()
	sqlDB.Close()
	rdb.Close()
	publisher.Close()
	consCh.Close()
	rbConn.Close()

	logger.Info("delivery-service stopped")
}
