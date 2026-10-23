package handler

import (
	"fmt"
	"time"
	"kanso/backend/internal/service"

	"github.com/gofiber/fiber/v2"
)

type GoogleAuthHandler struct {
	ssoSvc      service.GoogleSSOService
	kansoSvc    service.KansoService
	mutationSvc service.MutationService
	frontendURL string
}

func NewGoogleAuthHandler(ssoSvc service.GoogleSSOService, kansoSvc service.KansoService, mutationSvc service.MutationService, frontendURL string) *GoogleAuthHandler {
	if frontendURL == "" {
		frontendURL = "http://localhost:5173"
	}
	return &GoogleAuthHandler{
		ssoSvc:      ssoSvc,
		kansoSvc:    kansoSvc,
		mutationSvc: mutationSvc,
		frontendURL: frontendURL,
	}
}

func (h *GoogleAuthHandler) GetStatus(c *fiber.Ctx) error {
	connected := h.ssoSvc.IsConnected()
	user := h.ssoSvc.GetConnectedUser()
	accounts := h.ssoSvc.GetConnectedAccounts()
	return c.JSON(fiber.Map{
		"connected": connected,
		"user":      user,
		"accounts":  accounts,
	})
}

func (h *GoogleAuthHandler) GetAuthURL(c *fiber.Ctx) error {
	url := h.ssoSvc.GetAuthURL()
	return c.JSON(fiber.Map{
		"url": url,
	})
}

func (h *GoogleAuthHandler) HandleCallback(c *fiber.Ctx) error {
	code := c.Query("code")
	if code == "" {
		errMsg := c.Query("error", "authorization code missing")
		return c.Redirect(fmt.Sprintf("%s/?auth_error=%s", h.frontendURL, errMsg))
	}

	user, err := h.ssoSvc.ExchangeCode(c.Context(), code)
	if err != nil {
		return c.Redirect(fmt.Sprintf("%s/?auth_error=%s", h.frontendURL, err.Error()))
	}

	return c.Redirect(fmt.Sprintf("%s/?google_connected=true&user_name=%s", h.frontendURL, user.Name))
}

func (h *GoogleAuthHandler) Disconnect(c *fiber.Ctx) error {
	email := c.Query("email")
	var err error
	if email != "" {
		err = h.ssoSvc.DisconnectAccount(c.Context(), email)
	} else {
		err = h.ssoSvc.Disconnect(c.Context())
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal memutuskan sesi Google: " + err.Error(),
		})
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Sesi login Google berhasil diputuskan.",
	})
}

func (h *GoogleAuthHandler) ScanGmailMutations(c *fiber.Ctx) error {
	if !h.ssoSvc.IsConnected() {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Akun Google belum terhubung. Silakan login terlebih dahulu.",
		})
	}

	days := c.QueryInt("days", 7)
	maxResults := c.QueryInt("limit", 20)
	afterParam := c.Query("after")
	beforeParam := c.Query("before")

	var dateRange service.EmailDateRange
	hasCustomRange := false

	if afterParam != "" {
		if t, err := time.Parse(time.RFC3339, afterParam); err == nil {
			dateRange.AfterDate = &t
			hasCustomRange = true
		} else if t, err := time.Parse("2006-01-02T15:04", afterParam); err == nil {
			dateRange.AfterDate = &t
			hasCustomRange = true
		} else if t, err := time.Parse("2006-01-02", afterParam); err == nil {
			dateRange.AfterDate = &t
			hasCustomRange = true
		}
	}

	if beforeParam != "" {
		if t, err := time.Parse(time.RFC3339, beforeParam); err == nil {
			dateRange.BeforeDate = &t
			hasCustomRange = true
		} else if t, err := time.Parse("2006-01-02T15:04", beforeParam); err == nil {
			dateRange.BeforeDate = &t
			hasCustomRange = true
		} else if t, err := time.Parse("2006-01-02", beforeParam); err == nil {
			// end of day
			tEnd := t.Add(24*time.Hour - time.Second)
			dateRange.BeforeDate = &tEnd
			hasCustomRange = true
		}
	}

	var emails []service.ParsedEmailMessage
	var err error
	accountEmail := c.Query("email")

	if accountEmail != "" && accountEmail != "all" {
		if hasCustomRange {
			emails, err = h.ssoSvc.FetchRecentMutationEmailsForAccount(c.Context(), accountEmail, days, maxResults, dateRange)
		} else {
			emails, err = h.ssoSvc.FetchRecentMutationEmailsForAccount(c.Context(), accountEmail, days, maxResults)
		}
	} else {
		// If "all" or not specified, fetch from all connected accounts
		connectedAccs := h.ssoSvc.GetConnectedAccounts()
		if len(connectedAccs) > 1 {
			seenIDs := make(map[string]bool)
			for _, acc := range connectedAccs {
				var batch []service.ParsedEmailMessage
				if hasCustomRange {
					batch, _ = h.ssoSvc.FetchRecentMutationEmailsForAccount(c.Context(), acc.Email, days, maxResults, dateRange)
				} else {
					batch, _ = h.ssoSvc.FetchRecentMutationEmailsForAccount(c.Context(), acc.Email, days, maxResults)
				}
				for _, m := range batch {
					if !seenIDs[m.ID] {
						seenIDs[m.ID] = true
						emails = append(emails, m)
					}
				}
			}
		} else {
			if hasCustomRange {
				emails, err = h.ssoSvc.FetchRecentMutationEmails(c.Context(), days, maxResults, dateRange)
			} else {
				emails, err = h.ssoSvc.FetchRecentMutationEmails(c.Context(), days, maxResults)
			}
		}
	}

	if err != nil && len(emails) == 0 {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	var recordedCount int
	var parsedItems []fiber.Map

	coupleID := c.Query("couple_id", "DUO-LOVE")
	userID := "eva"

	for _, em := range emails {
		combined := em.Subject + "\n" + em.Body

		var htmlPtr *string
		if em.HTMLBody != "" {
			htmlPtr = &em.HTMLBody
		}
		var accEmailPtr *string
		if em.AccountEmail != "" {
			accEmailPtr = &em.AccountEmail
		}

		// First priority: route through Smart Email Mutation Service (LLM-Powered)
		var txErr error
		tx, txErr := h.mutationSvc.ProcessInboundEmail(c.Context(), coupleID, userID, em.ID, em.Sender, combined, service.InboundEmailOptions{
			EmailDate:    &em.Date,
			HTMLBody:     htmlPtr,
			AccountEmail: accEmailPtr,
		})
		parserUsed := "gemini_llm"
		if txErr != nil {
			// Fallback: heuristic bank receipt regex parser with full email context
			senderPtr := &em.Sender
			refPtr := &em.ID
			tx, _ = h.kansoSvc.ProcessBankSync(coupleID, userID, combined, service.BankSyncOptions{
				EmailDate:     &em.Date,
				EmailHTMLBody: htmlPtr,
				SenderEmail:   senderPtr,
				RefID:         refPtr,
				EmailAccount:  accEmailPtr,
			})
			parserUsed = "regex_fallback"
		}

		if tx != nil {
			recordedCount++
			parsedItems = append(parsedItems, fiber.Map{
				"id":          tx.ID,
				"title":       tx.Title,
				"amount":      tx.Amount,
				"subject":     em.Subject,
				"parser_used": parserUsed,
				"created_at":  tx.CreatedAt,
			})
		}
	}

	return c.JSON(fiber.Map{
		"status":         "synced",
		"days_filter":    days,
		"emails_scanned": len(emails),
		"recorded_count": recordedCount,
		"transactions":   parsedItems,
	})
}
