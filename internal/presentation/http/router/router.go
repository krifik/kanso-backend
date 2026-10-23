package router

import (
	"kanso/backend/internal/presentation/http/handler"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Handlers struct {
	HealthHandler *handler.HealthHandler
}

func SetupRoutes(app *fiber.App, h Handlers, corsOrigins string) {
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	api := app.Group("/api/v1")
	api.Get("/health", h.HealthHandler.Check)
}
