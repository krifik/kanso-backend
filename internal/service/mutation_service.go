package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"kanso/backend/internal/model"
	"kanso/backend/internal/repository"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

var allowedSenderDomains = []string{
	// Bank Konvensional & Syariah Populer Indonesia
	"klikbca.com",
	"bca.co.id",
	"bankmandiri.co.id",
	"mandiri.co.id",
	"livin.id",
	"bri.co.id",
	"bni.co.id",
	"bankbsi.co.id",
	"bsi.co.id",
	"cimbniaga.co.id",
	"permatabank.co.id",
	"permatabank.com",
	"permata.co.id",
	"permatanet.com",
	"danamon.co.id",
	"panin.co.id",
	"btn.co.id",
	"bankmega.com",
	"ocbc.id",
	"ocbcnisp.com",
	"maybank.co.id",
	"sinarmas.co.id",
	"bankbtpn.co.id",
	"uob.co.id",
	"dbs.com",
	"hsbc.co.id",
	"standardchartered.co.id",
	"shinhan.co.id",
	"woorisaudara.com",
	"mncbank.co.id",
	"banknobu.com",
	"bankmuamalat.co.id",
	"bankraya.co.id",
	"bjb.co.id",
	"bankjatim.co.id",
	"bankjateng.co.id",
	"bankdki.co.id",
	// Bank Digital Indonesia
	"jago.com",
	"bankjago.com",
	"seabank.co.id",
	"jenius.com",
	"blubybcadigital.id",
	"bcadigital.co.id",
	"aladinbank.id",
	"krom.id",
	"neobank.co.id",
	"bancneocommerce.co.id",
	"bankneo.co.id",
	"superbank.id",
	"hibank.co.id",
	"digibank.com",
	"tyme.com",
	// E-Wallet & Payment Gateway Indonesia
	"gopay.co.id",
	"gojek.com",
	"ovo.id",
	"dana.id",
	"dana.co.id",
	"shopee.co.id",
	"shopeepay.co.id",
	"linkaja.id",
	"astrapay.co.id",
	"isaku.co.id",
	"doku.com",
	"midtrans.com",
	"xendit.co",
	"flip.id",
	"oyindonesia.com",
	"plink.co.id",
}

type InboundEmailOptions struct {
	EmailDate    *time.Time
	HTMLBody     *string
	AccountEmail *string
}

type MutationService interface {
	ProcessInboundEmail(ctx context.Context, coupleID, userID, messageID, senderEmail, rawBody string, opts ...InboundEmailOptions) (*model.Transaction, error)
	GetMutationLogs(coupleID string, limit int) ([]model.EmailMutationLog, error)
	RetryMutationLog(ctx context.Context, logID string) (*model.Transaction, error)
	ReconcilePod(coupleID, userID, podID string, actualBalance float64) (*model.Transaction, error)
}

type mutationService struct {
	repo         repository.MutationRepository
	kansoRepo    repository.KansoRepository
	llmParser    LLMParserService
}

func NewMutationService(
	repo repository.MutationRepository,
	kansoRepo repository.KansoRepository,
	llmParser LLMParserService,
) MutationService {
	return &mutationService{
		repo:      repo,
		kansoRepo: kansoRepo,
		llmParser: llmParser,
	}
}

