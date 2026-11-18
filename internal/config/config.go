package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv, InstanceID, HTTPPort string
	DBDSN                       string
	DBMaxOpen, DBMaxIdle        int
	DBMaxLifetime               time.Duration
	RedisAddr, RedisPassword    string
	RedisDB                     int
	MinioEndpoint               string
	MinioPublicEndpoint         string
	MinioPublicSecure           bool
	MinioAccessKey, MinioSecretKey, MinioBucket string
	MinioPresignTTL             time.Duration
	JWTSecret                   string
	JWTAccessTTL, JWTRefreshTTL time.Duration
	WorkerEnabled               bool
}

func Load() (*Config, error) {
	c := &Config{
		AppEnv:       get("APP_ENV", "development"),
		InstanceID:   get("INSTANCE_ID", "local"),
		HTTPPort:     get("HTTP_PORT", "8080"),
		DBDSN:        os.Getenv("DB_DSN"),
		DBMaxOpen:    getInt("DB_MAX_OPEN", 20),
		DBMaxIdle:    getInt("DB_MAX_IDLE", 10),
		DBMaxLifetime: getDur("DB_MAX_LIFETIME", 30*time.Minute),
		RedisAddr:    get("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:      getInt("REDIS_DB", 0),
		MinioEndpoint: get("MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinioPublicEndpoint: os.Getenv("MINIO_PUBLIC_ENDPOINT"),
		MinioPublicSecure:   getBool("MINIO_PUBLIC_SECURE", true),
		MinioAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey: os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:   get("MINIO_BUCKET", "cbt"),
		MinioPresignTTL: getDur("MINIO_PRESIGN_TTL", 10*time.Minute),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTAccessTTL:  getDur("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: getDur("JWT_REFRESH_TTL", 168*time.Hour),
		WorkerEnabled: getBool("WORKER_ENABLED", false),
	}
	req := map[string]string{
		"DB_DSN": c.DBDSN, "REDIS_ADDR": c.RedisAddr, "JWT_SECRET": c.JWTSecret,
		"MINIO_ENDPOINT": c.MinioEndpoint, "MINIO_PUBLIC_ENDPOINT": c.MinioPublicEndpoint,
		"MINIO_ACCESS_KEY": c.MinioAccessKey, "MINIO_SECRET_KEY": c.MinioSecretKey,
		"MINIO_BUCKET": c.MinioBucket,
	}
	for k, v := range req {
		if v == "" {
			return nil, fmt.Errorf("config: %s wajib diisi", k)
		}
	}
	return c, nil
}

func get(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }
func getInt(k string, d int) int { if v, err := strconv.Atoi(os.Getenv(k)); err == nil { return v }; return d }
func getBool(k string, d bool) bool { if v, err := strconv.ParseBool(os.Getenv(k)); err == nil { return v }; return d }
func getDur(k string, d time.Duration) time.Duration { if v, err := time.ParseDuration(os.Getenv(k)); err == nil { return v }; return d }
