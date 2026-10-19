package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/Bebii-Digital/bebii-blog/config"
	"github.com/Bebii-Digital/bebii-blog/db"
	"github.com/Bebii-Digital/bebii-blog/db/migrations"
	"github.com/Bebii-Digital/bebii-blog/db/seeds"
	"github.com/Bebii-Digital/bebii-blog/internal/ai"
	"github.com/Bebii-Digital/bebii-blog/internal/cache"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/admin"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/validation"
	"github.com/Bebii-Digital/bebii-blog/internal/helper/worker"
	"github.com/Bebii-Digital/bebii-blog/internal/media"
	"github.com/Bebii-Digital/bebii-blog/internal/repositories"
	"github.com/Bebii-Digital/bebii-blog/internal/routes"
	"github.com/Bebii-Digital/bebii-blog/internal/scraper"
	"github.com/Bebii-Digital/bebii-blog/internal/social"
)

func main() {
	seedOnly := flag.Bool("seed-only", false, "run migrations + seeders, then exit")
	reapplyLayoutOnly := flag.Bool("reapply-layout", false, "re-apply active site layout (is_used) to public/templates, then exit")
	flag.Parse()

	_ = godotenv.Load()

	cfg := config.Load()

	cache.InitTTLs(cache.TTLConfig{
		SiteSetting:   durationSeconds(cfg.SiteSettingCacheTTL),
		NavCategories: durationSeconds(cfg.NavCategoriesCacheTTL),
		PostViewRank:  durationSeconds(cfg.PostViewRankCacheTTL),
		ImageResize:   durationSeconds(cfg.ImageResizeCacheTTL),
	})

	if err := media.InitVips(); err != nil {
		log.Printf("libvips unavailable, image resize uses Go fallback: %v", err)
	} else {
		defer media.ShutdownVips()
	}

	db, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := migrations.Run(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	if err := seeds.EnsureAISettingDefaults(db); err != nil {
		log.Fatalf("failed to ensure ai_setting defaults: %v", err)
	}

	if *reapplyLayoutOnly {
		if err := seeds.ReapplyUsedLayout(db); err != nil {
			log.Fatalf("failed to reapply used layout: %v", err)
		}
		log.Println("layout reapply complete")
		return
	}

	if *seedOnly {
		if err := seeds.Run(db); err != nil {
			log.Fatalf("failed to run seeders: %v", err)
		}
		if cfg.AppEnv == "development" {
			if err := seeds.EnsureDemoPortfolio(db); err != nil {
				log.Fatalf("failed to seed demo portfolio: %v", err)
			}
		}
		log.Println("seed complete")
		return
	}

	if cfg.AppEnv == "development" || cfg.RunSeedOnStartup {
		if err := seeds.Run(db); err != nil {
			log.Fatalf("failed to run seeders: %v", err)
		}
	}
	if cfg.AppEnv == "development" {
		if err := seeds.EnsureDemoPortfolio(db); err != nil {
			log.Fatalf("failed to seed demo portfolio: %v", err)
		}
	}

	aiSettingRepo := repositories.NewAISettingRepository(db)
	aiGen := ai.NewGenerator(ai.NewRepoKeysProvider(aiSettingRepo))
	autoPoster := social.NewAutoPoster(db)

	adminutil.Init(db, cfg.AdminLogRetentionHours)
	adminutil.StartBackground()

	scraper.StartNewsScraper(db, aiGen, cfg)
	worker.StartPublishDueWorker(db, autoPoster)

	r := gin.Default()
	// Default charset untuk halaman HTML (handler JSON/XML/Data menimpa Content-Type).
	r.Use(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/static/") || p == "/img" || strings.HasPrefix(p, "/img/") || strings.HasPrefix(p, "/img?") {
			c.Next()
			return
		}
		if p == "/sitemap.xml" || p == "/health" || p == "/robots.txt" || p == "/ads.txt" {
			c.Next()
			return
		}
		/* Feed RSS: /rss/:slug — bukan /rss-feeds (halaman HTML). */
		if strings.HasPrefix(p, "/rss/") {
			c.Next()
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.Next()
	})
	// Kompresi respons teks (HTML/JSON/CSS/JS); gambar sudah terkompresi — dikecualikan oleh middleware.
	r.Use(gzip.Gzip(gzip.DefaultCompression,
		gzip.WithMinLength(512),
		gzip.WithExcludedPaths([]string{
			"/robots.txt", "/ads.txt",
		}),
		gzip.WithExcludedExtensions([]string{
			".png", ".gif", ".jpeg", ".jpg", ".webp", ".ico", ".woff", ".woff2", ".eot", ".ttf",
		}),
	))

	var redisClient *redis.Client
	if cfg.RedisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisAddr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		if err := redisClient.Ping(context.Background()).Err(); err != nil {
			log.Printf("redis disabled: failed ping %s: %v", cfg.RedisAddr, err)
			redisClient = nil
		} else {
			ttl := time.Duration(cfg.RedisCacheTTL) * time.Second
			cache.InitStore(cache.StoreConfig{
				Client:     redisClient,
				Namespace:  cfg.RedisCacheNS,
				DefaultTTL: ttl,
			})
			r.Use(cache.NewRedisWriteInvalidator(redisClient, cfg.RedisCacheNS))
			r.Use(cache.NewRedisHTTPReadCache(redisClient, cache.HTTPCacheConfig{
				TTL:                  ttl,
				Namespace:            cfg.RedisCacheNS,
				CacheAuthorizedAdmin: cfg.RedisCacheAdminAuthorized,
			}))
			log.Printf("redis cache enabled: addr=%s ttl=%s namespace=%s admin_auth_cache=%t", cfg.RedisAddr, ttl, cfg.RedisCacheNS, cfg.RedisCacheAdminAuthorized)
		}
	}

	validation.RegisterSlugValidator()

	// Tenant hosts are resolved before API and blog routes so a subdomain cannot fall through to articles.
	r.Use(routes.PortfolioHostMiddleware(db, cfg))

	// API routes first so they are always registered before frontend catch-all handlers.
	routes.RegisterRoutes(r, db, cfg)

	// frontend client routes
	routes.RegisterFrontendRoutes(r, db, cfg)

	addr := ":" + cfg.HTTPPort
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	if err := r.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func durationSeconds(sec int) time.Duration {
	if sec <= 0 {
		return 0
	}
	return time.Duration(sec) * time.Second
}