func (s *mutationService) ProcessInboundEmail(
	ctx context.Context,
	coupleID, userID, messageID, senderEmail, rawBody string,
	opts ...InboundEmailOptions,
) (*model.Transaction, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, errors.New("message_id is required")
	}

	var emailDate *time.Time
	var rawHTML *string
	var accountEmail *string

	if len(opts) > 0 {
		opt := opts[0]
		emailDate = opt.EmailDate
		rawHTML = opt.HTMLBody
		accountEmail = opt.AccountEmail
	}

	// 1. Check idempotency on message_id
	existingLog, err := s.repo.GetEmailMutationLogByMessageID(messageID)
	if err != nil {
		return nil, fmt.Errorf("error checking existing message_id: %w", err)
	}
	if existingLog != nil {
		return nil, fmt.Errorf("duplicate message_id: email mutation has already been processed with status %s", existingLog.Status)
	}

	// 2. Validate sender domain against trusted Indonesian banks and e-wallets
	senderDomainAllowed := false
	lowerSender := strings.ToLower(senderEmail)
	for _, domain := range allowedSenderDomains {
		if strings.HasSuffix(lowerSender, "@"+domain) || strings.Contains(lowerSender, "@"+domain+">") || strings.Contains(lowerSender, domain) {
			senderDomainAllowed = true
			break
		}
	}

	// 3. Sanitize email body (prioritize full HTML body if plain text is empty/incomplete snippet)
	contentToSanitize := rawBody
	if rawHTML != nil && len(*rawHTML) > len(rawBody) {
		contentToSanitize = *rawHTML
	}
	sanitizedBody := SanitizeEmailBody(contentToSanitize)
	if strings.TrimSpace(sanitizedBody) == "" {
		sanitizedBody = SanitizeEmailBody(rawBody)
	}

	rawBodyCopy := rawBody
	logRecord := &model.EmailMutationLog{
		ID:            uuid.New().String(),
		CoupleID:      coupleID,
		UserID:        userID,
		MessageID:     messageID,
		SenderEmail:   senderEmail,
		CleanBodyText: sanitizedBody,
		RawBodyText:   &rawBodyCopy,
		RawHTMLBody:   rawHTML,
		CreatedAt:     time.Now(),
	}

	if !senderDomainAllowed {
		errMsg := fmt.Sprintf("sender %s is not in trusted financial domain list", senderEmail)
		logRecord.Status = "IGNORED_NON_TX"
		logRecord.ErrorMessage = &errMsg
		_ = s.repo.CreateEmailMutationLog(logRecord)
		return nil, fmt.Errorf("untrusted sender domain: %s", senderEmail)
	}

	// 4. Invoke LLM Structured Output Parser
	parsedResult, rawLLMResponse, err := s.llmParser.ParseEmailMutation(ctx, sanitizedBody)
	logRecord.LLMRawResponse = rawLLMResponse

	if err != nil {
		errMsg := err.Error()
		logRecord.Status = "FAILED"
		logRecord.ErrorMessage = &errMsg
		_ = s.repo.CreateEmailMutationLog(logRecord)
		return nil, fmt.Errorf("llm parsing error: %w", err)
	}

	if !parsedResult.IsTransaction {
		logRecord.Status = "IGNORED_NON_TX"
		_ = s.repo.CreateEmailMutationLog(logRecord)
		return nil, nil // Non-transaction email, acknowledged and ignored cleanly
	}

	// 5. Match target Pocket Pod if available, but do not require it
	var providerCode, accountSuffix string
	if parsedResult.ProviderCode != nil {
		providerCode = *parsedResult.ProviderCode
	}
	if parsedResult.AccountSuffix != nil {
		accountSuffix = *parsedResult.AccountSuffix
	}

	var podID *string
	syncStatus := "VERIFIED"

	targetPod, _ := s.repo.FindPodByProviderAndSuffix(coupleID, providerCode, accountSuffix)
	if targetPod != nil {
		pID := targetPod.ID
		podID = &pID
	}

	// 6. Compute direction & balance delta
	direction := "OUT"
	if parsedResult.Direction != nil && (*parsedResult.Direction == "IN" || *parsedResult.Direction == "INTERNAL_TRANSFER") {
		direction = *parsedResult.Direction
	}

	amount := 0.0
	if parsedResult.Amount != nil {
		amount = *parsedResult.Amount
	}

	adminFee := 0.0
	if parsedResult.AdminFee != nil {
		adminFee = *parsedResult.AdminFee
	}

	var deltaBalance float64
	if podID != nil {
		if direction == "IN" {
			deltaBalance = amount
		} else {
			// OUT: subtract principal amount + admin fee
			deltaBalance = -(amount + adminFee)
		}
	}

	// 7. Prepare external reference ID (hash of messageID)
	hasher := sha256.New()
	hasher.Write([]byte(messageID))
	hashHex := hex.EncodeToString(hasher.Sum(nil))
	externalRefID := fmt.Sprintf("EMAIL-%s", hashHex[:24])

	title := "Bank Mutation"
	if parsedResult.MerchantOrCounterparty != nil && *parsedResult.MerchantOrCounterparty != "" {
		title = *parsedResult.MerchantOrCounterparty
	} else if parsedResult.SuggestedCategory != nil && *parsedResult.SuggestedCategory != "" {
		title = *parsedResult.SuggestedCategory
	}

	txType := "EXPENSE"
	if direction == "IN" {
		txType = "INCOME"
	}

	// Determine accurate transaction date
	effectiveDate := time.Now()
	if emailDate != nil && !emailDate.IsZero() {
		effectiveDate = *emailDate
	}
	// If LLM extracted a more specific transaction time from email body, prioritize it
	if parsedResult.TransactionTime != nil && strings.TrimSpace(*parsedResult.TransactionTime) != "" {
		timeFormats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05-07:00",
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04",
			"02 Jan 2006, 15:04 WIB",
			"02 January 2006, 15:04 WIB",
			"2 January 2006, 15:04 WIB",
		}
		cleanTimeStr := strings.TrimSpace(*parsedResult.TransactionTime)
		for _, tf := range timeFormats {
			if pt, err := time.Parse(tf, cleanTimeStr); err == nil {
				effectiveDate = pt
				break
			}
		}
	}

	bodySnippet := sanitizedBody
	if len(bodySnippet) > 1000 {
		bodySnippet = bodySnippet[:1000] + "..."
	}
	rawCopy := rawBody

	txRecord := &model.Transaction{
		ID:               uuid.New().String(),
		CoupleID:         coupleID,
		CreatedByUserID:  userID,
		PodID:            podID,
		Type:             txType,
		Direction:        direction,
		Amount:           amount,
		AdminFee:         adminFee,
		Title:            title,
		CuteTitle:        fmt.Sprintf("⚡ Auto: %s", title),
		NoteEmotional:    fmt.Sprintf("Auto-synced from %s email (%s)", providerCode, messageID),
		CuteNote:         "Sinkronisasi otomatis dari mutasi email bank kamu ✨",
		MoodTag:          "essential",
		Icon:             "credit-card",
		CuteIcon:         "💳",
		SourceType:       "EMAIL_SYNC",
		ExternalRefID:    &externalRefID,
		SyncStatus:       syncStatus,
		EmailSender:      &senderEmail,
		EmailBodySnippet: &bodySnippet,
		EmailRawBody:     &rawCopy,
		EmailHTMLBody:    rawHTML,
		EmailAccount:     accountEmail,
		EmailDate:        &effectiveDate,
		CreatedAt:        effectiveDate,
	}

	// Always mark log status as PROCESSED since transaction is successfully captured
	logRecord.Status = "PROCESSED"
	logRecord.ErrorMessage = nil

	// 8. Atomic execution: insert Transaction + update Pod balance + save log
	err = s.repo.ApplyMutationTransactionAtomic(txRecord, podID, deltaBalance, logRecord)
	if err != nil {
		return nil, fmt.Errorf("failed atomic transaction persistence: %w", err)
	}

	// 9. Reward gamification XP (+15 XP for automated sync)
	_ = s.kansoRepo.AddHarmonyXP(coupleID, 15)

	return txRecord, nil
}

