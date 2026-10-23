package service

import (
	"fmt"
	"math"
	"time"
	"kanso/backend/internal/infrastructure/parser"
	"kanso/backend/internal/model"
	"kanso/backend/internal/repository"

	"github.com/google/uuid"
)

type DuoOverview struct {
	Couple       *model.HouseholdCouple `json:"couple"`
	Users        []model.User           `json:"users"`
	Pods         []model.PocketPod      `json:"pods"`
	Transactions []model.Transaction    `json:"transactions"`
	Wishlist     []model.WishlistItem   `json:"wishlist"`
	MainBalance  float64                `json:"main_balance"`
	BurnPace     float64                `json:"burn_pace"`
	DailyQuests  []model.DailyQuest     `json:"daily_quests"`
}

type KansoService interface {
	GetOverview(pairCode string) (*DuoOverview, error)
	LogTransaction(coupleID, userID string, txType string, amount float64, title, cuteTitle, note, cuteNote, mood, icon, cuteIcon string, podID *string) (*model.Transaction, error)
	DeleteTransaction(txID string) error
	BulkDeleteTransactions(txIDs []string) (int64, error)
	AllocateStash(coupleID string, podID string, amount float64) error
	CreatePod(coupleID string, nameZen, nameKawaii string, targetAmount float64, colorClass, liquidBgClass string, providerCode, accountNumberSuffix *string) (*model.PocketPod, error)
	UpdatePod(podID string, nameZen, nameKawaii string, targetAmount float64, colorClass, liquidBgClass string, providerCode, accountNumberSuffix *string) (*model.PocketPod, error)
	DeletePod(podID string) error
	AddWishlistItem(coupleID, userID, title, category string, price float64) (*model.WishlistItem, error)
	VoteWishlist(itemID, partnerRole, vote string) error
	SendLoveTip(coupleID, fromUser, toUser, stickerNote string, amount float64) (*model.LoveTip, error)
	ToggleEmergencyProtocol(coupleID string, enable bool) error
	ProcessBankSync(coupleID, userID, rawText string, options ...BankSyncOptions) (*model.Transaction, error)
	ClaimQuest(coupleID string, questKey string) (int, error)
	GetUser(userID string) (*model.User, error)
	LoginUser(emailOrRole string) (*model.User, error)
	UpdateUserProfile(userID string, displayName, themePreference string, soundEnabled bool, avatarURL string) (*model.User, error)
}

type kansoService struct {
	repo repository.KansoRepository
}

func NewKansoService(repo repository.KansoRepository) KansoService {
	return &kansoService{repo: repo}
}

func (s *kansoService) GetOverview(pairCode string) (*DuoOverview, error) {
	couple, err := s.repo.GetCoupleByPairCode(pairCode)
	if err != nil {
		return nil, err
	}

	users, err := s.repo.GetUsersByCoupleID(couple.ID)
	if err != nil {
		return nil, err
	}

	pods, err := s.repo.GetPodsByCoupleID(couple.ID)
	if err != nil {
		return nil, err
	}

	txs, err := s.repo.GetTransactionsByCoupleID(couple.ID, 20)
	if err != nil {
		return nil, err
	}

	wishlist, err := s.repo.GetWishlistItems(couple.ID)
	if err != nil {
		return nil, err
	}

	// Total liquid cash is calculated as the sum of all actual pod balances
	var totalPodBalances float64
	for _, p := range pods {
		totalPodBalances += p.CurrentAmount
	}
	mainBalance := totalPodBalances

	// If no pods configured yet or pod balances are unassigned, calculate net balance directly from transactions
	if len(pods) == 0 {
		var netTxBalance float64
		for _, t := range txs {
			if t.Direction == "IN" || t.Type == "INCOME" || t.Type == "INCOME_SALARY" || t.Type == "INCOME_PRELOVED" {
				netTxBalance += t.Amount
			} else {
				netTxBalance -= (t.Amount + t.AdminFee)
			}
		}
		mainBalance = netTxBalance
	}

	// Calculate Level from HarmonyXP
	level := 1
	if couple.HarmonyXP >= 600 {
		level = 3 // Golden Spirit
	} else if couple.HarmonyXP >= 250 {
		level = 2 // Cozy Bunny
	}
	couple.PetLevel = level

	// Daily Quests list
	quests := []model.DailyQuest{
		{
			ID:          "q-1",
			Key:         "check_journal",
			Title:       "Cek Jurnal Keuangan",
			CuteTitle:   "Sapa Kiko & Cek Pengeluaran 📖",
			Description: "Buka dan tinjau pengeluaran bersama hari ini",
			RewardXP:    20,
			Completed:   len(txs) > 0,
			Icon:        "📖",
		},
		{
			ID:          "q-2",
			Key:         "send_love_tip",
			Title:       "Kirim Love Tap Pasangan",
			CuteTitle:   "Apresiasi Manis Pasangan 💖",
			Description: "Kirim tip cinta kejutan Rp 10.000 / Rp 25.000",
			RewardXP:    50,
			Completed:   couple.HarmonyXP >= 300,
			Icon:        "💖",
		},
		{
			ID:          "q-3",
			Key:         "stash_savings",
			Title:       "Nabung Santai ke Pod",
			CuteTitle:   "Alokasi Kas Sayang ke Kantong 🧸",
			Description: "Sisihkan sisa jatah harian ke kantong tabungan impian",
			RewardXP:    30,
			Completed:   len(pods) > 0,
			Icon:        "🌱",
		},
	}

	return &DuoOverview{
		Couple:       couple,
		Users:        users,
		Pods:         pods,
		Transactions: txs,
		Wishlist:     wishlist,
		MainBalance:  mainBalance,
		BurnPace:     75000.0,
		DailyQuests:  quests,
	}, nil
}

