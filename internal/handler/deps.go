package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sasadu/cbt-api/internal/config"
	"github.com/sasadu/cbt-api/internal/model"
	"github.com/sasadu/cbt-api/internal/platform"
)

type Deps struct {
	DB    *gorm.DB
	Redis *redis.Client
	Store *platform.ObjectStorage
	Cfg   *config.Config
}

var (
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

func (d *Deps) Version(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"version": Version, "commit": Commit, "build_time": BuildTime, "instance": d.Cfg.InstanceID})
}

func errJSON(c *fiber.Ctx, code int, msg string) error {
	return c.Status(code).JSON(fiber.Map{"error": msg})
}

// loadAttempt mengambil attempt milik user yang sedang login.
func (d *Deps) loadAttempt(c *fiber.Ctx) (*model.ExamAttempt, error) {
	uid, _ := c.Locals("user_id").(string)
	var a model.ExamAttempt
	if err := d.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", c.Params("id"), uid).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fiber.NewError(fiber.StatusNotFound, "attempt tidak ditemukan")
		}
		return nil, err
	}
	return &a, nil
}

func (d *Deps) remainingSeconds(a *model.ExamAttempt, durationMinutes int) int64 {
	if a.StartedAt == nil || durationMinutes <= 0 {
		return 0
	}
	deadline := a.StartedAt.Add(time.Duration(durationMinutes) * time.Minute)
	left := int64(time.Until(deadline).Seconds())
	if left < 0 { return 0 }
	return left
}