func (s *mutationService) GetMutationLogs(coupleID string, limit int) ([]model.EmailMutationLog, error) {
	return s.repo.GetEmailMutationLogs(coupleID, limit)
}

func (s *mutationService) RetryMutationLog(ctx context.Context, logID string) (*model.Transaction, error) {
	logRecord, err := s.repo.GetEmailMutationLogByID(logID)
	if err != nil {
		return nil, fmt.Errorf("mutation log not found: %w", err)
	}

	// Parse again using clean body
	parsedResult, rawLLM, err := s.llmParser.ParseEmailMutation(ctx, logRecord.CleanBodyText)
	logRecord.LLMRawResponse = rawLLM
	if err != nil {
		errMsg := err.Error()
		logRecord.Status = "FAILED"
		logRecord.ErrorMessage = &errMsg
		_ = s.repo.UpdateEmailMutationLog(logRecord)
		return nil, err
	}

	if !parsedResult.IsTransaction {
		logRecord.Status = "IGNORED_NON_TX"
		_ = s.repo.UpdateEmailMutationLog(logRecord)
		return nil, nil
	}

	var providerCode, accountSuffix string
	if parsedResult.ProviderCode != nil {
		providerCode = *parsedResult.ProviderCode
	}
	if parsedResult.AccountSuffix != nil {
		accountSuffix = *parsedResult.AccountSuffix
	}

	var podID *string
	syncStatus := "VERIFIED"

	targetPod, _ := s.repo.FindPodByProviderAndSuffix(logRecord.CoupleID, providerCode, accountSuffix)
	if targetPod != nil {
		pID := targetPod.ID
		podID = &pID
	}

	direction := "OUT"
	if parsedResult.Direction != nil && (*parsedResult.Direction == "IN" || *parsedResult.Direction == "INTERNAL_TRANSFER") {
		direction = *parsedResult.Direction
	}

	amount := 0.0
	if parsedResult.Amount != nil {
		amount = *parsedResult.Amount
	}

	adminFee := 0.0
	if parsedResult.AdminFee != nil {
		adminFee = *parsedResult.AdminFee
	}

	var deltaBalance float64
	if podID != nil {
		if direction == "IN" {
			deltaBalance = amount
		} else {
			deltaBalance = -(amount + adminFee)
		}
	}

	hasher := sha256.New()
	hasher.Write([]byte(logRecord.MessageID))
	hashHex := hex.EncodeToString(hasher.Sum(nil))
	externalRefID := fmt.Sprintf("EMAIL-%s", hashHex[:24])

	title := "Bank Mutation"
	if parsedResult.MerchantOrCounterparty != nil && *parsedResult.MerchantOrCounterparty != "" {
		title = *parsedResult.MerchantOrCounterparty
	}

	txRecord := &model.Transaction{
		ID:              uuid.New().String(),
		CoupleID:        logRecord.CoupleID,
		CreatedByUserID: logRecord.UserID,
		PodID:           podID,
		Type:            "EXPENSE",
		Direction:       direction,
		Amount:          amount,
		AdminFee:        adminFee,
		Title:           title,
		CuteTitle:       fmt.Sprintf("⚡ Auto: %s", title),
		MoodTag:         "essential",
		Icon:            "credit-card",
		CuteIcon:        "💳",
		SourceType:      "EMAIL_SYNC",
		ExternalRefID:   &externalRefID,
		SyncStatus:      syncStatus,
		CreatedAt:       time.Now(),
	}
	if direction == "IN" {
		txRecord.Type = "INCOME"
	}

	// Check if this mutation transaction was already created previously
	existingTx, _ := s.repo.GetTransactionByExternalRefID(externalRefID)
	if existingTx != nil && existingTx.PodID != nil && *existingTx.PodID != "" {
		if podID != nil && *podID == *existingTx.PodID {
			deltaBalance = 0
		}
	}

	logRecord.Status = "PROCESSED"
	logRecord.ErrorMessage = nil

	err = s.repo.ApplyMutationTransactionAtomic(txRecord, podID, deltaBalance, logRecord)
	if err != nil {
		return nil, err
	}

	return txRecord, nil
}

