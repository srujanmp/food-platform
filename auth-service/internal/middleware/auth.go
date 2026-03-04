package middleware

import (
	"net/http"
	"strings"

	"github.com/food-platform/auth-service/internal/models"
	"github.com/food-platform/auth-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth validates the Bearer token and injects user_id + role into the context.
// Any downstream handler can read: c.Get("user_id"), c.Get("role")
func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "missing_or_invalid_authorization_header",
			})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		token, err := jwt.ParseWithClaims(tokenStr, &service.Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "invalid_or_expired_token",
			})
			return
		}

		claims, ok := token.Claims.(*service.Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{
				Error: "invalid_token_claims",
			})
			return
		}

		// Inject into context — handlers read these via c.Get("user_id"), c.Get("role")
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RequireRole allows a route to be accessed only by specific roles.
// Usage: router.PATCH("/admin/...", middleware.RequireRole("ADMIN"))
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
		c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{
			Error: "insufficient_permissions",
		})
	}
}