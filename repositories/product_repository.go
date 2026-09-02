package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type ProductRepository struct {
	*BaseRepository[models.Product]
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{BaseRepository: NewBaseRepository[models.Product](db)}
}

// ProductFilter captures every optional catalog filter/sort the storefront
// (and admin panel) can apply to a product listing.
type ProductFilter struct {
	Category string
	Search   string
	Featured *bool
	SortBy   string // "price" | "name" | "created_at"
	SortDir  string // "ASC" | "DESC"
}

// Scope turns a ProductFilter into a composable GORM scope.
func (f ProductFilter) Scope() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if f.Category != "" {
			db = db.Where("category_id = ?", f.Category)
		}
		if f.Search != "" {
			db = db.Where("name LIKE ?", "%"+f.Search+"%")
		}
		if f.Featured != nil {
			db = db.Where("featured = ?", *f.Featured)
		}

		// No explicit sort requested — show the most recently created
		// products first. IDs are admin-chosen strings (e.g. "bw-basic-tee"),
		// not auto-increment, so sorting by id gave alphabetical order and
		// never actually reflected "latest" the way a shopper or admin
		// would expect.
		if f.SortBy == "" {
			return db.Order("created_at DESC")
		}

		sortBy := "created_at"
		switch f.SortBy {
		case "price", "name", "created_at":
			sortBy = f.SortBy
		}
		sortDir := "ASC"
		if f.SortDir == "DESC" {
			sortDir = "DESC"
		}
		return db.Order(sortBy + " " + sortDir)
	}
}

// FindRelated returns other products in the same category, excluding excludeID.
func (r *ProductRepository) FindRelated(category, excludeID string, limit int) ([]models.Product, error) {
	var products []models.Product
	err := r.DB.
		Where("category_id = ? AND id <> ?", category, excludeID).
		Limit(limit).
		Find(&products).Error
	return products, err
}
