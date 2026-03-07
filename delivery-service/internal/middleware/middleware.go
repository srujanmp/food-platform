package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/food-platform/delivery-service/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// JWTAuth validates Bearer token and injects user_id and role into context.
func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "missing_or_invalid_authorization_header"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_or_expired_token"})
			return
		}
		claims, ok := token.Claims.(*jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid_token_claims"})
			return
		}
		if v, exists := (*claims)["user_id"]; exists {
			c.Set("user_id", uint(v.(float64)))
		}
		if v, exists := (*claims)["role"]; exists {
			c.Set("role", v.(string))
		}
		c.Next()
	}
}

// RequireRole allows only specified roles through.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
			return
		}
		for _, r := range roles {
			if role.(string) == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{Error: "insufficient_permissions"})
	}
}

// RequestLogger logs request metadata.
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()
		userID, _ := c.Get("user_id")
		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.ClientIP()),
			zap.Any("user_id", userID),
		)
	}
}

// RateLimit sliding window per-IP using Redis.
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s:%s", c.FullPath(), ip)
		ctx := context.Background()
		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window)
		if _, err := pipe.Exec(ctx); err != nil {
			c.Next()
			return
		}
		count := incr.Val()
		if count > int64(maxRequests) {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, models.ErrorResponse{
				Error:   "rate_limit_exceeded",
				Message: fmt.Sprintf("Maximum %d requests per %s exceeded. Try again later.", maxRequests, window),
			})
			return
		}
		remaining := int64(maxRequests) - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Next()
	}
}
