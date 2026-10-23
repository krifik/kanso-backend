package router

import (
	"kanso/backend/internal/handler"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Handlers struct {
	KansoHandler      *handler.KansoHandler
	HealthHandler     *handler.HealthHandler
	GoogleAuthHandler *handler.GoogleAuthHandler
	WebhookHandler    *handler.WebhookHandler
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
	if h.HealthHandler != nil {
		api.Get("/health", h.HealthHandler.Check)
	}

	// Kanso Duo endpoints
	duo := api.Group("/duo")
	duo.Get("/overview/:pairCode?", h.KansoHandler.GetOverview)
	duo.Post("/transactions", h.KansoHandler.CreateTransaction)
	duo.Delete("/transactions/bulk", h.KansoHandler.BulkDeleteTransactions)
	duo.Delete("/transactions/:id", h.KansoHandler.DeleteTransaction)
	duo.Post("/pods", h.KansoHandler.CreatePod)
	duo.Put("/pods/:id", h.KansoHandler.UpdatePod)
	duo.Delete("/pods/:id", h.KansoHandler.DeletePod)
	duo.Post("/pods/stash", h.KansoHandler.AllocateStash)
	duo.Post("/wishlist", h.KansoHandler.AddWishlist)
	duo.Post("/wishlist/:id/vote", h.KansoHandler.VoteWishlist)
	duo.Post("/love-tip", h.KansoHandler.SendLoveTip)
	duo.Post("/emergency-toggle", h.KansoHandler.ToggleEmergency)
	duo.Post("/sync/receipt", h.KansoHandler.SyncBankReceipt)
	duo.Post("/quests/claim", h.KansoHandler.ClaimDailyQuest)
	duo.Post("/auth/login", h.KansoHandler.LoginUser)
	duo.Get("/users/:id", h.KansoHandler.GetUserProfile)
	duo.Put("/users/:id", h.KansoHandler.UpdateUserProfile)

	// Google SSO & Gmail Sync endpoints
	if h.GoogleAuthHandler != nil {
		auth := duo.Group("/auth/google")
		auth.Get("/url", h.GoogleAuthHandler.GetAuthURL)
		auth.Get("/status", h.GoogleAuthHandler.GetStatus)
		auth.Get("/callback", h.GoogleAuthHandler.HandleCallback)
		auth.Post("/disconnect", h.GoogleAuthHandler.Disconnect)
		duo.Post("/sync/gmail", h.GoogleAuthHandler.ScanGmailMutations)
	}

	// Smart Email Mutation Webhook & Auto-Balance Reconcile endpoints
	if h.WebhookHandler != nil {
		duo.Post("/webhooks/email", h.WebhookHandler.HandleEmailWebhook)
		duo.Get("/mutations/logs", h.WebhookHandler.GetMutationLogs)
		duo.Post("/mutations/logs/:id/retry", h.WebhookHandler.RetryMutationLog)
		duo.Post("/pods/:id/reconcile", h.WebhookHandler.ReconcileWallet)
	}
}
