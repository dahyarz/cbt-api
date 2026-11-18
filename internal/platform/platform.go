package platform

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewPostgres(dsn string, maxOpen, maxIdle int, lifetime time.Duration) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil { return nil, err }
	sqlDB, err := db.DB()
	if err != nil { return nil, err }
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(lifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil { return nil, err }
	return db, nil
}

func NewRedis(addr, pass string, db int) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: pass, DB: db, PoolSize: 100, MinIdleConns: 10})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil { return nil, err }
	return rdb, nil
}

type ObjectStorage struct {
	internal, presign *minio.Client
	Bucket            string
	PresignTTL        time.Duration
}

func NewObjectStorage(ep, pub, ak, sk, bucket string, secure bool, ttl time.Duration) (*ObjectStorage, error) {
	in, err := minio.New(ep, &minio.Options{Creds: credentials.NewStaticV4(ak, sk, ""), Secure: false})
	if err != nil { return nil, err }
	pre, err := minio.New(pub, &minio.Options{Creds: credentials.NewStaticV4(ak, sk, ""), Secure: secure})
	if err != nil { return nil, err }
	return &ObjectStorage{internal: in, presign: pre, Bucket: bucket, PresignTTL: ttl}, nil
}

func (s *ObjectStorage) BucketExists(ctx context.Context) (bool, error) {
	return s.internal.BucketExists(ctx, s.Bucket)
}

func (s *ObjectStorage) PresignedGet(ctx context.Context, key string) (string, error) {
	u, err := s.presign.PresignedGetObject(ctx, s.Bucket, key, s.PresignTTL, nil)
	if err != nil { return "", err }
	return u.String(), nil
}

func (s *ObjectStorage) PresignedPut(ctx context.Context, key string) (string, error) {
	u, err := s.presign.PresignedPutObject(ctx, s.Bucket, key, s.PresignTTL)
	if err != nil { return "", err }
	return u.String(), nil
}
