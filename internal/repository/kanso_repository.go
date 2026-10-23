package repository

import (
	"kanso/backend/internal/model"

	"gorm.io/gorm"
)

type KansoRepository interface {
	GetCoupleByPairCode(pairCode string) (*model.HouseholdCouple, error)
	GetUsersByCoupleID(coupleID string) ([]model.User, error)
	GetUserByID(id string) (*model.User, error)
	GetUserByEmail(email string) (*model.User, error)
	UpdateUser(user *model.User) error
	GetPodsByCoupleID(coupleID string) ([]model.PocketPod, error)
	GetPodByID(id string) (*model.PocketPod, error)
	CreatePod(pod *model.PocketPod) error
	UpdatePod(pod *model.PocketPod) error
	DeletePod(id string) error
	UpdatePodBalance(podID string, deltaAmount float64) error
	GetTransactionsByCoupleID(coupleID string, limit int) ([]model.Transaction, error)
	CreateTransaction(tx *model.Transaction) error
	DeleteTransaction(txID string) error
	BulkDeleteTransactions(txIDs []string) (int64, error)
	GetTransactionByID(txID string) (*model.Transaction, error)
	GetWishlistItems(coupleID string) ([]model.WishlistItem, error)
	CreateWishlistItem(item *model.WishlistItem) error
	VoteWishlistItem(itemID, partnerRole, vote string) error
	RecordLoveTip(tip *model.LoveTip) error
	AddHarmonyXP(coupleID string, xp int) error
	ToggleEmergencyMode(coupleID string, enabled bool) error
}

type kansoRepository struct {
	db *gorm.DB
}

func NewKansoRepository(db *gorm.DB) KansoRepository {
	return &kansoRepository{db: db}
}

func (r *kansoRepository) GetCoupleByPairCode(pairCode string) (*model.HouseholdCouple, error) {
	var couple model.HouseholdCouple
	if err := r.db.Where("pair_code = ?", pairCode).First(&couple).Error; err != nil {
		return nil, err
	}
	return &couple, nil
}

func (r *kansoRepository) GetUsersByCoupleID(coupleID string) ([]model.User, error) {
	var users []model.User
	if err := r.db.Where("couple_id = ?", coupleID).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *kansoRepository) GetUserByID(id string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *kansoRepository) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *kansoRepository) UpdateUser(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *kansoRepository) GetPodsByCoupleID(coupleID string) ([]model.PocketPod, error) {
	var pods []model.PocketPod
	if err := r.db.Where("couple_id = ?", coupleID).Order("badge_number asc").Find(&pods).Error; err != nil {
		return nil, err
	}
	return pods, nil
}

func (r *kansoRepository) GetPodByID(id string) (*model.PocketPod, error) {
	var pod model.PocketPod
	if err := r.db.Where("id = ?", id).First(&pod).Error; err != nil {
		return nil, err
	}
	return &pod, nil
}

func (r *kansoRepository) CreatePod(pod *model.PocketPod) error {
	return r.db.Create(pod).Error
}

func (r *kansoRepository) UpdatePod(pod *model.PocketPod) error {
	return r.db.Save(pod).Error
}

func (r *kansoRepository) DeletePod(id string) error {
	return r.db.Where("id = ?", id).Delete(&model.PocketPod{}).Error
}

func (r *kansoRepository) UpdatePodBalance(podID string, deltaAmount float64) error {
	return r.db.Model(&model.PocketPod{}).
		Where("id = ?", podID).
		Update("current_amount", gorm.Expr("current_amount + ?", deltaAmount)).Error
}

func (r *kansoRepository) GetTransactionsByCoupleID(coupleID string, limit int) ([]model.Transaction, error) {
	var txs []model.Transaction
	query := r.db.Where("couple_id = ?", coupleID).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&txs).Error; err != nil {
		return nil, err
	}
	return txs, nil
}

func (r *kansoRepository) CreateTransaction(tx *model.Transaction) error {
	return r.db.Create(tx).Error
}