func (s *kansoService) LogTransaction(coupleID, userID string, txType string, amount float64, title, cuteTitle, note, cuteNote, mood, icon, cuteIcon string, podID *string) (*model.Transaction, error) {
	resolvedCoupleID := s.resolveCoupleID(coupleID)

	tx := &model.Transaction{
		ID:              uuid.NewString(),
		CoupleID:        resolvedCoupleID,
		CreatedByUserID: userID,
		PodID:           podID,
		Type:            txType,
		Amount:          amount,
		Title:           title,
		CuteTitle:       cuteTitle,
		NoteEmotional:   note,
		CuteNote:        cuteNote,
		MoodTag:         mood,
		Icon:            icon,
		CuteIcon:        cuteIcon,
		CreatedAt:       time.Now(),
	}

	if err := s.repo.CreateTransaction(tx); err != nil {
		return nil, err
	}

	// If linked to a pod and is expense/stash, adjust balance
	if podID != nil && *podID != "" {
		if txType == "INCOME" || txType == "INCOME_SALARY" || txType == "INCOME_PRELOVED" {
			_ = s.repo.UpdatePodBalance(*podID, amount)
		} else {
			_ = s.repo.UpdatePodBalance(*podID, -amount)
		}
	}

	_ = s.repo.AddHarmonyXP(resolvedCoupleID, 15)

	return tx, nil
}

func (s *kansoService) DeleteTransaction(txID string) error {
	return s.repo.DeleteTransaction(txID)
}

func (s *kansoService) BulkDeleteTransactions(txIDs []string) (int64, error) {
	return s.repo.BulkDeleteTransactions(txIDs)
}

func (s *kansoService) AllocateStash(coupleID string, podID string, amount float64) error {
	return s.repo.UpdatePodBalance(podID, amount)
}

func (s *kansoService) resolveCoupleID(codeOrID string) string {
	if c, err := s.repo.GetCoupleByPairCode(codeOrID); err == nil && c != nil {
		return c.ID
	}
	return codeOrID
}

func (s *kansoService) CreatePod(coupleID string, nameZen, nameKawaii string, targetAmount float64, colorClass, liquidBgClass string, providerCode, accountNumberSuffix *string) (*model.PocketPod, error) {
	resolvedID := s.resolveCoupleID(coupleID)

	if colorClass == "" {
		colorClass = "bg-[#F6E7B8]/70"
	}
	if liquidBgClass == "" {
		liquidBgClass = "bg-[#F6E7B8] text-[#F6E7B8]"
	}

	pods, _ := s.repo.GetPodsByCoupleID(resolvedID)
	badgeNum := fmt.Sprintf("%02d", len(pods)+1)

	pod := &model.PocketPod{
		ID:                  uuid.NewString(),
		CoupleID:            resolvedID,
		BadgeNumber:         badgeNum,
		NameZen:             nameZen,
		NameKawaii:          nameKawaii,
		TargetAmount:        targetAmount,
		CurrentAmount:       0,
		CategoryType:        "shared",
		ColorClass:          colorClass,
		LiquidBgClass:       liquidBgClass,
		HasDuck:             false,
		ProviderCode:        providerCode,
		AccountNumberSuffix: accountNumberSuffix,
		CreatedAt:           time.Now(),
	}

	if err := s.repo.CreatePod(pod); err != nil {
		return nil, err
	}
	_ = s.repo.AddHarmonyXP(resolvedID, 25)
	return pod, nil
}

