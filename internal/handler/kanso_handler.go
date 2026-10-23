package handler

import (
	"fmt"
	"kanso/backend/internal/service"

	"github.com/gofiber/fiber/v2"
)

type KansoHandler struct {
	svc service.KansoService
}

func NewKansoHandler(svc service.KansoService) *KansoHandler {
	return &KansoHandler{svc: svc}
}

func (h *KansoHandler) GetOverview(c *fiber.Ctx) error {
	pairCode := c.Params("pairCode")
	if pairCode == "" {
		pairCode = "DUO-LOVE"
	}

	overview, err := h.svc.GetOverview(pairCode)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Couple data not found",
		})
	}

	return c.JSON(overview)
}

func (h *KansoHandler) CreateTransaction(c *fiber.Ctx) error {
	var req struct {
		CoupleID      string  `json:"couple_id"`
		UserID        string  `json:"user_id"`
		Type          string  `json:"type"`
		Amount        float64 `json:"amount"`
		Title         string  `json:"title"`
		CuteTitle     string  `json:"cute_title"`
		NoteEmotional string  `json:"note_emotional"`
		CuteNote      string  `json:"cute_note"`
		MoodTag       string  `json:"mood_tag"`
		Icon          string  `json:"icon"`
		CuteIcon      string  `json:"cute_icon"`
		PodID         *string `json:"pod_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	tx, err := h.svc.LogTransaction(
		req.CoupleID, req.UserID, req.Type, req.Amount,
		req.Title, req.CuteTitle, req.NoteEmotional, req.CuteNote,
		req.MoodTag, req.Icon, req.CuteIcon, req.PodID,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(tx)
}

func (h *KansoHandler) DeleteTransaction(c *fiber.Ctx) error {
	txID := c.Params("id")
	if txID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Transaction ID is required"})
	}

	if err := h.svc.DeleteTransaction(txID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Transaksi berhasil dihapus dan saldo pod disesuaikan kembali",
	})
}

func (h *KansoHandler) BulkDeleteTransactions(c *fiber.Ctx) error {
	var req struct {
		IDs []string `json:"ids"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Format request tidak valid: " + err.Error()})
	}

	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Pilih minimal 1 transaksi untuk dihapus"})
	}

	deletedCount, err := h.svc.BulkDeleteTransactions(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"deleted_count": deletedCount,
		"message":       fmt.Sprintf("%d transaksi berhasil dihapus", deletedCount),
	})
}

func (h *KansoHandler) AllocateStash(c *fiber.Ctx) error {
	var req struct {
		CoupleID string  `json:"couple_id"`
		PodID    string  `json:"pod_id"`
		Amount   float64 `json:"amount"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.svc.AllocateStash(req.CoupleID, req.PodID, req.Amount); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "stashed", "amount": req.Amount})
}

func (h *KansoHandler) AddWishlist(c *fiber.Ctx) error {
	var req struct {
		CoupleID string  `json:"couple_id"`
		UserID   string  `json:"user_id"`
		Title    string  `json:"title"`
		Category string  `json:"category"`
		Price    float64 `json:"price"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	item, err := h.svc.AddWishlistItem(req.CoupleID, req.UserID, req.Title, req.Category, req.Price)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(item)
}

func (h *KansoHandler) VoteWishlist(c *fiber.Ctx) error {
	itemID := c.Params("id")
	var req struct {
		PartnerRole string `json:"partner_role"` // partner_a, partner_b
		Vote        string `json:"vote"`         // need, delay, later
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.svc.VoteWishlist(itemID, req.PartnerRole, req.Vote); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "voted", "vote": req.Vote})
}