func (r *kansoRepository) GetTransactionByID(txID string) (*model.Transaction, error) {
	var tx model.Transaction
	if err := r.db.Where("id = ?", txID).First(&tx).Error; err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *kansoRepository) DeleteTransaction(txID string) error {
	var tx model.Transaction
	if err := r.db.Where("id = ?", txID).First(&tx).Error; err != nil {
		return err
	}

	return r.db.Transaction(func(dbTx *gorm.DB) error {
		// If transaction had a pod_id, revert the balance effect
		if tx.PodID != nil && *tx.PodID != "" {
			var revertAmount float64
			if tx.Direction == "IN" || tx.Type == "INCOME" || tx.Type == "INCOME_SALARY" || tx.Type == "INCOME_PRELOVED" {
				// Revert by subtracting amount
				revertAmount = -tx.Amount
			} else {
				// Revert by refunding amount + admin fee
				revertAmount = tx.Amount + tx.AdminFee
			}
			if err := dbTx.Model(&model.PocketPod{}).
				Where("id = ?", *tx.PodID).
				Update("current_amount", gorm.Expr("current_amount + ?", revertAmount)).Error; err != nil {
				return err
			}
		}

		return dbTx.Delete(&tx).Error
	})
}

func (r *kansoRepository) BulkDeleteTransactions(txIDs []string) (int64, error) {
	if len(txIDs) == 0 {
		return 0, nil
	}

	var txs []model.Transaction
	if err := r.db.Where("id IN ?", txIDs).Find(&txs).Error; err != nil {
		return 0, err
	}

	if len(txs) == 0 {
		return 0, nil
	}

	err := r.db.Transaction(func(dbTx *gorm.DB) error {
		// Group revert amount by pod_id
		podDeltas := make(map[string]float64)
		for _, tx := range txs {
			if tx.PodID != nil && *tx.PodID != "" {
				var revertAmount float64
				if tx.Direction == "IN" || tx.Type == "INCOME" || tx.Type == "INCOME_SALARY" || tx.Type == "INCOME_PRELOVED" {
					revertAmount = -tx.Amount
				} else {
					revertAmount = tx.Amount + tx.AdminFee
				}
				podDeltas[*tx.PodID] += revertAmount
			}
		}

		for podID, delta := range podDeltas {
			if err := dbTx.Model(&model.PocketPod{}).
				Where("id = ?", podID).
				Update("current_amount", gorm.Expr("current_amount + ?", delta)).Error; err != nil {
				return err
			}
		}

		return dbTx.Where("id IN ?", txIDs).Delete(&model.Transaction{}).Error
	})

	return int64(len(txs)), err
}

func (r *kansoRepository) GetWishlistItems(coupleID string) ([]model.WishlistItem, error) {
	var items []model.WishlistItem
	if err := r.db.Where("couple_id = ?", coupleID).Order("created_at desc").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *kansoRepository) CreateWishlistItem(item *model.WishlistItem) error {
	return r.db.Create(item).Error
}

func (r *kansoRepository) VoteWishlistItem(itemID, partnerRole, vote string) error {
	col := "vote_partner_a"
	if partnerRole == "partner_b" {
		col = "vote_partner_b"
	}
	return r.db.Model(&model.WishlistItem{}).Where("id = ?", itemID).Update(col, vote).Error
}

func (r *kansoRepository) RecordLoveTip(tip *model.LoveTip) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(tip).Error; err != nil {
			return err
		}
		// Add +50 XP
		return tx.Model(&model.HouseholdCouple{}).
			Where("id = ?", tip.CoupleID).
			Update("harmony_xp", gorm.Expr("harmony_xp + ?", 50)).Error
	})
}

func (r *kansoRepository) AddHarmonyXP(coupleID string, xp int) error {
	return r.db.Model(&model.HouseholdCouple{}).
		Where("id = ?", coupleID).
		Update("harmony_xp", gorm.Expr("harmony_xp + ?", xp)).Error
}

func (r *kansoRepository) ToggleEmergencyMode(coupleID string, enabled bool) error {
	return r.db.Model(&model.HouseholdCouple{}).
		Where("id = ?", coupleID).
		Update("emergency_mode", enabled).Error
}