func (s *kansoService) UpdatePod(podID string, nameZen, nameKawaii string, targetAmount float64, colorClass, liquidBgClass string, providerCode, accountNumberSuffix *string) (*model.PocketPod, error) {
	pod, err := s.repo.GetPodByID(podID)
	if err != nil {
		return nil, err
	}

	if nameZen != "" {
		pod.NameZen = nameZen
	}
	if nameKawaii != "" {
		pod.NameKawaii = nameKawaii
	}
	if targetAmount > 0 {
		pod.TargetAmount = targetAmount
	}
	if colorClass != "" {
		pod.ColorClass = colorClass
	}
	if liquidBgClass != "" {
		pod.LiquidBgClass = liquidBgClass
	}
	if providerCode != nil {
		pod.ProviderCode = providerCode
	}
	if accountNumberSuffix != nil {
		pod.AccountNumberSuffix = accountNumberSuffix
	}

	if err := s.repo.UpdatePod(pod); err != nil {
		return nil, err
	}
	return pod, nil
}

func (s *kansoService) DeletePod(podID string) error {
	return s.repo.DeletePod(podID)
}

type BankSyncOptions struct {
	EmailDate     *time.Time
	EmailHTMLBody *string
	SenderEmail   *string
	RefID         *string
	EmailAccount  *string
}

func (s *kansoService) ProcessBankSync(coupleID, userID, rawText string, options ...BankSyncOptions) (*model.Transaction, error) {
	resolvedID := s.resolveCoupleID(coupleID)
	parsed := parser.ParseBankReceipt(rawText)
	if parsed.Amount <= 0 {
		return nil, fmt.Errorf("tidak dapat mendeteksi nominal transaksi dari notifikasi")
	}

	// Match pod based on suggestion
	pods, _ := s.repo.GetPodsByCoupleID(resolvedID)
	var matchedPodID *string
	for _, p := range pods {
		if p.BadgeNumber == parsed.SuggestedPod {
			matchedPodID = &p.ID
			break
		}
	}
	if matchedPodID == nil && len(pods) > 0 {
		matchedPodID = &pods[0].ID
	}

	now := time.Now()
	var effectiveDate time.Time = now
	var htmlBody *string
	var senderEmail *string
	var refID *string
	var emailAccount *string

	if len(options) > 0 {
		opt := options[0]
		if opt.EmailDate != nil && !opt.EmailDate.IsZero() {
			effectiveDate = *opt.EmailDate
		}
		if opt.EmailHTMLBody != nil && *opt.EmailHTMLBody != "" {
			htmlBody = opt.EmailHTMLBody
		}
		if opt.SenderEmail != nil && *opt.SenderEmail != "" {
			senderEmail = opt.SenderEmail
		}
		if opt.RefID != nil && *opt.RefID != "" {
			refID = opt.RefID
		}
		if opt.EmailAccount != nil && *opt.EmailAccount != "" {
			emailAccount = opt.EmailAccount
		}
	}

	sender := fmt.Sprintf("%s Notifikasi", parsed.BankOrSource)
	if senderEmail != nil {
		sender = *senderEmail
	}
	snippet := rawText
	if len(snippet) > 1000 {
		snippet = snippet[:1000] + "..."
	}
	rawCopy := rawText

	tx := &model.Transaction{
		ID:               uuid.NewString(),
		CoupleID:         resolvedID,
		CreatedByUserID:  userID,
		PodID:            matchedPodID,
		Type:             "EXPENSE",
		Amount:           parsed.Amount,
		Title:            parsed.Description,
		CuteTitle:        parsed.Merchant + " ✨",
		NoteEmotional:    fmt.Sprintf("Tersinkronisasi otomatis dari %s", parsed.BankOrSource),
		CuteNote:         fmt.Sprintf("Catatan otomatis dari %s 🌸", parsed.BankOrSource),
		MoodTag:          "calm",
		Icon:             "⚡",
		CuteIcon:         "🌸",
		SourceType:       "EMAIL_SYNC",
		ExternalRefID:    refID,
		SyncStatus:       "VERIFIED",
		EmailSender:      &sender,
		EmailBodySnippet: &snippet,
		EmailRawBody:     &rawCopy,
		EmailHTMLBody:    htmlBody,
		EmailAccount:     emailAccount,
		EmailDate:        &effectiveDate,
		CreatedAt:        effectiveDate,
	}

	if err := s.repo.CreateTransaction(tx); err != nil {
		return nil, err
	}

	// If pod matched, update balance
	if matchedPodID != nil {
		_ = s.repo.UpdatePodBalance(*matchedPodID, -parsed.Amount)
	}

	// Reward XP for honest bank sync
	_ = s.repo.AddHarmonyXP(resolvedID, 20)
	return tx, nil
}

