package handler

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/sasadu/cbt-api/internal/model"
)

func (d *Deps) RegisterDevice(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	fp := strings.ToLower(strings.TrimSpace(c.Get("X-Device-Fingerprint")))
	if len(fp) != 64 {
		return errJSON(c, 400, "header X-Device-Fingerprint harus 64 karakter hex")
	}

	var req struct {
		DeviceName string `json:"device_name"`
	}
	_ = c.BodyParser(&req)

	ip := c.IP()
	ua := c.Get("User-Agent")
	now := time.Now()

	var dev model.DeviceFingerprint
	err := d.DB.Where("user_id = ? AND fingerprint_hash = ?", uid, fp).First(&dev).Error
	if err != nil {
		dev = model.DeviceFingerprint{
			ID: uuid.NewString(), UserID: uid, FingerprintHash: fp,
			DeviceName: &req.DeviceName, IPAddress: &ip, UserAgent: &ua,
			FirstSeen: now, LastSeen: now, TimesSeen: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := d.DB.Create(&dev).Error; err != nil {
			return errJSON(c, 500, "gagal menyimpan perangkat")
		}
	} else {
		d.DB.Model(&model.DeviceFingerprint{}).Where("id = ?", dev.ID).Updates(map[string]any{
			"last_seen": now, "ip_address": ip, "user_agent": ua,
			"times_seen": dev.TimesSeen + 1, "updated_at": now,
			"device_name": req.DeviceName,
		})
	}

	if dev.IsBlocked {
		return errJSON(c, 403, "perangkat diblokir")
	}
	return c.JSON(fiber.Map{"status": "ok", "fingerprint_hash": fp, "is_trusted": dev.IsTrusted})
}
