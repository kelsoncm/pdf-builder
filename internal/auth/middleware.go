package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const usernameKey = "username"

// Middleware returns a Gin middleware that validates Bearer tokens.
func Middleware(tokenMap map[string]string, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("missing authorization header",
				zap.String("path", c.Request.URL.Path),
				zap.String("remote_addr", c.ClientIP()),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		const bearerPrefix = "Bearer "
		if !strings.HasPrefix(authHeader, bearerPrefix) {
			logger.Warn("invalid authorization scheme",
				zap.String("path", c.Request.URL.Path),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
		username, ok := tokenMap[token]
		if !ok {
			logger.Warn("invalid token",
				zap.String("path", c.Request.URL.Path),
			)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set(usernameKey, username)
		c.Next()
	}
}

// Username retrieves the authenticated username from the Gin context.
func Username(c *gin.Context) string {
	v, _ := c.Get(usernameKey)
	s, _ := v.(string)
	return s
}