func (s *kansoService) ClaimQuest(coupleID string, questKey string) (int, error) {
	resolvedID := s.resolveCoupleID(coupleID)
	xp := 20
	if questKey == "send_love_tip" {
		xp = 50
	} else if questKey == "stash_savings" {
		xp = 30
	}
	err := s.repo.AddHarmonyXP(resolvedID, xp)
	return xp, err
}

func (s *kansoService) AddWishlistItem(coupleID, userID, title, category string, price float64) (*model.WishlistItem, error) {
	// Emotional Relative Calculator (PRD 5.1):
	// Rata-rata belanja bahan sayur & buah harian keluarga di Indonesia (~Rp 35.000 / hari)
	groceryDays := int(math.Round(price / 35000.0))
	if groceryDays < 1 {
		groceryDays = 1
	}
	emotionalEquiv := fmt.Sprintf("Setara dengan %d hari jatah belanja bahan segar keluarga 🥦", groceryDays)

	item := &model.WishlistItem{
		ID:                  uuid.NewString(),
		CoupleID:            coupleID,
		CreatedByUserID:     userID,
		Title:               title,
		Price:               price,
		Category:            category,
		EmotionalEquivalent: emotionalEquiv,
		VotePartnerA:        "pending",
		VotePartnerB:        "pending",
		CooldownEndsAt:      time.Now().Add(7 * 24 * time.Hour), // 7-day cooldown
		Status:              "in_cooldown",
		CreatedAt:           time.Now(),
	}

	if err := s.repo.CreateWishlistItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *kansoService) VoteWishlist(itemID, partnerRole, vote string) error {
	return s.repo.VoteWishlistItem(itemID, partnerRole, vote)
}

func (s *kansoService) SendLoveTip(coupleID, fromUser, toUser, stickerNote string, amount float64) (*model.LoveTip, error) {
	tip := &model.LoveTip{
		ID:          uuid.NewString(),
		CoupleID:    coupleID,
		FromUserID:  fromUser,
		ToUserID:    toUser,
		Amount:      amount,
		StickerNote: stickerNote,
		CreatedAt:   time.Now(),
	}

	if err := s.repo.RecordLoveTip(tip); err != nil {
		return nil, err
	}
	return tip, nil
}

func (s *kansoService) ToggleEmergencyProtocol(coupleID string, enable bool) error {
	return s.repo.ToggleEmergencyMode(coupleID, enable)
}

func (s *kansoService) GetUser(userID string) (*model.User, error) {
	// Support lookup by ID, role ('dimas', 'eva'), or email
	if userID == "dimas" || userID == "partner_a" {
		return s.repo.GetUserByEmail("dimas@kanso.love")
	}
	if userID == "eva" || userID == "partner_b" {
		return s.repo.GetUserByEmail("eva@kanso.love")
	}
	user, err := s.repo.GetUserByID(userID)
	if err == nil && user != nil {
		return user, nil
	}
	return s.repo.GetUserByEmail(userID)
}

func (s *kansoService) LoginUser(emailOrRole string) (*model.User, error) {
	return s.GetUser(emailOrRole)
}

func (s *kansoService) UpdateUserProfile(userID string, displayName, themePreference string, soundEnabled bool, avatarURL string) (*model.User, error) {
	user, err := s.GetUser(userID)
	if err != nil {
		return nil, err
	}

	if displayName != "" {
		user.DisplayName = displayName
	}
	if themePreference == "zen" || themePreference == "kawaii" {
		user.ThemePreference = themePreference
	}
	user.SoundEnabled = soundEnabled
	if avatarURL != "" {
		user.AvatarURL = avatarURL
	}

	if err := s.repo.UpdateUser(user); err != nil {
		return nil, err
	}
	return user, nil
}
