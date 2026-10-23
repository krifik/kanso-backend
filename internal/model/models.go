package model

import (
	"time"
)

// 1. Couples Pairing Table
type HouseholdCouple struct {
	ID            string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	PairCode      string    `gorm:"uniqueIndex;type:varchar(10)" json:"pair_code"`
	HarmonyXP     int       `gorm:"default:150" json:"harmony_xp"`
	PetLevel      int       `gorm:"default:1" json:"pet_level"`
	EmergencyMode bool      `gorm:"default:false" json:"emergency_mode"`
	Status        string    `gorm:"default:'active'" json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// 2. Users Table with Independent Theme Preference
type User struct {
	ID              string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID        string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	Email           string    `gorm:"uniqueIndex;type:varchar(255)" json:"email"`
	DisplayName     string    `gorm:"type:varchar(100)" json:"display_name"`
	Role            string    `gorm:"type:varchar(20)" json:"role"` // partner_a, partner_b
	ThemePreference string    `gorm:"default:'zen'" json:"theme_preference"` // zen, kawaii
	SoundEnabled    bool      `gorm:"default:true" json:"sound_enabled"`
	AvatarURL       string    `json:"avatar_url"`
	CreatedAt       time.Time `json:"created_at"`
}

// 3. Pocket Pods (Capsules with Liquid Waves)
type PocketPod struct {
	ID                  string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID            string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	BadgeNumber         string    `gorm:"type:varchar(10)" json:"badge_number"`
	NameZen             string    `gorm:"type:varchar(100)" json:"name_zen"`
	NameKawaii          string    `gorm:"type:varchar(100)" json:"name_kawaii"`
	TargetAmount        float64   `gorm:"type:decimal(15,2)" json:"target_amount"`
	CurrentAmount       float64   `gorm:"type:decimal(15,2);default:0" json:"current_amount"`
	CategoryType        string    `gorm:"default:'shared'" json:"category_type"`
	ColorClass          string    `gorm:"type:varchar(100)" json:"color_class"`
	LiquidBgClass       string    `gorm:"type:varchar(100)" json:"liquid_bg_class"`
	HasDuck             bool      `gorm:"default:false" json:"has_duck"`
	ProviderCode        *string   `gorm:"type:varchar(50);index" json:"provider_code"`        // e.g. BCA, JAGO, MANDIRI, GOPAY
	AccountNumberSuffix *string   `gorm:"type:varchar(10)" json:"account_number_suffix"`      // e.g. 1234
	CreatedAt           time.Time `json:"created_at"`
}

// 4. Unified Transactions (Buy, Sell Preloved, Salary, Allowance)
type Transaction struct {
	ID              string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID        string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	CreatedByUserID string    `gorm:"index;type:varchar(36)" json:"created_by_user_id"`
	PodID           *string   `gorm:"index;type:varchar(36)" json:"pod_id"`
	Type            string    `gorm:"type:varchar(25)" json:"type"` // EXPENSE, INCOME, INCOME_SALARY, INCOME_PRELOVED, TRANSFER, ALLOWANCE
	Direction       string    `gorm:"type:varchar(20);default:'OUT'" json:"direction"` // IN, OUT, INTERNAL_TRANSFER
	Amount          float64   `gorm:"type:decimal(15,2)" json:"amount"`
	AdminFee        float64   `gorm:"type:decimal(15,2);default:0" json:"admin_fee"`
	Title           string    `gorm:"type:varchar(255)" json:"title"`
	CuteTitle       string    `gorm:"type:varchar(255)" json:"cute_title"`
	NoteEmotional   string    `gorm:"type:text" json:"note_emotional"`
	CuteNote        string    `gorm:"type:text" json:"cute_note"`
	MoodTag         string    `gorm:"type:varchar(50)" json:"mood_tag"` // calm, spark, essential, oops
	Icon            string    `gorm:"type:varchar(20)" json:"icon"`
	CuteIcon        string    `gorm:"type:varchar(20)" json:"cute_icon"`
	SourceType       string     `gorm:"type:varchar(20);default:'MANUAL'" json:"source_type"` // MANUAL, EMAIL_SYNC, RECONCILE
	ExternalRefID    *string    `gorm:"type:varchar(100);uniqueIndex" json:"external_ref_id"` // Hash of Message-ID
	SyncStatus       string     `gorm:"type:varchar(20);default:'VERIFIED'" json:"sync_status"` // VERIFIED, NEEDS_REVIEW
	EmailSender      *string    `gorm:"type:varchar(150)" json:"email_sender"`
	EmailSubject     *string    `gorm:"type:varchar(255)" json:"email_subject"`
	EmailBodySnippet *string    `gorm:"type:text" json:"email_body_snippet"`
	EmailRawBody     *string    `gorm:"type:text" json:"email_raw_body"`
	EmailHTMLBody    *string    `gorm:"type:text" json:"email_html_body"`
	EmailAccount     *string    `gorm:"type:varchar(255)" json:"email_account"` // Google/Gmail account address that received this transaction
	EmailDate        *time.Time `json:"email_date"`
	CreatedAt        time.Time  `json:"created_at"`
}

// 5. Wishlist Item in 7-Day Cooldown Chamber
type WishlistItem struct {
	ID                  string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID            string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	CreatedByUserID     string    `gorm:"type:varchar(36)" json:"created_by_user_id"`
	Title               string    `gorm:"type:varchar(255)" json:"title"`
	Price               float64   `gorm:"type:decimal(15,2)" json:"price"`
	Category            string    `gorm:"type:varchar(100)" json:"category"`
	EmotionalEquivalent string    `gorm:"type:text" json:"emotional_equivalent"`
	VotePartnerA        string    `gorm:"type:varchar(20);default:'pending'" json:"vote_partner_a"` // need, delay, later, pending
	VotePartnerB        string    `gorm:"type:varchar(20);default:'pending'" json:"vote_partner_b"`
	CooldownEndsAt      time.Time `json:"cooldown_ends_at"`
	Status              string    `gorm:"default:'in_cooldown'" json:"status"` // in_cooldown, approved, cancelled
	CreatedAt           time.Time `json:"created_at"`
}

// 6. Micro-Allowance Love Tap
type LoveTip struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID    string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	FromUserID  string    `gorm:"type:varchar(36)" json:"from_user_id"`
	ToUserID    string    `gorm:"type:varchar(36)" json:"to_user_id"`
	Amount      float64   `gorm:"type:decimal(15,2)" json:"amount"`
	StickerNote string    `gorm:"type:varchar(255)" json:"sticker_note"`
	CreatedAt   time.Time `json:"created_at"`
}

// 7. Daily Gamification Quests
type DailyQuest struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Title       string `json:"title"`
	CuteTitle   string `json:"cute_title"`
	Description string `json:"description"`
	RewardXP    int    `json:"reward_xp"`
	Completed   bool   `json:"completed"`
	Icon        string `json:"icon"`
}

// 8. Smart Email Mutation Logs (SPEC_EMAIL_SYNC.md 3.3)
type EmailMutationLog struct {
	ID             string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	CoupleID       string    `gorm:"index;type:varchar(36)" json:"couple_id"`
	UserID         string    `gorm:"index;type:varchar(36)" json:"user_id"`
	MessageID      string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"message_id"`
	SenderEmail    string    `gorm:"type:varchar(150);not null" json:"sender_email"`
	CleanBodyText  string    `gorm:"type:text;not null" json:"clean_body_text"`
	RawBodyText    *string   `gorm:"type:text" json:"raw_body_text"`
	RawHTMLBody    *string   `gorm:"type:text" json:"raw_html_body"`
	LLMRawResponse string    `gorm:"type:text" json:"llm_raw_response"`
	Status         string    `gorm:"type:varchar(30);not null" json:"status"` // PROCESSED, IGNORED_NON_TX, NEEDS_REVIEW, FAILED
	ErrorMessage   *string   `gorm:"type:text" json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
}

// 9. Persistent OAuth SSO Session (Persistent Gmail Sync & User Auth)
type UserOAuthSession struct {
	ID           string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Provider     string    `gorm:"type:varchar(50);default:'google';index" json:"provider"` // google
	UserEmail    string    `gorm:"uniqueIndex;type:varchar(255);not null" json:"user_email"`
	UserName     string    `gorm:"type:varchar(150)" json:"user_name"`
	AvatarURL    string    `gorm:"type:text" json:"avatar_url"`
	AccessToken  string    `gorm:"type:text;not null" json:"access_token"`
	RefreshToken string    `gorm:"type:text" json:"refresh_token"`
	TokenExpiry  time.Time `json:"token_expiry"`
	CoupleID     string    `gorm:"type:varchar(36);default:'DUO-LOVE'" json:"couple_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
