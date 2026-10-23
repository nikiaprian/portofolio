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

	"github.com/nikiaprian/tirtayasa/config"
	"github.com/nikiaprian/tirtayasa/db"
	"github.com/nikiaprian/tirtayasa/db/migrations"
	"github.com/nikiaprian/tirtayasa/db/seeds"
	"github.com/nikiaprian/tirtayasa/internal/ai"
	"github.com/nikiaprian/tirtayasa/internal/cache"
	"github.com/nikiaprian/tirtayasa/internal/media"
	"github.com/nikiaprian/tirtayasa/internal/helper/adminlogprune"
	"github.com/nikiaprian/tirtayasa/internal/helper/scheduledposts"
	"github.com/nikiaprian/tirtayasa/internal/repositories"
	"github.com/nikiaprian/tirtayasa/internal/routes"
	"github.com/nikiaprian/tirtayasa/internal/scraper"
	"github.com/nikiaprian/tirtayasa/internal/social"
)

func main() {
	seedOnly := flag.Bool("seed-only", false, "run migrations + seeders, then exit")
	flag.Parse()

	_ = godotenv.Load()

	cfg := config.Load()

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

	if *seedOnly {
		if err := seeds.Run(db); err != nil {
			log.Fatalf("failed to run seeders: %v", err)
		}
		log.Println("seed complete")
		return
	}

	if cfg.AppEnv == "development" || cfg.RunSeedOnStartup {
		if err := seeds.Run(db); err != nil {
			log.Fatalf("failed to run seeders: %v", err)
		}
	}

	aiSettingRepo := repositories.NewAISettingRepository(db)
	aiGen := ai.NewGenerator(ai.NewRepoKeysProvider(aiSettingRepo))
	autoPoster := social.NewAutoPoster(db)

	adminlogprune.Init(db, cfg.AdminLogRetentionHours)
	adminlogprune.StartBackground()

	scraper.StartNewsScraper(db, aiGen, cfg)
	scheduledposts.StartPublishDueWorker(db, autoPoster)

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

	routes.RegisterSlugValidator()

	// frontend client routes
	routes.RegisterFrontendRoutes(r, db, cfg)

	// API routes
	routes.RegisterRoutes(r, db, cfg)

	addr := ":" + cfg.HTTPPort
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	if err := r.Run(addr); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
