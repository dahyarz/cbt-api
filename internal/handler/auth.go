package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"

	"github.com/sasadu/cbt-api/internal/auth"
	"github.com/sasadu/cbt-api/internal/model"
)

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (d *Deps) Login(c *fiber.Ctx) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.BodyParser(&req); err != nil || req.Username == "" || req.Password == "" {
		return errJSON(c, 400, "username dan password wajib")
	}

	fp := c.Get("X-Device-Fingerprint")
	ip := c.IP()
	ua := c.Get("User-Agent")

	var u model.User
	if err := d.DB.Where("username = ? AND deleted_at IS NULL", req.Username).First(&u).Error; err != nil {
		_ = d.DB.Create(&model.LoginAttempt{
			ID: uuid.NewString(), Identifier: req.Username, IPAddress: ip,
			UserAgent: &ua, Success: false, CreatedAt: time.Now(),
		}).Error
		return errJSON(c, 401, "username atau password salah")
	}

	if u.Status != "active" {
		return errJSON(c, 403, "akun tidak aktif")
	}
	if u.LockedUntil != nil && u.LockedUntil.After(time.Now()) {
		return errJSON(c, 423, "akun terkunci sementara")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		fails := u.FailedLoginAttempts + 1
		upd := map[string]any{"failed_login_attempts": fails, "last_failed_login": time.Now()}
		if fails >= 5 {
			upd["locked_until"] = time.Now().Add(15 * time.Minute)
		}
		d.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(upd)
		_ = d.DB.Create(&model.LoginAttempt{
			ID: uuid.NewString(), UserID: &u.ID, Identifier: req.Username,
			IPAddress: ip, UserAgent: &ua, Success: false, CreatedAt: time.Now(),
		}).Error
		return errJSON(c, 401, "username atau password salah")
	}

	now := time.Now()
	sessionID := uuid.NewString()
	if !u.AllowMultipleSessions && u.ActiveSessionID != nil && *u.ActiveSessionID != "" {
		old := *u.ActiveSessionID
		d.DB.Create(&model.LoginSessionAudit{
			ID: uuid.NewString(), UserID: u.ID, Event: "takeover",
			OldSessionID: &old, NewSessionID: &sessionID, IPAddress: &ip, UserAgent: &ua, CreatedAt: now,
		})
		d.Redis.Del(context.Background(), "session:"+u.ID)
	}

	refresh := randomToken(32)
	refreshHash := hashToken(refresh)
	devInfo, _ := datatypes.NewJSONType(map[string]any{"platform": c.Get("X-Device-Platform"), "fingerprint": fp}).MarshalJSON()

	expires := now.Add(d.Cfg.JWTRefreshTTL)
	d.DB.Model(&model.User{}).Where("id = ?", u.ID).Updates(map[string]any{
		"active_session_id":   sessionID,
		"session_expires_at":  expires,
		"session_ip_address":  ip,
		"session_created_at":  now,
		"session_device_info": datatypes.JSON(devInfo),
		"refresh_token":       refreshHash,
		"last_login":          now,
		"last_ip":             ip,
		"login_count":         u.LoginCount + 1,
		"failed_login_attempts": 0,
		"locked_until":        nil,
	})

	d.Redis.Set(context.Background(), "session:"+u.ID, sessionID, d.Cfg.JWTRefreshTTL)
	d.Redis.Set(context.Background(), "refresh:"+refreshHash, u.ID, d.Cfg.JWTRefreshTTL)

	access, err := auth.Sign(d.Cfg.JWTSecret, d.Cfg.JWTAccessTTL, u.ID, sessionID, u.Role)
	if err != nil {
		return errJSON(c, 500, "gagal membuat token")
	}

	_ = d.DB.Create(&model.LoginAttempt{
		ID: uuid.NewString(), UserID: &u.ID, Identifier: req.Username,
		IPAddress: ip, UserAgent: &ua, Success: true, CreatedAt: now,
	}).Error
	d.DB.Create(&model.LoginSessionAudit{
		ID: uuid.NewString(), UserID: u.ID, Event: "login", NewSessionID: &sessionID,
		IPAddress: &ip, UserAgent: &ua, CreatedAt: now,
	})

	return c.JSON(fiber.Map{
		"access_token":  access,
		"refresh_token": refresh,
		"expires_in":    int(d.Cfg.JWTAccessTTL.Seconds()),
		"user": fiber.Map{
			"id": u.ID, "username": u.Username, "name": u.Name, "role": u.Role,
		},
	})
}

func (d *Deps) Refresh(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return errJSON(c, 400, "refresh_token wajib")
	}
	hash := hashToken(req.RefreshToken)
	ctx := context.Background()
	userID, err := d.Redis.Get(ctx, "refresh:"+hash).Result()
	if err != nil {
		return errJSON(c, 401, "refresh token tidak valid")
	}

	var u model.User
	if err := d.DB.Where("id = ? AND deleted_at IS NULL", userID).First(&u).Error; err != nil {
		return errJSON(c, 401, "user tidak ditemukan")
	}
	if u.RefreshToken == nil || *u.RefreshToken != hash {
		return errJSON(c, 401, "refresh token sudah dipakai")
	}
	if u.ActiveSessionID == nil || u.SessionExpiresAt == nil || u.SessionExpiresAt.Before(time.Now()) {
		return errJSON(c, 401, "sesi berakhir")
	}

	d.Redis.Del(ctx, "refresh:"+hash)
	newRefresh := randomToken(32)
	newHash := hashToken(newRefresh)
	d.DB.Model(&model.User{}).Where("id = ?", u.ID).Update("refresh_token", newHash)
	d.Redis.Set(ctx, "refresh:"+newHash, u.ID, d.Cfg.JWTRefreshTTL)
	d.Redis.Set(ctx, "session:"+u.ID, *u.ActiveSessionID, d.Cfg.JWTRefreshTTL)

	access, err := auth.Sign(d.Cfg.JWTSecret, d.Cfg.JWTAccessTTL, u.ID, *u.ActiveSessionID, u.Role)
	if err != nil {
		return errJSON(c, 500, "gagal membuat token")
	}
	return c.JSON(fiber.Map{
		"access_token": access, "refresh_token": newRefresh,
		"expires_in": int(d.Cfg.JWTAccessTTL.Seconds()),
	})
}

func (d *Deps) Logout(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	ctx := context.Background()
	d.Redis.Del(ctx, "session:"+uid)
	d.DB.Model(&model.User{}).Where("id = ?", uid).Updates(map[string]any{
		"active_session_id": nil, "session_expires_at": nil, "refresh_token": nil, "last_logout": time.Now(),
	})
	ip := c.IP()
	ua := c.Get("User-Agent")
	d.DB.Create(&model.LoginSessionAudit{
		ID: uuid.NewString(), UserID: uid, Event: "logout",
		IPAddress: &ip, UserAgent: &ua, CreatedAt: time.Now(),
	})
	return c.JSON(fiber.Map{"status": "ok"})
}

func (d *Deps) Me(c *fiber.Ctx) error {
	uid, _ := c.Locals("user_id").(string)
	var u model.User
	if err := d.DB.Where("id = ?", uid).First(&u).Error; err != nil {
		return errJSON(c, 404, "user tidak ditemukan")
	}
	return c.JSON(fiber.Map{"id": u.ID, "username": u.Username, "name": u.Name, "role": u.Role, "status": u.Status})
}

var _ = strings.TrimSpace
