package main

import (
	"log"
	"kanso/backend/internal/handler"
	"kanso/backend/internal/infrastructure/config"
	"kanso/backend/internal/infrastructure/database"
	"kanso/backend/internal/repository"
	"kanso/backend/internal/router"
	"kanso/backend/internal/service"

	"github.com/gofiber/fiber/v2"
)

func main() {
	// 1. Config
	cfg := config.LoadConfig()

	// 2. Database & Seeder
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Printf("Warning: Database initialization: %v", err)
	}

	// 3. Layered Wire: Repository -> Service -> Handler
	kansoRepo := repository.NewKansoRepository(db)
	mutationRepo := repository.NewMutationRepository(db)

	kansoService := service.NewKansoService(kansoRepo)
	googleSSOService := service.NewGoogleSSOService(cfg, db)
	llmParserService := service.NewGeminiLLMParserService(cfg.LLMAPIKey, cfg.LLMModel)
	mutationService := service.NewMutationService(mutationRepo, kansoRepo, llmParserService)

	kansoHandler := handler.NewKansoHandler(kansoService)
	healthHandler := handler.NewHealthHandler()
	googleAuthHandler := handler.NewGoogleAuthHandler(googleSSOService, kansoService, mutationService, cfg.FrontendURL)
	webhookHandler := handler.NewWebhookHandler(mutationService, cfg.WebhookSecret)

	// 4. Fiber App
	app := fiber.New(fiber.Config{
		AppName: "Kanso Duo API",
	})

	handlers := router.Handlers{
		KansoHandler:      kansoHandler,
		HealthHandler:     healthHandler,
		GoogleAuthHandler: googleAuthHandler,
		WebhookHandler:    webhookHandler,
	}

	router.SetupRoutes(app, handlers, cfg.CorsOrigins)

	log.Printf("🚀 Kanso Duo Backend running on port :%s (DB: %s)\n", cfg.Port, cfg.DBDriver)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
