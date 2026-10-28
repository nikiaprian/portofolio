package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/internal/ai"
	"github.com/nikiaprian/tirtayasa/internal/middleware"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes/handler"
	"github.com/nikiaprian/tirtayasa/internal/services"
	"github.com/nikiaprian/tirtayasa/internal/social"
)

// function to routes
func RegisterRoutes(r *gin.Engine, db *gorm.DB, cfg config.Config) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// create the repositories
	userRepo := repositories.NewUserRepository(db)
	pageRepo := repositories.NewPageRepository(db)
	postRepo := repositories.NewPostRepository(db)
	categoryRepo := repositories.NewCategoryRepository(db)
	tagRepo := repositories.NewTagRepository(db)
	siteSettingRepo := repositories.NewSiteSettingRepository(db)
	siteConnRepo := repositories.NewSiteConnectionRepository(db)
	siteCloudStorageRepo := repositories.NewSiteCloudStorageRepository(db)
	siteWebImageProviderRepo := repositories.NewSiteWebImageProviderRepository(db)
	scraperSettingRepo := repositories.NewScraperSettingRepository(db)
	scraperCategoryFeedRepo := repositories.NewScraperCategoryFeedRepository(db)
	aiSettingRepo := repositories.NewAISettingRepository(db)

	// create the JWT manager
	jwtMgr := middleware.NewJWTManager(cfg.JWTSecret, cfg.JWTExpiresInHours)

	// create the services
	userSvc := services.NewUserService(userRepo)
	pageSvc := services.NewPageService(pageRepo)
	postSvc := services.NewPostService(postRepo, categoryRepo, tagRepo, userRepo, db, cfg)
	categorySvc := services.NewCategoryService(categoryRepo, postRepo, db, cfg)
	tagSvc := services.NewTagService(tagRepo)
	siteSettingSvc := services.NewSiteSettingService(siteSettingRepo)
	siteConnSvc := services.NewSiteConnectionService(siteConnRepo)
	siteCloudStorageSvc := services.NewSiteCloudStorageService(siteCloudStorageRepo)
	siteWebImageProviderSvc := services.NewSiteWebImageProviderService(siteWebImageProviderRepo)
	scraperSettingSvc := services.NewScraperSettingService(scraperSettingRepo, scraperCategoryFeedRepo, db)
	aiSettingSvc := services.NewAISettingService(aiSettingRepo)
	authSvc := services.NewAuthService(userRepo, jwtMgr, db)

	aiGen := ai.NewGenerator(ai.NewRepoKeysProvider(aiSettingRepo))
	autoPoster := social.NewAutoPoster(db)

	// create the handlers
	userHandler := handler.NewUserHandler(userSvc, db, cfg)
	pageHandler := handler.NewPageHandler(pageSvc, db, cfg)
	postHandler := handler.NewPostHandler(postSvc, aiGen, cfg, aiSettingRepo, db, autoPoster)
	aiSettingHandler := handler.NewAISettingHandler(aiSettingSvc, db, cfg)
	categoryHandler := handler.NewCategoryHandler(categorySvc, db)
	tagHandler := handler.NewTagHandler(tagSvc, db)
	siteSettingHandler := handler.NewSiteSettingHandler(siteSettingSvc, db, cfg)
	siteConnHandler := handler.NewSiteConnectionHandler(siteConnSvc, autoPoster)
	siteCloudStorageHandler := handler.NewSiteCloudStorageHandler(siteCloudStorageSvc)
	siteWebImageProviderHandler := handler.NewSiteWebImageProviderHandler(siteWebImageProviderSvc)
	scraperSettingHandler := handler.NewScraperSettingHandler(scraperSettingSvc, db, aiGen, cfg)
	authHandler := handler.NewAuthHandler(authSvc)
	loginLogHandler := handler.NewLoginLogHandler(db)
	bulkPostLogHandler := handler.NewBulkPostLogHandler(db)
	manualPostAILogHandler := handler.NewManualPostAILogHandler(db)
	dbBackupHandler := handler.NewDatabaseBackupHandler(cfg)

	// routes
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{

			client := v1.Group("/")
			client.Use(middleware.PublicSearchLimit())
			{
				client.GET("/posts", postHandler.ListPublished)
				client.GET("/posts/popular", postHandler.ListPopular)
				client.GET("/posts/slug/:slug", postHandler.GetPublishedBySlug)
				client.GET("/pages/slug/:slug", pageHandler.GetPublicBySlug)
				client.GET("/categories", categoryHandler.List)
				client.GET("/authors/:slug", userHandler.GetPublicBySlug)
			}

			// public auth routes (no auth required)
			authGroup := v1.Group("/auth")
			{
				authGroup.POST("/register", authHandler.Register)
				authGroup.POST("/login", authHandler.Login)
				authGroup.POST("/logout", authHandler.Logout)
			}

			// protected routes (requires JWT auth)
			protected := v1.Group("/admin")
			protected.Use(middleware.JWTMiddleware(jwtMgr))
			{
				protected.GET("/profile", userHandler.Profile)
				protected.PUT("/profile", userHandler.UpdateProfile)
				protected.POST("/profile/upload-image", userHandler.UploadProfileImage)

				adminOnly := protected.Group("")
				adminOnly.Use(middleware.RoleMiddleware(db, "admin"))
				{
					adminOnly.GET("/site-setting", siteSettingHandler.Get)
					adminOnly.PUT("/site-setting", siteSettingHandler.Update)
					adminOnly.PUT("/site-setting/scripts", siteSettingHandler.UpdateScripts)
					adminOnly.POST("/site-setting/upload-image", siteSettingHandler.UploadImage)
					adminOnly.POST("/site-setting/post-date-url-recycle/run", siteSettingHandler.PostDateUrlRecycleRunNow)

					adminOnly.GET("/site-connections", siteConnHandler.Get)
					adminOnly.PUT("/site-connections", siteConnHandler.Update)
					adminOnly.POST("/site-connections/test-autopost", siteConnHandler.TestAutoPost)
					adminOnly.GET("/site-cloud-storage", siteCloudStorageHandler.Get)
					adminOnly.PUT("/site-cloud-storage", siteCloudStorageHandler.Update)
					adminOnly.GET("/site-web-image-providers", siteWebImageProviderHandler.List)
					adminOnly.PUT("/site-web-image-providers", siteWebImageProviderHandler.Update)
					adminOnly.GET("/scraper-setting", scraperSettingHandler.Get)
					adminOnly.PUT("/scraper-setting", scraperSettingHandler.Update)
					adminOnly.POST("/scraper-setting/run-now", scraperSettingHandler.RunNow)
					adminOnly.GET("/scraper-logs", scraperSettingHandler.ListLogs)
					adminOnly.DELETE("/scraper-logs", scraperSettingHandler.ClearLogs)
					adminOnly.GET("/login-logs", loginLogHandler.List)
					adminOnly.DELETE("/login-logs", loginLogHandler.Clear)
					adminOnly.DELETE("/bulk-post-logs", bulkPostLogHandler.Clear)
					adminOnly.DELETE("/manual-post-ai-logs", manualPostAILogHandler.Clear)

					adminOnly.GET("/ai-setting", aiSettingHandler.Get)
					adminOnly.PUT("/ai-setting", aiSettingHandler.Update)
					adminOnly.GET("/database/export", dbBackupHandler.ExportSQL)
					adminOnly.POST("/database/import", dbBackupHandler.ImportSQL)

					// users routes
					users := adminOnly.Group("/users")
					{
						users.GET("", userHandler.List)
						users.POST("", userHandler.Create)
						users.GET("/:id", userHandler.GetByID)
						users.PUT("/:id", userHandler.Update)
						users.DELETE("/:id", userHandler.Delete)
					}

					// pages routes
					pages := adminOnly.Group("/pages")
					{
						pages.GET("", pageHandler.List)
						pages.GET("/:id", pageHandler.GetByID)
						pages.POST("", pageHandler.Create)
						pages.POST("/upload-image", pageHandler.UploadImage)
						pages.POST("/bulk-status", pageHandler.BulkUpdateStatus)
						pages.POST("/bulk-index-gsc", pageHandler.BulkIndexGSC)
						pages.POST("/:id/index-gsc", pageHandler.IndexGSC)
						pages.PUT("/:id", pageHandler.Update)
						pages.DELETE("/:id", pageHandler.Delete)
					}

					// categories CRUD for admin
					categoriesAdmin := adminOnly.Group("/categories")
					{
						categoriesAdmin.POST("", categoryHandler.Create)
						categoriesAdmin.POST("/bulk-index-gsc", categoryHandler.BulkIndexGSC)
						categoriesAdmin.GET("/:id", categoryHandler.GetByID)
						categoriesAdmin.POST("/:id/index-gsc", categoryHandler.IndexGSC)
						categoriesAdmin.PUT("/:id", categoryHandler.Update)
						categoriesAdmin.DELETE("/:id", categoryHandler.Delete)
					}
				}

				writerOrAdmin := protected.Group("")
				writerOrAdmin.Use(middleware.RoleMiddleware(db, "admin", "writer"))
				{
					// Bulk post prompts (shared ai_setting row; writers need this to persist from /admin/bulk-posts)
					writerOrAdmin.PUT("/ai-setting/bulk-post-template", aiSettingHandler.UpdateBulkPostPromptTemplate)
					writerOrAdmin.POST("/ai-setting/upload-bulk-cover-background", aiSettingHandler.UploadBulkCoverBackground)
					writerOrAdmin.GET("/bulk-post-logs", bulkPostLogHandler.List)
					writerOrAdmin.GET("/manual-post-ai-logs", manualPostAILogHandler.List)

					// category read for writer/admin
					categories := writerOrAdmin.Group("/categories")
					{
						categories.GET("", categoryHandler.List)
					}
					// tags CRUD for writer/admin
					tags := writerOrAdmin.Group("/tags")
					{
						tags.GET("", tagHandler.List)
						tags.POST("", tagHandler.Create)
						tags.POST("/bulk-index-gsc", tagHandler.BulkIndexGSC)
						tags.GET("/:id", tagHandler.GetByID)
						tags.POST("/:id/index-gsc", tagHandler.IndexGSC)
						tags.PUT("/:id", tagHandler.Update)
						tags.DELETE("/:id", tagHandler.Delete)
					}
					// posts CRUD for writer/admin (writer limited to own posts in handler)
					posts := writerOrAdmin.Group("/posts")
					{
						posts.GET("", postHandler.List)
						posts.POST("/generate-draft", postHandler.GenerateDraft)
						posts.POST("/generate-image", postHandler.GenerateImage)
						posts.POST("/bulk-suggest-titles", postHandler.BulkSuggestTitles)
						posts.POST("/bulk-regenerate-titles", postHandler.BulkRegenerateTitles)
						posts.POST("/bulk-scrape-xml-titles", postHandler.BulkScrapeXMLTitles)
						posts.POST("/bulk-check-titles", postHandler.BulkCheckTitles)
						posts.POST("/bulk-generate", postHandler.BulkGeneratePosts)
						posts.GET("/bulk-generate-jobs-active", postHandler.ActiveBulkGenerateJob)
						posts.GET("/bulk-generate-jobs/:id", postHandler.BulkGenerateJobStatus)
						posts.POST("/bulk-generate-jobs/:id/cancel", postHandler.CancelBulkGenerateJob)
						posts.POST("/upload-image", postHandler.UploadImage)
						posts.POST("/bulk-status", postHandler.BulkUpdateStatus)
						posts.POST("/bulk-index-gsc", postHandler.BulkIndexGSC)
						posts.POST("/:id/index-gsc", postHandler.IndexGSC)
						posts.GET("/:id", postHandler.GetByID)
						posts.POST("", postHandler.Create)
						posts.PUT("/:id", postHandler.Update)
						posts.DELETE("/:id", postHandler.Delete)
					}
				}
			}
		}
	}
}
