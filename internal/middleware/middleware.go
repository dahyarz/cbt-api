package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/sasadu/cbt-api/internal/auth"
)

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

func Auth(secret string, rdb *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		h := c.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token tidak ada"})
		}
		claims, err := auth.Parse(secret, strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "token tidak valid"})
		}
		sid, err := rdb.Get(context.Background(), "session:"+claims.Subject).Result()
		if err != nil || sid != claims.SessionID {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "sesi tidak aktif"})
		}
		c.Locals("user_id", claims.Subject)
		c.Locals("session_id", claims.SessionID)
		c.Locals("role", claims.Role)
		return c.Next()
	}
}
