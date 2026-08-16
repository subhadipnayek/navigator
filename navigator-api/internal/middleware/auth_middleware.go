package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"navigator-api/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ContextUserID   = "user_id"
	ContextUsername = "username"
	ContextEmail    = "email"
	ContextRole     = "role"
)

func AuthRequired(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authorization header is required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Authorization format must be Bearer <token>"})
			return
		}

		tokenStr := parts[1]
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(cfg.JwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Invalid token claims"})
			return
		}

		subStr, _ := claims["sub"].(string)
		userId, _ := strconv.Atoi(subStr)
		username, _ := claims["name"].(string)
		email, _ := claims["email"].(string)
		role, _ := claims["role"].(string)

		c.Set(ContextUserID, userId)
		c.Set(ContextUsername, username)
		c.Set(ContextEmail, email)
		c.Set(ContextRole, role)

		c.Next()
	}
}

func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(ContextRole)
		if !exists || !strings.EqualFold(userRole.(string), role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "Access forbidden: insufficient permissions"})
			return
		}
		c.Next()
	}
}

func GetCurrentUserID(c *gin.Context) int {
	if val, exists := c.Get(ContextUserID); exists {
		if id, ok := val.(int); ok {
			return id
		}
	}
	return 0
}
