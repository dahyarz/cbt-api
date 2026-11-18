package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/sasadu/cbt-api/internal/config"
	"github.com/sasadu/cbt-api/internal/handler"
	"github.com/sasadu/cbt-api/internal/platform"
	"github.com/sasadu/cbt-api/internal/router"
)

var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil { log.Fatalf("config: %v", err) }

	db, err := platform.NewPostgres(cfg.DBDSN, cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBMaxLifetime)
	if err != nil { log.Fatalf("postgres: %v", err) }

	rdb, err := platform.NewRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil { log.Fatalf("redis: %v", err) }

	store, err := platform.NewObjectStorage(cfg.MinioEndpoint, cfg.MinioPublicEndpoint,
		cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioPublicSecure, cfg.MinioPresignTTL)
	if err != nil { log.Fatalf("minio: %v", err) }

	handler.Version = version
	handler.Commit = commit
	handler.BuildTime = buildTime

	d := &handler.Deps{DB: db, Redis: rdb, Store: store, Cfg: cfg}
	app := router.New(cfg, d)

	go func() {
		log.Printf("cbt-api [%s] %s listening :%s", cfg.InstanceID, version, cfg.HTTPPort)
		if err := app.Listen(":" + cfg.HTTPPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(nil, os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	log.Println("shutdown...")
	_ = app.Shutdown()
	if sqlDB, err := db.DB(); err == nil { _ = sqlDB.Close() }
	_ = rdb.Close()
}