func (s *mutationService) ReconcilePod(coupleID, userID, podID string, actualBalance float64) (*model.Transaction, error) {
	pod, err := s.kansoRepo.GetPodByID(podID)
	if err != nil {
		return nil, fmt.Errorf("pod not found: %w", err)
	}

	diff := actualBalance - pod.CurrentAmount
	if math.Abs(diff) < 0.01 {
		// Balance is already equal, nothing to reconcile
		return nil, nil
	}

	direction := "IN"
	txType := "INCOME"
	adjTitle := "Rekonsiliasi Saldo (Penyesuaian Masuk)"
	if diff < 0 {
		direction = "OUT"
		txType = "EXPENSE"
		adjTitle = "Rekonsiliasi Saldo (Penyesuaian Keluar)"
	}

	amount := math.Abs(diff)
	reconcileTx := &model.Transaction{
		ID:              uuid.New().String(),
		CoupleID:        coupleID,
		CreatedByUserID: userID,
		PodID:           &podID,
		Type:            txType,
		Direction:       direction,
		Amount:          amount,
		AdminFee:        0,
		Title:           adjTitle,
		CuteTitle:       fmt.Sprintf("⚖️ %s", adjTitle),
		NoteEmotional:   fmt.Sprintf("Penyesuaian manual saldo dari Rp %.0f menjadi Rp %.0f", pod.CurrentAmount, actualBalance),
		CuteNote:        "Saldo disamakan dengan rekening aslimu ✨",
		MoodTag:         "spark",
		Icon:            "scale",
		CuteIcon:        "⚖️",
		SourceType:      "RECONCILE",
		SyncStatus:      "VERIFIED",
		CreatedAt:       time.Now(),
	}

	err = s.repo.ReconcilePodAtomic(podID, actualBalance, reconcileTx)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile pod: %w", err)
	}

	// Reward gamification XP (+20 XP for reconciling bank balance)
	_ = s.kansoRepo.AddHarmonyXP(coupleID, 20)

	return reconcileTx, nil
}