func (h *KansoHandler) SendLoveTip(c *fiber.Ctx) error {
	var req struct {
		CoupleID    string  `json:"couple_id"`
		FromUser    string  `json:"from_user"`
		ToUser      string  `json:"to_user"`
		StickerNote string  `json:"sticker_note"`
		Amount      float64 `json:"amount"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	tip, err := h.svc.SendLoveTip(req.CoupleID, req.FromUser, req.ToUser, req.StickerNote, req.Amount)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":      "love_tip_sent",
		"tip":         tip,
		"xp_rewarded": 50,
	})
}

func (h *KansoHandler) CreatePod(c *fiber.Ctx) error {
	var req struct {
		CoupleID            string  `json:"couple_id"`
		NameZen             string  `json:"name_zen"`
		NameKawaii          string  `json:"name_kawaii"`
		TargetAmount        float64 `json:"target_amount"`
		ColorClass          string  `json:"color_class"`
		LiquidBgClass       string  `json:"liquid_bg_class"`
		ProviderCode        *string `json:"provider_code"`
		AccountNumberSuffix *string `json:"account_number_suffix"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	pod, err := h.svc.CreatePod(req.CoupleID, req.NameZen, req.NameKawaii, req.TargetAmount, req.ColorClass, req.LiquidBgClass, req.ProviderCode, req.AccountNumberSuffix)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(pod)
}

func (h *KansoHandler) UpdatePod(c *fiber.Ctx) error {
	podID := c.Params("id")
	var req struct {
		NameZen             string  `json:"name_zen"`
		NameKawaii          string  `json:"name_kawaii"`
		TargetAmount        float64 `json:"target_amount"`
		ColorClass          string  `json:"color_class"`
		LiquidBgClass       string  `json:"liquid_bg_class"`
		ProviderCode        *string `json:"provider_code"`
		AccountNumberSuffix *string `json:"account_number_suffix"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	pod, err := h.svc.UpdatePod(podID, req.NameZen, req.NameKawaii, req.TargetAmount, req.ColorClass, req.LiquidBgClass, req.ProviderCode, req.AccountNumberSuffix)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(pod)
}

func (h *KansoHandler) DeletePod(c *fiber.Ctx) error {
	podID := c.Params("id")
	if err := h.svc.DeletePod(podID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"status": "deleted", "id": podID})
}

func (h *KansoHandler) SyncBankReceipt(c *fiber.Ctx) error {
	var req struct {
		CoupleID string `json:"couple_id"`
		UserID   string `json:"user_id"`
		RawText  string `json:"raw_text"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	tx, err := h.svc.ProcessBankSync(req.CoupleID, req.UserID, req.RawText)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(tx)
}

func (h *KansoHandler) ClaimDailyQuest(c *fiber.Ctx) error {
	var req struct {
		CoupleID string `json:"couple_id"`
		QuestKey string `json:"quest_key"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	xp, err := h.svc.ClaimQuest(req.CoupleID, req.QuestKey)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "quest_claimed", "xp_earned": xp})
}

func (h *KansoHandler) ToggleEmergency(c *fiber.Ctx) error {
	var req struct {
		CoupleID string `json:"couple_id"`
		Enable   bool   `json:"enable"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.svc.ToggleEmergencyProtocol(req.CoupleID, req.Enable); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status":         "emergency_toggled",
		"emergency_mode": req.Enable,
	})
}

func (h *KansoHandler) LoginUser(c *fiber.Ctx) error {
	var req struct {
		Identifier string `json:"identifier"` // email, user_id, or role (dimas, eva)
	}

	if err := c.BodyParser(&req); err != nil || req.Identifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Identitas login (email/role) diperlukan"})
	}

	user, err := h.svc.LoginUser(req.Identifier)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Pengguna tidak ditemukan: " + err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"user":    user,
	})
}

func (h *KansoHandler) GetUserProfile(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID diperlukan"})
	}

	user, err := h.svc.GetUser(userID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(user)
}

func (h *KansoHandler) UpdateUserProfile(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID diperlukan"})
	}

	var req struct {
		DisplayName     string `json:"display_name"`
		ThemePreference string `json:"theme_preference"`
		SoundEnabled    bool   `json:"sound_enabled"`
		AvatarURL       string `json:"avatar_url"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	updated, err := h.svc.UpdateUserProfile(userID, req.DisplayName, req.ThemePreference, req.SoundEnabled, req.AvatarURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"user":    updated,
	})
}
