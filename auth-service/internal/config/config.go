package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config holds all environment-driven configuration for the service.
type Config struct {
	AppEnv  string
	AppPort string

	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DatabaseURL string

	RedisHost     string
	RedisPort     string
	RedisPassword string

	RabbitMQURL string

	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	RateLimitAuthRPM   int
	RateLimitPublicRPM int

	AdminEmail    string
	AdminPassword string
}

// Load reads environment variables (from .env in dev, from real env in prod).
func Load() *Config {
	// In production (APP_ENV=production) the .env file won't exist — that's fine.
	// Real env vars are injected by Docker / Kubernetes.
	if err := godotenv.Load(); err != nil {
		log.Println("[config] no .env file found — reading from environment")
	}

	accessTTL, _ := strconv.Atoi(getEnv("ACCESS_TOKEN_TTL_MINUTES", "15"))
	refreshTTL, _ := strconv.Atoi(getEnv("REFRESH_TOKEN_TTL_DAYS", "7"))
	authRPM, _ := strconv.Atoi(getEnv("RATE_LIMIT_AUTH_RPM", "30"))
	pubRPM, _ := strconv.Atoi(getEnv("RATE_LIMIT_PUBLIC_RPM", "100"))

	return &Config{
		AppEnv:  getEnv("APP_ENV", "development"),
		AppPort: getEnv("APP_PORT", "8001"),

		DBHost:      getEnv("DB_HOST", "localhost"),
		DBPort:      getEnv("DB_PORT", "5432"),
		DBUser:      getEnv("DB_USER", "postgres"),
		DBPassword:  getEnv("DB_PASSWORD", "postgres"),
		DBName:      getEnv("DB_NAME", "auth_db"),
		DatabaseURL: getEnv("DATABASE_URL", ""),

		RedisHost:     getEnv("REDIS_HOST", "localhost"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),

		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),

		JWTSecret:       getEnv("JWT_SECRET", "changeme"),
		AccessTokenTTL:  time.Duration(accessTTL) * time.Minute,
		RefreshTokenTTL: time.Duration(refreshTTL) * 24 * time.Hour,

		RateLimitAuthRPM:   authRPM,
		RateLimitPublicRPM: pubRPM,

		AdminEmail:    getEnv("ADMIN_EMAIL", "admin@admin.com"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),
	}
}

// ConnectDB opens a PostgreSQL connection via GORM.
func ConnectDB(cfg *Config) *gorm.DB {
	dsn := cfg.DatabaseURL
	if dsn == "" {
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName,
		)
	}

	logLevel := logger.Silent
	if cfg.AppEnv == "development" {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		log.Fatalf("[config] failed to connect to database: %v", err)
	}

	// Connection pool settings — required for production.
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	log.Println("[config] database connected")
	return db
}

// ConnectRedis opens a Redis client.
func ConnectRedis(cfg *Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})
	log.Println("[config] redis connected")
	return rdb
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
