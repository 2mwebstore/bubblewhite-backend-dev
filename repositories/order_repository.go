package repositories

import (
	"strconv"
	"strings"

	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type OrderRepository struct {
	*BaseRepository[models.Order]
}

func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{BaseRepository: NewBaseRepository[models.Order](db)}
}

// OrderFilter is the admin order list's filter set — same
// struct-plus-Scope() pattern as ProductFilter.
type OrderFilter struct {
	Search        string // matches order ID exactly if numeric, or phone (LIKE) either way
	CustomerID    string
	Status        string
	PaymentMethod string
}

func (f OrderFilter) Scope() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if f.Search != "" {
			// The admin UI displays orders by their "BW-000042" reference
			// (see models.Order.Reference), not the raw numeric ID — an
			// admin searching would reasonably paste exactly what they see
			// on screen. Strip that formatting back down to the plain
			// number before attempting the numeric match, so searching
			// "BW-000042", "000042", or "42" all correctly find order 42.
			numericPart := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(f.Search)), "BW-")
			numericPart = strings.TrimLeft(numericPart, "0")
			if id, err := strconv.ParseUint(numericPart, 10, 64); err == nil && numericPart != "" {
				db = db.Where("id = ? OR phone LIKE ?", id, "%"+f.Search+"%")
			} else {
				db = db.Where("phone LIKE ?", "%"+f.Search+"%")
			}
		}
		if f.CustomerID != "" {
			db = db.Where("customer_id = ?", f.CustomerID)
		}
		if f.Status != "" {
			db = db.Where("status = ?", f.Status)
		}
		if f.PaymentMethod != "" {
			db = db.Where("payment_method = ?", f.PaymentMethod)
		}
		return db
	}
}

// FindByCustomer returns a customer's own orders, newest first, WITHOUT
// items preloaded — the list view only needs id/total/status/date, not
// every line item; detail views (FindByIDForCustomer/FindByIDAdmin) do
// preload.
func (r *OrderRepository) FindByCustomer(customerID uint, scopes ...func(*gorm.DB) *gorm.DB) ([]models.Order, error) {
	var orders []models.Order
	q := r.DB.Where("customer_id = ?", customerID).Order("created_at DESC")
	for _, scope := range scopes {
		q = scope(q)
	}
	err := q.Find(&orders).Error
	return orders, err
}

// FindByIDForCustomer loads an order WITH its items, but only if it
// actually belongs to this customer.
func (r *OrderRepository) FindByIDForCustomer(id, customerID uint) (*models.Order, error) {
	var order models.Order
	err := r.DB.Preload("Items").Where("id = ? AND customer_id = ?", id, customerID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByIDAdmin loads any order with its items — no ownership restriction,
// for the admin panel.
func (r *OrderRepository) FindByIDAdmin(id uint) (*models.Order, error) {
	var order models.Order
	err := r.DB.Preload("Items").First(&order, id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *OrderRepository) FindAllAdmin(scopes ...func(*gorm.DB) *gorm.DB) ([]models.Order, error) {
	var orders []models.Order
	q := r.DB.Order("created_at DESC")
	for _, s := range scopes {
		q = s(q)
	}
	err := q.Find(&orders).Error
	return orders, err
}
