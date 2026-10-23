package database

import (
	"log"
	"time"
	"kanso/backend/internal/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func SeedDuoInitialData(db *gorm.DB) {
	// Auto migrate all Duo tables
	_ = db.AutoMigrate(
		&model.HouseholdCouple{},
		&model.User{},
		&model.PocketPod{},
		&model.Transaction{},
		&model.WishlistItem{},
		&model.LoveTip{},
		&model.EmailMutationLog{},
		&model.UserOAuthSession{},
	)

	// Check if couple exists
	var count int64
	db.Model(&model.HouseholdCouple{}).Count(&count)
	if count > 0 {
		return
	}

	coupleID := uuid.NewString()
	couple := model.HouseholdCouple{
		ID:            coupleID,
		PairCode:      "DUO-LOVE",
		HarmonyXP:     250,
		PetLevel:      2,
		EmergencyMode: false,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	db.Create(&couple)

	// Seed Partner A & Partner B
	userA := model.User{
		ID:              uuid.NewString(),
		CoupleID:        coupleID,
		Email:           "dimas@kanso.love",
		DisplayName:     "Dimas",
		Role:            "partner_a",
		ThemePreference: "zen",
		SoundEnabled:    true,
		AvatarURL:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb",
		CreatedAt:       time.Now(),
	}
	userB := model.User{
		ID:              uuid.NewString(),
		CoupleID:        coupleID,
		Email:           "eva@kanso.love",
		DisplayName:     "Eva",
		Role:            "partner_b",
		ThemePreference: "kawaii",
		SoundEnabled:    true,
		AvatarURL:       "https://images.unsplash.com/photo-1517841905240-472988babdf9",
		CreatedAt:       time.Now(),
	}
	db.Create(&userA)
	db.Create(&userB)

	// Seed Pods in Rupiah
	pods := []model.PocketPod{
		{
			ID:            uuid.NewString(),
			CoupleID:      coupleID,
			BadgeNumber:   "01",
			NameZen:       "Ritual Kopi & Dapur",
			NameKawaii:    "Strawberry Latte & Sarapan 🍓",
			TargetAmount:  500000.0,
			CurrentAmount: 385000.0,
			CategoryType:  "shared",
			ColorClass:    "bg-[#F6E7B8]/70",
			LiquidBgClass: "bg-[#F6E7B8] text-[#F6E7B8]",
			HasDuck:       false,
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.NewString(),
			CoupleID:      coupleID,
			BadgeNumber:   "02",
			NameZen:       "Liburan & Jalan Berdua",
			NameKawaii:    "Piknik Sakura Kyoto 🌸",
			TargetAmount:  10000000.0,
			CurrentAmount: 4500000.0,
			CategoryType:  "shared",
			ColorClass:    "bg-[#DBE7EC]/80",
			LiquidBgClass: "bg-[#DBE7EC] text-[#DBE7EC]",
			HasDuck:       true,
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.NewString(),
			CoupleID:      coupleID,
			BadgeNumber:   "03",
			NameZen:       "Dana Siaga & Darurat",
			NameKawaii:    "Sarang Selimut Hangat 🧸",
			TargetAmount:  15000000.0,
			CurrentAmount: 8250000.0,
			CategoryType:  "shared",
			ColorClass:    "bg-[#DCE8D6]/80",
			LiquidBgClass: "bg-[#DCE8D6] text-[#DCE8D6]",
			HasDuck:       false,
			CreatedAt:     time.Now(),
		},
	}
	for _, p := range pods {
		db.Create(&p)
	}

	// Seed Initial Transactions in Rupiah
	txs := []model.Transaction{
		{
			ID:              uuid.NewString(),
			CoupleID:        coupleID,
			CreatedByUserID: userA.ID,
			Type:            "EXPENSE",
			Amount:          45000.0,
			Title:           "Espresso Pagi & Roti Canai",
			CuteTitle:       "Strawberry Mochi Latte 🍓",
			NoteEmotional:   "Jedah hening pagi berdua",
			CuteNote:        "Semangat manis pembuka hari!",
			MoodTag:         "calm",
			Icon:            "🍵",
			CuteIcon:        "🍓",
			CreatedAt:       time.Now().Add(-2 * time.Hour),
		},
		{
			ID:              uuid.NewString(),
			CoupleID:        coupleID,
			CreatedByUserID: userB.ID,
			Type:            "EXPENSE",
			Amount:          175000.0,
			Title:           "Buku Arsitektur Minimalis",
			CuteTitle:       "Buku Ilustrasi Kucing Lucu 📖",
			NoteEmotional:   "Inspirasi sudut ruang keluarga",
			CuteNote:        "Bikin hati hangat & senang ✨",
			MoodTag:         "spark",
			Icon:            "📚",
			CuteIcon:        "✨",
			CreatedAt:       time.Now().Add(-24 * time.Hour),
		},
	}
	for _, t := range txs {
		db.Create(&t)
	}

	// Seed Initial Wishlist in Rupiah with 7-day cooldown
	wish := model.WishlistItem{
		ID:                  uuid.NewString(),
		CoupleID:            coupleID,
		CreatedByUserID:     userA.ID,
		Title:               "Noise-Cancelling Headphones",
		Price:               1250000.0,
		Category:            "Gadget & Fokus Kerja",
		EmotionalEquivalent: "Setara dengan 35 hari jatah sayur & buah segar keluarga 🥦",
		VotePartnerA:        "need",
		VotePartnerB:        "delay",
		CooldownEndsAt:      time.Now().Add(4 * 24 * time.Hour),
		Status:              "in_cooldown",
		CreatedAt:           time.Now().Add(-3 * 24 * time.Hour),
	}
	db.Create(&wish)

	log.Printf("✨ Kanso Duo initial couple seeded: DUO-LOVE (Dimas & Eva - Indonesia Rupiah)")
}
