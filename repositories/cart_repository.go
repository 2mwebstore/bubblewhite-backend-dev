package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type CartRepository struct {
	*BaseRepository[models.CartItem]
}

func NewCartRepository(db *gorm.DB) *CartRepository {
	return &CartRepository{BaseRepository: NewBaseRepository[models.CartItem](db)}
}

func (r *CartRepository) FindByCustomer(customerID uint) ([]models.CartItem, error) {
	var items []models.CartItem
	err := r.DB.Where("customer_id = ?", customerID).Order("created_at ASC").Find(&items).Error
	return items, err
}

// FindLine finds the existing cart line for this exact
// (customer, product, size, image) combination, if one exists — used to
// decide whether AddItem should increment an existing line or insert a new
// one. Image is part of the key so the same product/size but a different
// preview image (e.g. a different color shot) is treated as a distinct
// line, not merged into an unrelated variant's quantity.
func (r *CartRepository) FindLine(customerID uint, productID, size, image string) (*models.CartItem, error) {
	var item models.CartItem
	err := r.DB.Where("customer_id = ? AND product_id = ? AND size = ? AND image = ?", customerID, productID, size, image).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// FindByIDForCustomer loads a cart item, but ONLY if it actually belongs to
// this customer — the check that stops customer A from updating/deleting
// customer B's cart line by guessing an item ID.
func (r *CartRepository) FindByIDForCustomer(id, customerID uint) (*models.CartItem, error) {
	var item models.CartItem
	err := r.DB.Where("id = ? AND customer_id = ?", id, customerID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *CartRepository) DeleteAllForCustomer(customerID uint) error {
	return r.DB.Where("customer_id = ?", customerID).Delete(&models.CartItem{}).Error
}
