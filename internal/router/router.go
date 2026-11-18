package router

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/sasadu/cbt-api/internal/config"
	"github.com/sasadu/cbt-api/internal/handler"
	"github.com/sasadu/cbt-api/internal/middleware"
)

func New(cfg *config.Config, d *handler.Deps) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:                 "cbt-api",
		ProxyHeader:             fiber.HeaderXForwardedFor,
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"10.103.0.200"},
		ReadTimeout:             30 * time.Second,
		WriteTimeout:            30 * time.Second,
		IdleTimeout:             60 * time.Second,
		BodyLimit:               10 * 1024 * 1024,
	})

	app.Use(middleware.RequestID())
	app.Get("/healthz", d.Live)
	app.Get("/readyz", d.Ready)

	api := app.Group("/api/v1")
	api.Post("/auth/login", d.Login)
	api.Post("/auth/refresh", d.Refresh)

	authed := api.Group("", middleware.Auth(cfg.JWTSecret, d.Redis))
	authed.Post("/auth/logout", d.Logout)
	authed.Get("/me", d.Me)
	authed.Get("/version", d.Version)
	authed.Post("/devices/register", d.RegisterDevice)

	authed.Get("/exams", d.ListExams)
	authed.Get("/exams/:id", d.GetExam)
	authed.Post("/exams/:id/validate-token", d.ValidateToken)
	authed.Post("/exams/:id/start", d.StartExam)

	authed.Get("/attempts/:id", d.GetAttempt)
	authed.Put("/attempts/:id/answers/:question_id", d.SaveAnswer)
	authed.Post("/attempts/:id/answers/batch", d.SaveAnswersBatch)
	authed.Post("/attempts/:id/flag", d.FlagQuestion)
	authed.Post("/attempts/:id/heartbeat", d.Heartbeat)
	authed.Post("/attempts/:id/submit", d.SubmitAttempt)
	authed.Get("/attempts/:id/result", d.GetResult)
	authed.Get("/attempts/:id/review", d.GetReview)

	return app
}
