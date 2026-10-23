package repository

import (
	"errors"
	"fmt"
	"kanso/backend/internal/model"

	"gorm.io/gorm"
)

type MutationRepository interface {
	CreateEmailMutationLog(log *model.EmailMutationLog) error
	GetEmailMutationLogByMessageID(messageID string) (*model.EmailMutationLog, error)
	GetEmailMutationLogs(coupleID string, limit int) ([]model.EmailMutationLog, error)
	GetEmailMutationLogByID(id string) (*model.EmailMutationLog, error)
	UpdateEmailMutationLog(log *model.EmailMutationLog) error

	FindPodByProviderAndSuffix(coupleID, providerCode, suffix string) (*model.PocketPod, error)
	GetTransactionByExternalRefID(refID string) (*model.Transaction, error)
	ApplyMutationTransactionAtomic(txRecord *model.Transaction, podID *string, deltaBalance float64, logRecord *model.EmailMutationLog) error
	ReconcilePodAtomic(podID string, actualBalance float64, reconcileTx *model.Transaction) error
}

type mutationRepository struct {
	db *gorm.DB
}

func NewMutationRepository(db *gorm.DB) MutationRepository {
	return &mutationRepository{db: db}
}

func (r *mutationRepository) CreateEmailMutationLog(log *model.EmailMutationLog) error {
	return r.db.Create(log).Error
}

func (r *mutationRepository) GetEmailMutationLogByMessageID(messageID string) (*model.EmailMutationLog, error) {
	var log model.EmailMutationLog
	if err := r.db.Where("message_id = ?", messageID).First(&log).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &log, nil
}

func (r *mutationRepository) GetEmailMutationLogs(coupleID string, limit int) ([]model.EmailMutationLog, error) {
	var logs []model.EmailMutationLog
	query := r.db.Where("couple_id = ?", coupleID).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

func (r *mutationRepository) GetEmailMutationLogByID(id string) (*model.EmailMutationLog, error) {
	var log model.EmailMutationLog
	if err := r.db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *mutationRepository) UpdateEmailMutationLog(log *model.EmailMutationLog) error {
	return r.db.Save(log).Error
}

func (r *mutationRepository) FindPodByProviderAndSuffix(coupleID, providerCode, suffix string) (*model.PocketPod, error) {
	var pod model.PocketPod

	if providerCode != "" && suffix != "" {
		// Exact match on both provider code and account suffix
		if err := r.db.Where("couple_id = ? AND LOWER(provider_code) = LOWER(?) AND account_number_suffix = ?", coupleID, providerCode, suffix).First(&pod).Error; err == nil {
			return &pod, nil
		}
	}

	if providerCode != "" {
		// Fallback match on provider code
		if err := r.db.Where("couple_id = ? AND LOWER(provider_code) = LOWER(?)", coupleID, providerCode).First(&pod).Error; err == nil {
			return &pod, nil
		}
	}

	// Suffix only match (e.g. last 4 digits of card/account)
	if suffix != "" {
		if err := r.db.Where("couple_id = ? AND account_number_suffix = ?", coupleID, suffix).First(&pod).Error; err == nil {
			return &pod, nil
		}
	}

	// If only 1 pod exists for the couple, associate with that primary pod
	var pods []model.PocketPod
	if err := r.db.Where("couple_id = ?", coupleID).Limit(2).Find(&pods).Error; err == nil && len(pods) == 1 {
		return &pods[0], nil
	}

	// Not found
	return nil, nil
}

func (r *mutationRepository) GetTransactionByExternalRefID(refID string) (*model.Transaction, error) {
	var tx model.Transaction
	if err := r.db.Where("external_ref_id = ?", refID).First(&tx).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tx, nil
}

func (r *mutationRepository) ApplyMutationTransactionAtomic(
	txRecord *model.Transaction,
	podID *string,
	deltaBalance float64,
	logRecord *model.EmailMutationLog,
) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Create or update transaction if provided
		if txRecord != nil {
			if txRecord.ExternalRefID != nil && *txRecord.ExternalRefID != "" {
				var existing model.Transaction
				err := tx.Where("external_ref_id = ?", *txRecord.ExternalRefID).First(&existing).Error
				if err == nil {
					// Existing transaction found - update its pod_id and sync_status instead of inserting duplicate
					txRecord.ID = existing.ID
					if err := tx.Model(&model.Transaction{}).
						Where("id = ?", existing.ID).
						Updates(map[string]interface{}{
							"pod_id":      txRecord.PodID,
							"sync_status": txRecord.SyncStatus,
							"title":       txRecord.Title,
							"amount":      txRecord.Amount,
							"direction":   txRecord.Direction,
						}).Error; err != nil {
						return fmt.Errorf("failed to update existing transaction: %w", err)
					}
				} else {
					if err := tx.Create(txRecord).Error; err != nil {
						return fmt.Errorf("failed to insert transaction: %w", err)
					}
				}
			} else {
				if err := tx.Create(txRecord).Error; err != nil {
					return fmt.Errorf("failed to insert transaction: %w", err)
				}
			}
		}

		// 2. Adjust pod balance directly if podID exists and delta is non-zero
		// Allows realistic negative balance if expenses exceed current stash
		if podID != nil && *podID != "" && deltaBalance != 0 {
			if err := tx.Model(&model.PocketPod{}).
				Where("id = ?", *podID).
				Update("current_amount", gorm.Expr("current_amount + ?", deltaBalance)).Error; err != nil {
				return fmt.Errorf("failed to update pod balance: %w", err)
			}
		}

		// 3. Save or update mutation log
		if logRecord != nil {
			if logRecord.ID == "" {
				if err := tx.Create(logRecord).Error; err != nil {
					return fmt.Errorf("failed to create mutation log: %w", err)
				}
			} else {
				if err := tx.Save(logRecord).Error; err != nil {
					return fmt.Errorf("failed to save mutation log: %w", err)
				}
			}
		}

		return nil
	})
}

func (r *mutationRepository) ReconcilePodAtomic(podID string, actualBalance float64, reconcileTx *model.Transaction) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Set absolute new balance
		if err := tx.Model(&model.PocketPod{}).
			Where("id = ?", podID).
			Update("current_amount", actualBalance).Error; err != nil {
			return fmt.Errorf("failed to update pocket pod balance during reconcile: %w", err)
		}

		// Insert adjustment transaction
		if reconcileTx != nil {
			if err := tx.Create(reconcileTx).Error; err != nil {
				return fmt.Errorf("failed to insert reconcile transaction: %w", err)
			}
		}

		return nil
	})
}
