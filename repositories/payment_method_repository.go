package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type PaymentMethodRepository struct {
	*BaseRepository[models.PaymentMethod]
}

func NewPaymentMethodRepository(db *gorm.DB) *PaymentMethodRepository {
	return &PaymentMethodRepository{BaseRepository: NewBaseRepository[models.PaymentMethod](db)}
}

// ClearPrimary unsets IsPrimary on every row except the given ID — used
// when an admin sets a new primary payment method, so exactly one row is
// ever primary at a time. Deliberately a raw UPDATE (not read-then-save
// each row) so this stays a single query regardless of how many payment
// methods exist.
func (r *PaymentMethodRepository) ClearPrimary(exceptID uint) error {
	return r.DB.Model(&models.PaymentMethod{}).
		Where("id != ?", exceptID).
		Update("is_primary", false).Error
}
