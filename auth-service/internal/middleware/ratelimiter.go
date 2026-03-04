package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/food-platform/auth-service/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimit returns a sliding-window rate limiter middleware.
//   - maxRequests: allowed requests per window
//   - window:      time window duration (e.g. time.Minute)
//
// Key is per-IP address. Failed requests still count against the limit.
func RateLimit(rdb *redis.Client, maxRequests int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s:%s", c.FullPath(), ip)
		ctx := context.Background()

		// Increment counter. On first request, also set the TTL.
		pipe := rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, window)
		if _, err := pipe.Exec(ctx); err != nil {
			// If Redis is down, fail open (allow the request) to avoid cascading failure.
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

		// Expose headers so clients can see their remaining budget.
		remaining := int64(maxRequests) - count
		if remaining < 0 {
			remaining = 0
		}
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		c.Next()
	}
}