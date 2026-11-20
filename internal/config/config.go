package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                string
	AppPort               string
	AutoMigrate           bool
	MigrationsDir         string
	JWTSecret             string
	JWTExpireHours        int
	JWTRefreshExpireHours int

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	RedisHost     string
	RedisPort     string
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	S3Provider       string
	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKeyID    string
	S3SecretAccessKey string
	S3PublicBaseURL  string

	CORSAllowedOrigins string
}

func Load() (*Config, error) {
	// Load .env if present; ignore error in production where env vars are injected directly.
	_ = godotenv.Load()

	redisHost := getEnv("REDIS_HOST", "")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisAddr := getEnv("REDIS_ADDR", "")
	if redisHost != "" {
		redisAddr = fmt.Sprintf("%s:%s", redisHost, redisPort)
	} else if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	s3Endpoint := strings.TrimRight(getEnv("S3_ENDPOINT", ""), "/")
	if s3Endpoint == "" {
		if accountID := getEnv("R2_ACCOUNT_ID", ""); accountID != "" {
			s3Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
		}
	}

	cfg := &Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		AppPort:               getEnv("APP_PORT", "8080"),
		AutoMigrate:           getEnvBool("AUTO_MIGRATE", true),
		MigrationsDir:         getEnv("MIGRATIONS_DIR", "migrations"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		JWTExpireHours:        getEnvInt("JWT_EXPIRE_HOURS", 168),
		JWTRefreshExpireHours: getEnvInt("JWT_REFRESH_EXPIRE_HOURS", 720),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "edugame"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "edugame_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		RedisHost:     redisHost,
		RedisPort:     redisPort,
		RedisAddr:     redisAddr,
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		S3Provider:        getEnv("S3_PROVIDER", "R2"),
		S3Endpoint:        s3Endpoint,
		S3Region:          getEnv("S3_REGION", "auto"),
		S3Bucket:          firstNonEmpty(getEnv("S3_BUCKET", ""), getEnv("R2_BUCKET_NAME", "")),
		S3AccessKeyID:     firstNonEmpty(getEnv("S3_ACCESS_KEY_ID", ""), getEnv("R2_ACCESS_KEY_ID", "")),
		S3SecretAccessKey: firstNonEmpty(getEnv("S3_SECRET_ACCESS_KEY", ""), getEnv("R2_SECRET_ACCESS_KEY", "")),
		S3PublicBaseURL:   strings.TrimRight(firstNonEmpty(getEnv("S3_PUBLIC_BASE_URL", ""), getEnv("R2_PUBLIC_URL", "")), "/"),

		CORSAllowedOrigins: getEnv(
			"CORS_ALLOWED_ORIGINS",
			"capacitor://localhost,https://localhost,http://localhost:3000",
		),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if len(c.JWTSecret) < 16 {
		return fmt.Errorf("JWT_SECRET must be set and at least 16 characters long")
	}
	if c.DBHost == "" || c.DBUser == "" || c.DBName == "" {
		return fmt.Errorf("DB_HOST, DB_USER, and DB_NAME are required")
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=Asia/Ho_Chi_Minh",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}
