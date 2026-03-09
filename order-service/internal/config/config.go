package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds all environment variables used by order-service.
type Config struct {
	AppEnv                string
	Port                  string
	DatabaseURL           string
	RedisURL              string
	RabbitMQURL           string
	JWTSecret             string
	RestaurantSvcURL      string
	IdempotencyExpiry     string
	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string
}

// Load reads env vars (and .env file) and returns a Config.
func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		Port:                  getEnv("PORT", "8004"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5436/order_db?sslmode=disable"),
		RedisURL:              getEnv("REDIS_URL", "redis://localhost:6379"),
		RabbitMQURL:           getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		JWTSecret:             getEnv("JWT_SECRET", "your-secret-key"),
		RestaurantSvcURL:      getEnv("RESTAURANT_SERVICE_URL", "http://localhost:8003"),
		IdempotencyExpiry:     getEnv("IDEMPOTENCY_EXPIRY", "24h"),
		RazorpayKeyID:         getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:     getEnv("RAZORPAY_KEY_SECRET", ""),
		RazorpayWebhookSecret: getEnv("RAZORPAY_WEBHOOK_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ConnectDB establishes a GORM connection to Postgres.
func ConnectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("order-service: failed to connect to DB: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	fmt.Println("order-service: DB connected")
	return db
}

// ConnectRedis returns a go-redis client. Ping is performed briefly.
func ConnectRedis(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("order-service: Redis unavailable (%v) — rate-limit degraded", err)
	}
	return rdb
}
