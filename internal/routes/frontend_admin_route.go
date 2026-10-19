package routes

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/internal/entity"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/admin"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/post"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
	"github.com/Bebii-Digital/bebii-blog/internal/middleware"
)

// adminAssetQueryVersion cache-busts admin static URLs from file mtimes so deploys
// pick up new JS/CSS without relying on process restart (see staticCacheHeaders immutable policy).
func adminAssetQueryVersion() string {
	paths := []string{
		"public/static/css/adminlte.min.css",
		"public/static/css/admin/crud.min.css",
		"public/static/js/adminlte.min.js",
		"public/static/js/api.min.js",
		"public/static/js/admin/crud.min.js",
		"public/static/js/admin/category.min.js",
		"public/static/js/admin/tag.min.js",
		"public/static/js/admin/post.min.js",
		"public/static/js/admin/page.min.js",
		"public/static/js/admin/page_form.min.js",
		"public/static/js/admin/user.min.js",
		"public/static/js/admin/profile.min.js",
		"public/static/js/admin/bulk_post.min.js",
		"public/static/js/admin/post_form.min.js",
		"public/static/js/admin/ai_setting.min.js",
		"public/static/js/admin/custom_layout.min.js",
		"public/static/js/admin/custom_layout_form.min.js",
		"public/static/js/admin/custom_layout_vars.min.js",
		"public/static/js/admin/custom_layout_ai.min.js",
		"public/static/js/admin/backup.min.js",
		"public/static/js/admin/wordpress_import.min.js",
		"public/static/js/admin/site_setting.js",
		"public/static/js/admin/site_setting.min.js",
		"public/static/js/admin/site_connections.min.js",
		"public/static/js/admin/scraper_setting.min.js",
		"public/static/js/admin/scraper_logs.min.js",
		"public/static/js/admin/login_logs.min.js",
		"public/static/js/admin/bulk_post_logs.min.js",
		"public/static/js/admin/manual_post_ai_logs.min.js",
		"public/static/js/admin/log_timestamp.min.js",
		"public/static/js/admin/tinymce.min.js",
	}
	var max int64
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if t := fi.ModTime().Unix(); t > max {
			max = t
		}
	}
	if max == 0 {
		return strconv.FormatInt(time.Now().UTC().Unix(), 10)
	}
	return strconv.FormatInt(max, 10)
}

func injectAdminAbsoluteBaseURL(c *gin.Context, data gin.H) {
	if c == nil || data == nil {
		return
	}
	base := canonicalBaseURL(c)
	data["public_base"] = base
	data["admin_public_site_origin"] = seo.PublicSiteOrigin(c)
}

