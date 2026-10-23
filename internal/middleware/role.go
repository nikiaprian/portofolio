package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/internal/entity"
)

// RoleMiddleware ensures authenticated users have one of allowed roles.
func RoleMiddleware(db *gorm.DB, roles ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, r := range roles {
		v := strings.ToLower(strings.TrimSpace(r))
		if v != "" {
			allowed[v] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		userIDAny, ok := c.Get("userID")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		userID, ok := userIDAny.(uint)
		if !ok || userID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		var u entity.User
		if err := db.Select("id, role, enabled").First(&u, userID).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		if !u.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "account disabled"})
			c.Abort()
			return
		}
		role := strings.ToLower(strings.TrimSpace(u.Role))
		if _, ok := allowed[role]; !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			c.Abort()
			return
		}
		c.Set("userRole", role)
		c.Next()
	}
}
