package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// function to authenticate the user using JWT
func JWTMiddleware(mgr *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				candidate := strings.TrimSpace(parts[1])
				low := strings.ToLower(candidate)
				if candidate != "" && low != "null" && low != "undefined" {
					token = candidate
				}
			}
		}
		// Fallback for admin browser sessions that rely on cookie auth.
		if token == "" {
			if cookieToken, err := c.Cookie("bebii_token"); err == nil {
				candidate := strings.TrimSpace(cookieToken)
				low := strings.ToLower(candidate)
				if candidate != "" && low != "null" && low != "undefined" {
					token = candidate
				}
			}
		}
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid token"})
			c.Abort()
			return
		}

		userID, err := mgr.Verify(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		// set the user ID in the context
		c.Set("userID", userID)
		c.Next()
	}
}
