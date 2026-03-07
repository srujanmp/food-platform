package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	AppEnv               string
	Port                 string
	JWTSecret            string
	RedisURL             string
	UserServiceURL       string
	AuthServiceURL       string
	RestaurantServiceURL string
	OrderServiceURL      string
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		Port:                 getEnv("PORT", "8007"),
		JWTSecret:            getEnv("JWT_SECRET", "your-secret-key"),
		RedisURL:             getEnv("REDIS_URL", "localhost:6379"),
		UserServiceURL:       getEnv("USER_SERVICE_URL", "http://localhost:8002"),
		AuthServiceURL:       getEnv("AUTH_SERVICE_URL", "http://localhost:8001"),
		RestaurantServiceURL: getEnv("RESTAURANT_SERVICE_URL", "http://localhost:8003"),
		OrderServiceURL:      getEnv("ORDER_SERVICE_URL", "http://localhost:8004"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func ConnectRedis(addr string) *redis.Client {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("admin-service: Redis unavailable (%v) - rate-limit degraded", err)
	} else {
		fmt.Println("admin-service: Redis connected")
	}
	return rdb
}