// RegisterAdminFrontendRoutes configures SSR admin pages that require login.
func RegisterAdminFrontendRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	redirectHome := func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, notFoundRedirectTarget(db))
	}
	jwtMgr := middleware.NewJWTManager(cfg.JWTSecret, cfg.JWTExpiresInHours)
	admin := r.Group("/admin")
	admin.Use(middleware.AdminPageAuthMiddleware(jwtMgr, db))
	{
		// DASHBOARD
		admin.GET("/dashboard", func(c *gin.Context) {
			tmpl, err := template.ParseFiles(
				"public/templates/layouts/admin/app.html",
				"public/templates/layouts/admin/navbar.html",
				"public/templates/layouts/admin/sidebar.html",
				"public/templates/pages/admin/dashboard.html",
			)
			if err != nil {
				c.String(http.StatusInternalServerError, "template error: %v", err)
				return
			}

			data := adminDashboardPage(db)

			if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
				return
			}
		})

		// PROFILE
		admin.GET("/profile", func(c *gin.Context) {
			tmpl, err := template.ParseFiles(
				"public/templates/layouts/admin/app.html",
				"public/templates/layouts/admin/navbar.html",
				"public/templates/layouts/admin/sidebar.html",
				"public/templates/pages/admin/profile/index.html",
			)
			if err != nil {
				c.String(http.StatusInternalServerError, "template error: %v", err)
				return
			}
			data := adminSitePage(db, "Profile", "profile", nil)
			if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
				c.String(http.StatusInternalServerError, "render error: %v", err)
				return
			}
		})

		adminOnly := admin.Group("")
		adminOnly.Use(middleware.AdminPageRoleMiddleware(db, "admin"))
		{
			registerPortfolioAdminRoutes(adminOnly, db, cfg)
			// LIST ALL USERS
			adminOnly.GET("/users", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/user/create.html",
					"public/templates/pages/admin/user/edit.html",
					"public/templates/pages/admin/user/delete.html",
					"public/templates/pages/admin/user/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "User", "users", adminutil.CrudListView(
					"users",
					"Semua User",
					"Tambah User",
					"bi-person-plus",
					"#modalUserCreate",
					[]gin.H{
						{"label": "User"},
						{"label": "Nama pengguna"},
						{"label": "Peran"},
						{"label": "Status"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// CREATE PAGE
			adminOnly.GET("/pages/create", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/page/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPageFormPage(db, "Tambah Page", "create", 0)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT PAGE
			adminOnly.GET("/pages/:id/edit", func(c *gin.Context) {
				id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
				if err != nil || id64 == 0 {
					redirectHome(c)
					return
				}
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/page/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPageFormPage(db, "Edit Page", "edit", uint(id64))
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL PAGES
			adminOnly.GET("/pages", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/page/delete.html",
					"public/templates/pages/admin/page/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Page", "pages", adminutil.CrudListViewWithCreateLink(
					"pages",
					"Semua Page",
					"Tambah Page",
					"bi-folder-plus",
					"/admin/pages/create",
					[]gin.H{
						{"label": "Page"},
						{"label": "Slug"},
						{"label": "Status"},
						{"label": "Index GSC"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))
				data["admin_pages_subnav"] = "list"

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL CATEGORIES
			adminOnly.GET("/categories", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/category/create.html",
					"public/templates/pages/admin/category/edit.html",
					"public/templates/pages/admin/category/delete.html",
					"public/templates/pages/admin/category/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Category", "categories", adminutil.CrudListView(
					"categories",
					"Semua Category",
					"Tambah Category",
					"bi-folder-plus",
					"#modalCategoryCreate",
					[]gin.H{
						{"label": "Category"},
						{"label": "Urutan"},
						{"label": "Slug"},
						{"label": "Judul"},
						{"label": "Kata kunci"},
						{"label": "Deskripsi"},
						{"label": "Status"},
						{"label": "Index GSC"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT SITE SETTINGS
			adminOnly.GET("/site-settings", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/site_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSiteSettingsPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT AI SETTINGS
			adminOnly.GET("/ai-setting", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/ai_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminAISettingPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// CUSTOM LAYOUT — list (register before /:id)
			adminOnly.GET("/custom-layout", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/custom_layout/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminCustomLayoutListPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// CUSTOM LAYOUT — create or edit (id=new for create)
			adminOnly.GET("/custom-layout/:id", func(c *gin.Context) {
				idParam := strings.TrimSpace(c.Param("id"))
				formMode := "edit"
				var layoutID uint
				title := "Edit custom layout"
				if strings.EqualFold(idParam, "new") {
					formMode = "create"
					title = "Tambah custom layout"
				} else {
					id64, err := strconv.ParseUint(idParam, 10, 64)
					if err != nil || id64 == 0 {
						redirectHome(c)
						return
					}
					layoutID = uint(id64)
				}
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/custom_layout/form.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminCustomLayoutFormPage(db, title, formMode, layoutID)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// RESTORE & BACKUP DATABASE
			adminOnly.GET("/restore-backup", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/backup/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminSitePage(db, "Restore & Backup", "settings", nil)
				data["admin_settings_subnav"] = "backup"
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// IMPORT WORDPRESS
			adminOnly.GET("/wordpress-import", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/wordpress_import/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminSitePage(db, "Import WordPress", "settings", nil)
				data["admin_settings_subnav"] = "wordpress_import"
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// EDIT SCRAPER SETTINGS
			adminOnly.GET("/scraper-settings", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/scraper_setting/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminScraperSettingsPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// SCRAPER LOGS (read-only viewer)
			adminOnly.GET("/scraper-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/scraper_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminScraperLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LOGIN LOGS (read-only audit)
			adminOnly.GET("/login-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/login_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminLoginLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})
		}

		writerOrAdmin := admin.Group("")
		writerOrAdmin.Use(middleware.AdminPageRoleMiddleware(db, "admin", "writer"))
		{
			// LIST ALL TAGS
			writerOrAdmin.GET("/tags", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/tag/create.html",
					"public/templates/pages/admin/tag/edit.html",
					"public/templates/pages/admin/tag/delete.html",
					"public/templates/pages/admin/tag/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Tag", "tags", adminutil.CrudListView(
					"tags",
					"Tag",
					"Tambah tag",
					"bi-tag-fill",
					"#modalTagCreate",
					[]gin.H{
						{"label": "Tag"},
						{"label": "Slug"},
						{"label": "Index GSC"},
						{"label": "Diperbarui"},
						{"label": "Aksi", "class": "text-end"},
					},
				))

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			postListColumns := []gin.H{
				{"label": "Post"},
				{"label": "Status"},
				{"label": "Kategori"},
				{"label": "Penulis"},
				{"label": "Index GSC"},
				{"label": "Diperbarui"},
				{"label": "Aksi", "class": "text-end"},
			}

			// CREATE POST
			writerOrAdmin.GET("/posts/create", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPostFormPage(db, c, "Tambah post", "create", 0)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/bulk-posts", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/bulk_post.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminBulkPostPage(db)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/bulk-post-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/bulk_post_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminBulkPostLogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/manual-post-ai-logs", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/manual_post_ai_logs/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminManualPostAILogsPage(db, cfg)
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			writerOrAdmin.GET("/posts/bulk", func(c *gin.Context) {
				c.Redirect(http.StatusMovedPermanently, "/admin/bulk-posts")
			})

			// EDIT POST
			writerOrAdmin.GET("/posts/:id/edit", func(c *gin.Context) {
				id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
				if err != nil || id64 == 0 {
					redirectHome(c)
					return
				}
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/pages/admin/post/form_page.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}
				data := adminPostFormPage(db, c, "Edit post", "edit", uint(id64))
				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})

			// LIST ALL POSTS
			writerOrAdmin.GET("/posts", func(c *gin.Context) {
				tmpl, err := template.ParseFiles(
					"public/templates/layouts/admin/app.html",
					"public/templates/layouts/admin/navbar.html",
					"public/templates/layouts/admin/sidebar.html",
					"public/templates/partials/admin/crud_list.html",
					"public/templates/pages/admin/post/delete.html",
					"public/templates/pages/admin/post/index.html",
				)
				if err != nil {
					c.String(http.StatusInternalServerError, "template error: %v", err)
					return
				}

				data := adminSitePage(db, "Post", "posts", adminutil.CrudListViewWithCreateLink(
					"posts",
					"Post",
					"Tambah post",
					"bi-file-earmark-plus",
					"/admin/posts/create",
					postListColumns,
				))
				data["admin_posts_subnav"] = "list"

				if err := executeAdminTemplate(c, tmpl, db, data); err != nil {
					c.String(http.StatusInternalServerError, "render error: %v", err)
					return
				}
			})
		}
	}
}

// adminDashboardPage builds data for the admin home dashboard (stats + meta).
func adminDashboardPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Dashboard"

	var nUsers, nPosts, nCategories, nTags, nPublished, nDraft, nScheduled int64
	var nPages, nScraperLogs, nLoginLogs, nBulkPostLogs, nManualPostAILogs int64
	_ = db.Model(&entity.User{}).Count(&nUsers).Error
	_ = db.Model(&entity.Post{}).Count(&nPosts).Error
	_ = db.Model(&entity.Category{}).Count(&nCategories).Error
	_ = db.Model(&entity.Tag{}).Count(&nTags).Error
	_ = db.Model(&entity.Post{}).Scopes(postutil.ScopeVisiblePublished("", time.Now())).Count(&nPublished).Error
	_ = db.Model(&entity.Post{}).Where("status = ?", "draft").Count(&nDraft).Error
	_ = db.Model(&entity.Post{}).Where("status = ?", "scheduled").Count(&nScheduled).Error
	_ = db.Model(&entity.Page{}).Count(&nPages).Error
	_ = db.Model(&entity.ScraperLog{}).Count(&nScraperLogs).Error
	_ = db.Model(&entity.LoginLog{}).Count(&nLoginLogs).Error
	_ = db.Model(&entity.BulkPostLog{}).Count(&nBulkPostLogs).Error
	_ = db.Model(&entity.ManualPostAILog{}).Count(&nManualPostAILogs).Error

	publishedPct, draftPct := 0, 0
	if nPosts > 0 {
		publishedPct = int(nPublished * 100 / nPosts)
		draftPct = int(nDraft * 100 / nPosts)
	}

	return gin.H{
		"title":                        title,
		"admin_nav":                    "dashboard",
		"admin_settings_subnav":        "",
		"admin_pages_subnav":           "",
		"admin_posts_subnav":           "",
		"meta_title":                   title + " - " + siteMeta.DefaultTitle,
		"meta_description":             siteMeta.DefaultDesc,
		"meta_keywords":                siteMeta.DefaultKeywords,
		"meta_image":                   siteMeta.DefaultImage,
		"site_name":                    siteSetting.SiteName,
		"logo_url":                     siteSetting.LogoURL,
		"favicon_url":                  siteSetting.FaviconURL,
		"theme_color":                  themeColorMeta(siteSetting),
		"asset_version":                adminAssetQueryVersion(),
		"stat_users":                   nUsers,
		"stat_users_fmt":               formatIntDecimalGrouping(nUsers),
		"stat_posts":                   nPosts,
		"stat_posts_fmt":               formatIntDecimalGrouping(nPosts),
		"stat_categories":              nCategories,
		"stat_categories_fmt":          formatIntDecimalGrouping(nCategories),
		"stat_tags":                    nTags,
		"stat_tags_fmt":                formatIntDecimalGrouping(nTags),
		"stat_pages":                   nPages,
		"stat_pages_fmt":               formatIntDecimalGrouping(nPages),
		"stat_scraper_logs":            nScraperLogs,
		"stat_scraper_logs_fmt":        formatIntDecimalGrouping(nScraperLogs),
		"stat_login_logs":              nLoginLogs,
		"stat_login_logs_fmt":          formatIntDecimalGrouping(nLoginLogs),
		"stat_bulk_post_logs":          nBulkPostLogs,
		"stat_bulk_post_logs_fmt":      formatIntDecimalGrouping(nBulkPostLogs),
		"stat_manual_post_ai_logs":     nManualPostAILogs,
		"stat_manual_post_ai_logs_fmt": formatIntDecimalGrouping(nManualPostAILogs),
		"stat_published":               nPublished,
		"stat_published_fmt":           formatIntDecimalGrouping(nPublished),
		"stat_drafts":                  nDraft,
		"stat_drafts_fmt":              formatIntDecimalGrouping(nDraft),
		"stat_scheduled":               nScheduled,
		"stat_scheduled_fmt":           formatIntDecimalGrouping(nScheduled),
		"stat_published_pct":           publishedPct,
		"stat_draft_pct":               draftPct,
		"login_href":                   entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
}

// adminSiteSettingsPage builds data for the site settings edit form (singleton).
func adminSiteSettingsPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	siteCloudStorage := loadSiteCloudStorage(db)
	title := "Site settings"

	var sc entity.SiteConnection
	_ = db.First(&sc).Error

	ct := entity.NormalizeColorTheme(strings.TrimSpace(siteSetting.ColorTheme))
	sidebarLimits := sidebarListLimitFields(siteSetting)
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "site",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteMeta.DefaultTitle,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"color_theme_custom":    ct == entity.ColorThemeCustom,
		"setting": gin.H{
			"site_name":                           siteSetting.SiteName,
			"footer_copy":                         siteSetting.FooterCopy,
			"tagline":                             siteSetting.Tagline,
			"logo_url":                            siteSetting.LogoURL,
			"favicon_url":                         siteSetting.FaviconURL,
			"theme_color":                         themeColorMeta(siteSetting),
			"text_primary":                        siteSetting.TextPrimary,
			"text_secondary":                      siteSetting.TextSecondary,
			"background_primary":                  siteSetting.BackgroundPrimary,
			"background_secondary":                siteSetting.BackgroundSecondary,
			"primary_button":                      siteSetting.PrimaryButton,
			"secondary_button":                    siteSetting.SecondaryButton,
			"accent_color":                        siteSetting.AccentColor,
			"border_color":                        siteSetting.BorderColor,
			"font_family":                         siteSetting.FontFamily,
			"content_font_size":                   siteSetting.ContentFontSize,
			"sidebar_font_size":                   siteSetting.SidebarFontSize,
			"sidebar_list_limit":                  sidebarLimits["sidebar_list_limit"],
			"sidebar_list_limit_popular":          sidebarLimits["sidebar_list_limit_popular"],
			"sidebar_list_limit_latest":           sidebarLimits["sidebar_list_limit_latest"],
			"image_caption_enabled":               siteSetting.ImageCaptionEnabled,
			"image_caption_mode":                  siteSetting.ImageCaptionMode,
			"image_caption_template":              siteSetting.ImageCaptionTemplate,
			"image_resize_enabled":                siteSetting.ImageResizeEnabled,
			"img_resize_enabled":                  siteSetting.ImgResizeEnabled,
			"img_resize_width":                    siteSetting.ImgResizeWidth,
			"img_resize_height":                   siteSetting.ImgResizeHeight,
			"img_resize_quality":                  siteSetting.ImgResizeQuality,
			"image_resize_width":                  siteSetting.ImageResizeWidth,
			"image_resize_quality":                siteSetting.ImageResizeQuality,
			"ads_lazy_load_enabled":               siteSetting.AdsLazyLoadEnabled,
			"stored_image_max_width":              siteSetting.StoredImageMaxWidth,
			"stored_image_quality":                siteSetting.StoredImageQuality,
			"avatar_resize_width":                 siteSetting.AvatarResizeWidth,
			"avatar_resize_quality":               siteSetting.AvatarResizeQuality,
			"not_found_redirect_mode":             entity.NormalizeNotFoundRedirectMode(strings.TrimSpace(siteSetting.NotFoundRedirectMode)),
			"default_title":                       siteSetting.DefaultTitle,
			"default_description":                 siteSetting.DefaultDesc,
			"default_image":                       siteSetting.DefaultImage,
			"default_keywords":                    siteSetting.DefaultKeywords,
			"homepage_layout":                     entity.NormalizeHomeLayout(strings.TrimSpace(siteSetting.HomepageLayout)),
			"color_theme":                         ct,
			"header_script":                       siteSetting.HeaderScript,
			"footer_script":                       siteSetting.FooterScript,
			"ads_top":                             siteSetting.AdsTop,
			"ads_center":                          siteSetting.AdsCenter,
			"ads_bottom":                          siteSetting.AdsBottom,
			"article_ads_json":                    siteSetting.ArticleAdsJSON,
			"social_facebook_url":                 siteSetting.SocialFacebookURL,
			"social_x_url":                        siteSetting.SocialXURL,
			"social_instagram_url":                siteSetting.SocialInstagramURL,
			"social_youtube_url":                  siteSetting.SocialYouTubeURL,
			"social_tiktok_url":                   siteSetting.SocialTikTokURL,
			"social_rss_url":                      siteSetting.SocialRSSURL,
			"login_url":                           siteSetting.LoginURL,
			"article_slug_mode":                   entity.NormalizeArticleSlugMode(strings.TrimSpace(siteSetting.ArticleSlugMode)),
			"article_schema_type":                 entity.NormalizeArticleSchemaType(strings.TrimSpace(siteSetting.ArticleSchemaType)),
			"scraper_auto_tag":                    siteSetting.ScraperAutoTag,
			"auto_href_enabled":                   siteSetting.AutoHrefEnabled,
			"also_read_enabled":                   siteSetting.AlsoReadEnabled,
			"also_read_max_inserts":               entity.EffectiveAlsoReadMaxInserts(siteSetting.AlsoReadMaxInserts),
			"also_read_paragraph_interval":        entity.EffectiveAlsoReadParagraphInterval(siteSetting.AlsoReadParagraphInterval),
			"post_date_refresh_enabled":           siteSetting.PostDateRefreshEnabled,
			"post_date_refresh_days":              siteSetting.PostDateRefreshDays,
			"post_date_interval_mins":             siteSetting.PostDateIntervalMins,
			"post_date_url_recycle_enabled":       siteSetting.PostDateUrlRecycleEnabled,
			"post_date_url_recycle_interval_mins": siteSetting.PostDateUrlRecycleIntervalMins,
			"source_disclaimer_mode":              siteSetting.SourceDisclaimerMode,
			"source_disclaimer_template":          siteSetting.SourceDisclaimerTemplate,
		},
		"cloud_storage": gin.H{
			"r2_account_id":        siteCloudStorage.R2AccountID,
			"r2_access_key_id":     siteCloudStorage.R2AccessKeyID,
			"r2_secret_access_key": siteCloudStorage.R2SecretAccessKey,
			"r2_bucket":            siteCloudStorage.R2Bucket,
			"r2_public_base_url":   siteCloudStorage.R2PublicBaseURL,
			"r2_prefix":            siteCloudStorage.R2Prefix,
		},
		"connections": gin.H{
			"ahrefs":                      sc.Ahrefs,
			"google":                      sc.Google,
			"bing":                        sc.Bing,
			"yandex":                      sc.Yandex,
			"auto_post_enabled":           sc.AutoPostEnabled,
			"auto_post_template":          sc.AutoPostTemplate,
			"facebook_enabled":            sc.FacebookEnabled,
			"facebook_client_id":          sc.FacebookClientID,
			"facebook_client_secret":      sc.FacebookClientSecret,
			"facebook_token_url":          sc.FacebookTokenURL,
			"facebook_publish_url":        sc.FacebookPublishURL,
			"facebook_scope":              sc.FacebookScope,
			"facebook_account_id":         sc.FacebookAccountID,
			"x_enabled":                   sc.XEnabled,
			"x_client_id":                 sc.XClientID,
			"x_client_secret":             sc.XClientSecret,
			"x_token_url":                 sc.XTokenURL,
			"x_publish_url":               sc.XPublishURL,
			"x_scope":                     sc.XScope,
			"x_account_id":                sc.XAccountID,
			"tiktok_enabled":              sc.TiktokEnabled,
			"tiktok_client_id":            sc.TiktokClientID,
			"tiktok_client_secret":        sc.TiktokClientSecret,
			"tiktok_token_url":            sc.TiktokTokenURL,
			"tiktok_publish_url":          sc.TiktokPublishURL,
			"tiktok_scope":                sc.TiktokScope,
			"tiktok_account_id":           sc.TiktokAccountID,
			"instagram_enabled":           sc.InstagramEnabled,
			"instagram_client_id":         sc.InstagramClientID,
			"instagram_client_secret":     sc.InstagramClientSecret,
			"instagram_token_url":         sc.InstagramTokenURL,
			"instagram_publish_url":       sc.InstagramPublishURL,
			"instagram_scope":             sc.InstagramScope,
			"instagram_account_id":        sc.InstagramAccountID,
			"google_indexing_enabled":     sc.GoogleIndexingEnabled,
			"google_service_account_json": sc.GoogleServiceAccountJSON,
		},
	}
}

// adminCustomLayoutListPage builds template data for the custom layout list.
func adminCustomLayoutListPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Custom layout"
	crud := adminutil.CrudListViewWithCreateLink(
		"custom-layout",
		"Custom layout",
		"Tambah layout",
		"bi-layout-text-window-reverse",
		"/admin/custom-layout/new",
		[]gin.H{
			{"label": "Layout"},
			{"label": "Status"},
			{"label": "Diperbarui"},
			{"label": "Aksi", "class": "text-end"},
		},
	)
	crud["hide_row_select"] = true
	data := adminSitePage(db, title, "settings", crud)
	data["admin_settings_subnav"] = "custom_layout"
	data["meta_title"] = title + " - " + siteSetting.SiteName
	data["meta_description"] = siteMeta.DefaultDesc
	data["meta_keywords"] = siteMeta.DefaultKeywords
	data["meta_image"] = siteMeta.DefaultImage
	return data
}

// adminCustomLayoutFormPage builds template data for create/edit custom layout.
func adminCustomLayoutFormPage(db *gorm.DB, title, formMode string, layoutID uint) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	data := gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "custom_layout",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"layout_form_mode":      formMode,
		"layout_id":             layoutID,
	}
	return data
}

// adminAISettingPage builds template data for the AI settings admin page.
func adminAISettingPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "AI settings"
	aiSetting := loadAISetting(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "ai",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_setting":            aiSetting,
	}
}

// adminLogRetentionHelp documents the shared rolling window for admin audit tables (env ADMIN_LOG_RETENTION_HOURS).
func adminLogRetentionHelp(cfg config.Config) string {
	h := cfg.AdminLogRetentionHours
	if h <= 0 {
		return "Pembersihan otomatis nonaktif (ADMIN_LOG_RETENTION_HOURS <= 0). Log tetap tersimpan sampai Anda menghapusnya manual."
	}
	if h == 1 {
		return "Baris lebih lama dari 1 jam otomatis dihapus dari log scraper, login, bulk post, dan AI post manual (ADMIN_LOG_RETENTION_HOURS). Pembersihan berjalan sekitar 15 menit sekali."
	}
	return fmt.Sprintf("Baris lebih lama dari %d jam otomatis dihapus dari log scraper, login, bulk post, dan AI post manual (ADMIN_LOG_RETENTION_HOURS). Pembersihan berjalan sekitar 15 menit sekali.", h)
}

func adminLoginLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log login"
	crud := adminutil.CrudListViewReadOnly("login-logs", "Log login", []gin.H{
		{"label": "Waktu"},
		{"label": "Email"},
		{"label": "Hasil"},
		{"label": "Pengguna"},
		{"label": "Alasan"},
		{"label": "IP"},
		{"label": "User agent"},
	})
	crud["card_subtitle"] = "Percobaan masuk (berhasil dan gagal). Waktu memakai zona waktu lokal browser Anda. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "login-logs-refresh"
	crud["clear_logs_btn_id"] = "login-logs-clear"
	crud["success_filter_id"] = "login-logs-success-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "login",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminScraperLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log scraper"
	crud := adminutil.CrudListViewReadOnly("scr-logs", "Log scraper", []gin.H{
		{"label": "Waktu"},
		{"label": "Level"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "URL"},
		{"label": "Kategori"},
	})
	crud["card_subtitle"] = adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "scr-logs-refresh"
	crud["clear_logs_btn_id"] = "scr-logs-clear"
	crud["level_filter_id"] = "scr-logs-level-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "scraper",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminScraperSettingsPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Scraper settings"
	aiOK := aiConfigured(db)
	var s entity.ScraperSetting
	_ = db.Order("id desc").First(&s).Error
	var categories []entity.Category
	_ = db.Order("name asc").Find(&categories).Error
	// s.ScraperAIProvider = siteSetting.ScraperAIProvider
	// s.ScraperPromptTitle = siteSetting.ScraperPromptTitle
	// s.ScraperPromptContent = siteSetting.ScraperPromptContent
	return gin.H{
		"title":                 title,
		"admin_nav":             "settings",
		"admin_settings_subnav": "scraper",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"accent_color":          siteSetting.AccentColor,
		"background_primary":    siteSetting.BackgroundPrimary,
		"background_secondary":  siteSetting.BackgroundSecondary,
		"text_primary":          siteSetting.TextPrimary,
		"text_secondary":        siteSetting.TextSecondary,
		"border_color":          siteSetting.BorderColor,
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":         aiOK,
		"scraper_setting":       s,
		"categories":            categories,
	}
}

// adminPostGeneratePage builds template data for the AI generate-post admin page.
func adminPostGeneratePage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Buat post"
	aiOK := aiConfigured(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             "posts",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "generate",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":         aiOK,
	}
}

// adminPageFormPage builds template data for full-page create/edit static pages.
func adminPageFormPage(db *gorm.DB, title, formMode string, pageID uint) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	subnav := ""
	switch formMode {
	case "create":
		subnav = "create"
	case "edit":
		subnav = "edit"
	}
	return gin.H{
		"title":                 title,
		"admin_nav":             "pages",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    subnav,
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"page_form_mode":        formMode,
		"page_id":               pageID,
	}
}

// adminPostFormPage builds template data for full-page create/edit post forms.
func adminPostFormPage(db *gorm.DB, c *gin.Context, title, formMode string, postID uint) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	aiOK := aiConfigured(db)
	subnav := ""
	switch formMode {
	case "create":
		subnav = "create"
	case "edit":
		subnav = "edit"
	}
	var requesterID uint
	if uid, ok := c.Get("userID"); ok {
		if v, ok := uid.(uint); ok {
			requesterID = v
		}
	}
	requesterRole := ""
	if r, ok := c.Get("userRole"); ok {
		if s, ok := r.(string); ok {
			requesterRole = strings.ToLower(strings.TrimSpace(s))
		}
	}
	var authors []entity.User
	_ = db.Where("role IN ? AND enabled = ?", []string{"admin", "writer"}, true).
		Order("name asc").Find(&authors).Error
	var aiRow entity.AISetting
	_ = db.First(&aiRow).Error
	promptName1 := strings.TrimSpace(aiRow.GeneratePromptContentName)
	if promptName1 == "" {
		promptName1 = "Prompt content 1"
	}
	promptName2 := strings.TrimSpace(aiRow.GeneratePromptContentName2)
	if promptName2 == "" {
		promptName2 = "Prompt content 2"
	}
	promptName3 := strings.TrimSpace(aiRow.GeneratePromptContentName3)
	if promptName3 == "" {
		promptName3 = "Prompt content 3"
	}
	return gin.H{
		"title":                    title,
		"admin_nav":                "posts",
		"admin_settings_subnav":    "",
		"admin_pages_subnav":       "",
		"admin_posts_subnav":       subnav,
		"meta_title":               title + " - " + siteSetting.SiteName,
		"meta_description":         siteMeta.DefaultDesc,
		"meta_keywords":            siteMeta.DefaultKeywords,
		"meta_image":               siteMeta.DefaultImage,
		"site_name":                siteSetting.SiteName,
		"logo_url":                 siteSetting.LogoURL,
		"favicon_url":              siteSetting.FaviconURL,
		"theme_color":              themeColorMeta(siteSetting),
		"asset_version":            adminAssetQueryVersion(),
		"login_href":               entity.EffectiveLoginHref(siteSetting.LoginURL),
		"ai_configured":            aiOK,
		"post_form_mode":           formMode,
		"post_id":                  postID,
		"post_form_requester_id":   requesterID,
		"post_form_requester_role": requesterRole,
		"post_form_authors":        authors,
		"prompt_content_name_1":    promptName1,
		"prompt_content_name_2":    promptName2,
		"prompt_content_name_3":    promptName3,
	}
}

// adminBulkPostPage builds data for AI bulk post generation (template + suggest titles + generate).
func adminBulkPostPage(db *gorm.DB) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Bulk post"
	aiOK := aiConfigured(db)
	var categories []entity.Category
	_ = db.Order("name asc").Find(&categories).Error
	var aiRow entity.AISetting
	_ = db.First(&aiRow).Error
	promptName1 := strings.TrimSpace(aiRow.GeneratePromptContentName)
	if promptName1 == "" {
		promptName1 = "Prompt content 1"
	}
	promptName2 := strings.TrimSpace(aiRow.GeneratePromptContentName2)
	if promptName2 == "" {
		promptName2 = "Prompt content 2"
	}
	promptName3 := strings.TrimSpace(aiRow.GeneratePromptContentName3)
	if promptName3 == "" {
		promptName3 = "Prompt content 3"
	}
	var authors []entity.User
	_ = db.Where("role IN ?", []string{"admin", "writer"}).Order("name asc").Find(&authors).Error
	var tags []entity.Tag
	_ = db.Order("name asc").Find(&tags).Error
	return gin.H{
		"title":                            title,
		"admin_nav":                        "bulk_posts",
		"admin_settings_subnav":            "",
		"admin_pages_subnav":               "",
		"admin_posts_subnav":               "",
		"meta_title":                       title + " - " + siteSetting.SiteName,
		"meta_description":                 siteMeta.DefaultDesc,
		"meta_keywords":                    siteMeta.DefaultKeywords,
		"meta_image":                       siteMeta.DefaultImage,
		"site_name":                        siteSetting.SiteName,
		"logo_url":                         siteSetting.LogoURL,
		"favicon_url":                      siteSetting.FaviconURL,
		"theme_color":                      themeColorMeta(siteSetting),
		"asset_version":                    adminAssetQueryVersion(),
		"ai_configured":                    aiOK,
		"generate_prompt_title":            aiRow.GeneratePromptTitle,
		"prompt_content_name_1":            promptName1,
		"generate_prompt_content":          aiRow.GeneratePromptContent,
		"prompt_content_name_2":            promptName2,
		"prompt_content_name_3":            promptName3,
		"generate_prompt_meta_description": aiRow.GeneratePromptMetaDescription,
		"generate_prompt_meta_keywords":    aiRow.GeneratePromptMetaKeywords,
		"bulk_cover_background_url":        aiRow.BulkCoverBackgroundURL,
		"categories":                       categories,
		"authors":                          authors,
		"tags":                             tags,
		"login_href":                       entity.EffectiveLoginHref(siteSetting.LoginURL),
	}
}

func adminBulkPostLogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log bulk post"
	crud := adminutil.CrudListViewReadOnly("bulk-post-logs", "Log bulk post", []gin.H{
		{"label": "Waktu"},
		{"label": "Pengguna"},
		{"label": "Level"},
		{"label": "Event"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "Kategori"},
		{"label": "Post"},
		{"label": "Judul"},
	})
	crud["card_subtitle"] = "Saran judul AI dan pembuatan batch. Penulis hanya melihat baris miliknya; admin melihat semua. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "bulk-post-logs-refresh"
	crud["clear_logs_btn_id"] = "bulk-post-logs-clear"
	crud["level_filter_id"] = "bulk-post-logs-level-filter"
	crud["event_filter_id"] = "bulk-post-logs-event-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "bulk",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func adminManualPostAILogsPage(db *gorm.DB, cfg config.Config) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	title := "Log AI post manual"
	crud := adminutil.CrudListViewReadOnly("manual-post-ai-logs", "Log AI post manual", []gin.H{
		{"label": "Waktu"},
		{"label": "Pengguna"},
		{"label": "Level"},
		{"label": "Event"},
		{"label": "AI diminta"},
		{"label": "AI dipakai"},
		{"label": "Pesan"},
		{"label": "Post"},
		{"label": "Judul"},
	})
	crud["card_subtitle"] = "Dicatat hanya saat form post manual memicu pembuatan AI (konten/gambar). Simpan/update manual tanpa generate tidak dicatat. Penulis hanya melihat baris miliknya; admin melihat semua. " + adminLogRetentionHelp(cfg)
	crud["refresh_btn_id"] = "manual-post-ai-logs-refresh"
	crud["clear_logs_btn_id"] = "manual-post-ai-logs-clear"
	crud["level_filter_id"] = "manual-post-ai-logs-level-filter"
	crud["event_filter_id"] = "manual-post-ai-logs-event-filter"
	return gin.H{
		"title":                 title,
		"admin_nav":             "logs",
		"admin_logs_subnav":     "manual_post_ai",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteSetting.SiteName,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

// adminSitePage builds gin.H for admin HTML pages that use partial admin-crud-list (.crud).
// navKey drives sidebar active state: dashboard, users, categories, posts, tags, settings.
func adminSitePage(db *gorm.DB, title, navKey string, crud gin.H) gin.H {
	siteMeta := loadSiteMeta(db)
	siteSetting := loadSiteSetting(db)
	return gin.H{
		"title":                 title,
		"admin_nav":             navKey,
		"admin_logs_subnav":     "",
		"admin_settings_subnav": "",
		"admin_pages_subnav":    "",
		"admin_posts_subnav":    "",
		"meta_title":            title + " - " + siteMeta.DefaultTitle,
		"meta_description":      siteMeta.DefaultDesc,
		"meta_keywords":         siteMeta.DefaultKeywords,
		"meta_image":            siteMeta.DefaultImage,
		"site_name":             siteSetting.SiteName,
		"logo_url":              siteSetting.LogoURL,
		"favicon_url":           siteSetting.FaviconURL,
		"theme_color":           themeColorMeta(siteSetting),
		"asset_version":         adminAssetQueryVersion(),
		"login_href":            entity.EffectiveLoginHref(siteSetting.LoginURL),
		"crud":                  crud,
	}
}

func executeAdminTemplate(c *gin.Context, tmpl *template.Template, db *gorm.DB, data gin.H) error {
	if data == nil {
		data = gin.H{}
	}
	ss := loadSiteSetting(db)
	data["article_slug_mode"] = entity.NormalizeArticleSlugMode(ss.ArticleSlugMode)
	data["public_article_path_prefix"] = entity.PublicArticlePathPrefix()
	injectAdminAbsoluteBaseURL(c, data)
	adminInjectCurrentUser(c, db, data)
	return tmpl.ExecuteTemplate(c.Writer, "layouts-admin-app", data)
}

func adminInjectCurrentUser(c *gin.Context, db *gorm.DB, data gin.H) {
	if c == nil || db == nil || data == nil {
		return
	}
	userIDAny, ok := c.Get("userID")
	if !ok {
		return
	}
	userID, ok := userIDAny.(uint)
	if !ok || userID == 0 {
		return
	}
	var u entity.User
	if err := db.Select("id, name, image").First(&u, userID).Error; err != nil {
		return
	}
	data["admin_user_name"] = strings.TrimSpace(u.Name)
	if u.Image != nil {
		data["admin_user_image"] = strings.TrimSpace(*u.Image)
	}
}
