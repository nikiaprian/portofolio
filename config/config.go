package config

import (
	"os"
	"strconv"
)

// config struct
type Config struct {
	HTTPPort string

	DBDriver   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	AppEnv            string
	RunSeedOnStartup  bool
	LocalTimezone     string
	JWTSecret         string
	JWTExpiresInHours int

	R2AccountID       string
	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Bucket          string
	R2PublicBaseURL   string
	R2Prefix          string

	RedisAddr                 string
	RedisPassword             string
	RedisDB                   int
	RedisCacheTTL             int
	RedisCacheNS              string
	RedisCacheAdminAuthorized bool

	// AdminLogRetentionHours is a rolling window (hours) for scraper_logs, login_logs, and bulk_post_logs.
	// Values <= 0 disable automatic deletion.
	AdminLogRetentionHours int
}

// function to load the config
func Load() Config {
	jwtExpires, _ := strconv.Atoi(getenv("JWT_EXPIRES_IN_HOURS", "24"))
	redisDB, _ := strconv.Atoi(getenv("REDIS_DB", "0"))
	redisCacheTTL, _ := strconv.Atoi(getenv("REDIS_CACHE_TTL_SECONDS", "180"))
	redisCacheAdminAuthorized := getenv("REDIS_CACHE_ADMIN_AUTHORIZED", "false") == "true"
	runSeedOnStartup := getenv("RUN_SEED_ON_STARTUP", "false") == "true"
	adminLogRetentionHours, _ := strconv.Atoi(getenv("ADMIN_LOG_RETENTION_HOURS", "24"))

	return Config{
		HTTPPort: getenv("HTTP_PORT", "8080"),

		DBDriver:   getenv("DB_DRIVER", "mysql"),
		DBHost:     getenv("DB_HOST", "127.0.0.1"),
		DBPort:     getenv("DB_PORT", "3306"),
		DBUser:     getenv("DB_USER", "root"),
		DBPassword: getenv("DB_PASSWORD", ""),
		DBName:     getenv("DB_NAME", "tirtayasa"),

		AppEnv:            getenv("APP_ENV", "development"),
		RunSeedOnStartup:  runSeedOnStartup,
		LocalTimezone:     getenv("LOCAL_TIMEZONE", "Asia/Jakarta"),
		JWTSecret:         getenv("JWT_SECRET", "super-secret-key"),
		JWTExpiresInHours: jwtExpires,

		R2AccountID:       getenv("R2_ACCOUNT_ID", ""),
		R2AccessKeyID:     getenv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:          getenv("R2_BUCKET", ""),
		R2PublicBaseURL:   getenv("R2_PUBLIC_BASE_URL", ""),
		R2Prefix:          getenv("R2_PREFIX", "uploads"),

		RedisAddr:                 getenv("REDIS_ADDR", ""),
		RedisPassword:             getenv("REDIS_PASSWORD", ""),
		RedisDB:                   redisDB,
		RedisCacheTTL:             redisCacheTTL,
		RedisCacheNS:              getenv("REDIS_CACHE_NAMESPACE", "tirtayasa"),
		RedisCacheAdminAuthorized: redisCacheAdminAuthorized,

		AdminLogRetentionHours: adminLogRetentionHours,
	}
}

// function to get the environment variable
func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
