package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
)

func (d *Deps) Live(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (d *Deps) Ready(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	ready := true

	sqlDB, err := d.DB.DB()
	if err == nil {
		err = sqlDB.PingContext(ctx)
	}
	if err != nil {
		checks["postgres"] = "down"
		ready = false
	} else {
		checks["postgres"] = "up"
	}

	if err := d.Redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down"
		ready = false
	} else {
		checks["redis"] = "up"
	}

	if ok, err := d.Store.BucketExists(ctx); err != nil || !ok {
		checks["minio"] = "degraded"
	} else {
		checks["minio"] = "up"
	}

	if !ready {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not_ready", "checks": checks})
	}
	return c.JSON(fiber.Map{"status": "ready", "checks": checks})
}
