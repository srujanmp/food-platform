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

type Config struct {
	Port            string
	DBUrl           string
	RedisAddr       string
	JWTSecret       string
	OrderServiceURL string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:            getEnv("PORT", "8003"),
		DBUrl:           getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5434/restaurant_db?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		JWTSecret:       getEnv("JWT_SECRET", "supersecret"),
		OrderServiceURL: getEnv("ORDER_SERVICE_URL", "http://order-service:8004"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConnectDB(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("restaurant-service: failed to connect to DB: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	fmt.Println("restaurant-service: DB connected")
	return db
}

func ConnectRedis(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("restaurant-service: Redis unavailable (%v) — rate-limit degraded", err)
	}
	return rdb
}
