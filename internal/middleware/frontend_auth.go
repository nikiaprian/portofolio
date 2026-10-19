package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
)

func getRequestToken(c *gin.Context) string {
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return strings.TrimSpace(parts[1])
		}
	}

	if cookieToken, err := c.Cookie("bebii_token"); err == nil && strings.TrimSpace(cookieToken) != "" {
		return strings.TrimSpace(cookieToken)
	}

	return ""
}

func adminLoginRedirectTarget(db *gorm.DB) string {
	return entity.EffectiveLoginHref(repositories.SiteSettingOrDefault(db).LoginURL)
}

// AdminPageAuthMiddleware ensures the admin HTML routes are only accessible by authenticated users.
func AdminPageAuthMiddleware(mgr *JWTManager, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginURL := adminLoginRedirectTarget(db)
		token := getRequestToken(c)
		if token == "" {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}

		userID, err := mgr.Verify(token)
		if err != nil || userID == 0 {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

// AdminPageRoleMiddleware ensures the admin HTML routes are only accessible by users with allowed roles.
func AdminPageRoleMiddleware(db *gorm.DB, roles ...string) gin.HandlerFunc {
	allowed := map[string]struct{}{}
	for _, r := range roles {
		v := strings.ToLower(strings.TrimSpace(r))
		if v != "" {
			allowed[v] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		loginURL := adminLoginRedirectTarget(db)
		userIDAny, ok := c.Get("userID")
		if !ok {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}

		userID, ok := userIDAny.(uint)
		if !ok || userID == 0 {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}

		var u entity.User
		if err := db.Select("id, role, enabled").First(&u, userID).Error; err != nil {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}
		if !u.Enabled {
			c.Redirect(http.StatusFound, loginURL)
			c.Abort()
			return
		}

		role := strings.ToLower(strings.TrimSpace(u.Role))
		if _, allowedRole := allowed[role]; !allowedRole {
			c.Redirect(http.StatusFound, "/admin/dashboard")
			c.Abort()
			return
		}

		c.Set("userRole", role)
		c.Next()
	}
}
