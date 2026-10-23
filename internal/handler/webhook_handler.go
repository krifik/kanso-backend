package handler

import (
	"kanso/backend/internal/service"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type WebhookHandler struct {
	mutationService service.MutationService
	webhookSecret   string
}

func NewWebhookHandler(mutationService service.MutationService, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{
		mutationService: mutationService,
		webhookSecret:   webhookSecret,
	}
}

// InboundEmailPayload represents the JSON payload from email forwarding services (e.g. Cloudflare Email Workers, SendGrid, Postmark)
type InboundEmailPayload struct {
	CoupleID    string `json:"couple_id"`
	UserID      string `json:"user_id"`
	MessageID   string `json:"message_id"`
	SenderEmail string `json:"sender_email"`
	RawBody     string `json:"raw_body"`
}

// HandleEmailWebhook processes inbound bank mutation emails
func (h *WebhookHandler) HandleEmailWebhook(c *fiber.Ctx) error {
	// 1. Verify Secret Header if secret is set
	if h.webhookSecret != "" {
		headerSecret := c.Get("X-Webhook-Secret")
		if headerSecret == "" {
			// Also check query param or Authorization header
			headerSecret = c.Query("secret")
		}
		if headerSecret != h.webhookSecret {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid or missing X-Webhook-Secret header",
			})
		}
	}

	var req InboundEmailPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body: " + err.Error(),
		})
	}

	if req.MessageID == "" || req.SenderEmail == "" || req.RawBody == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "message_id, sender_email, and raw_body are required fields",
		})
	}

	// Default couple_id and user_id if not explicitly sent in webhook payload
	if req.CoupleID == "" {
		req.CoupleID = "cpl-duo-01"
	}
	if req.UserID == "" {
		req.UserID = "usr-partner-a"
	}

	tx, err := h.mutationService.ProcessInboundEmail(
		c.Context(),
		req.CoupleID,
		req.UserID,
		req.MessageID,
		req.SenderEmail,
		req.RawBody,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate message_id") {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		}
		if strings.Contains(err.Error(), "untrusted sender") {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	if tx == nil {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"success": true,
			"message": "Email received and acknowledged as non-transactional",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Transaction extracted and synced successfully",
		"data":    tx,
	})
}

// GetMutationLogs returns recent email mutation logs for audit/review
func (h *WebhookHandler) GetMutationLogs(c *fiber.Ctx) error {
	coupleID := c.Query("couple_id", "cpl-duo-01")
	logs, err := h.mutationService.GetMutationLogs(coupleID, 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    logs,
	})
}

// RetryMutationLog attempts to re-parse a failed or reviewable mutation log
func (h *WebhookHandler) RetryMutationLog(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Log ID is required",
		})
	}

	tx, err := h.mutationService.RetryMutationLog(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Mutation log reprocessed successfully",
		"data":    tx,
	})
}

// ReconcileWalletPayload represents the payload to sync actual bank balance
type ReconcileWalletPayload struct {
	ActualBalance float64 `json:"actual_balance"`
	CoupleID      string  `json:"couple_id"`
	UserID        string  `json:"user_id"`
}

// ReconcileWallet handles wallet auto-balancing against ground truth
func (h *WebhookHandler) ReconcileWallet(c *fiber.Ctx) error {
	podID := c.Params("id")
	if podID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Pocket Pod / Wallet ID is required",
		})
	}

	var req ReconcileWalletPayload
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body: " + err.Error(),
		})
	}

	if req.CoupleID == "" {
		req.CoupleID = "cpl-duo-01"
	}
	if req.UserID == "" {
		req.UserID = "usr-partner-a"
	}

	tx, err := h.mutationService.ReconcilePod(req.CoupleID, req.UserID, podID, req.ActualBalance)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Wallet reconciled successfully",
		"data":    tx,
	})
}
