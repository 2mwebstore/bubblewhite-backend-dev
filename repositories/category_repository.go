package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type CategoryRepository struct {
	*BaseRepository[models.Category]
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{BaseRepository: NewBaseRepository[models.Category](db)}
}

func (r *CategoryRepository) FindBySlug(slug string) (*models.Category, error) {
	var cat models.Category
	if err := r.DB.Where("slug = ?", slug).First(&cat).Error; err != nil {
		return nil, err
	}
	return &cat, nil
}

// FindAllWithProductCount returns categories in admin-controlled display
// order (sort_order ASC, falling back to newest-first for categories that
// haven't been manually reordered yet — every category is seeded/created
// with sort_order 0, so ties there resolve to created_at), with how many
// products currently reference it (products.category_id stores the
// category's SLUG, not its numeric id — see Category's doc comment — so
// this is a manual grouped count rather than a GORM association preload).
//
// activeOnly=true returns only categories with IsActive=true (the public
// storefront listing); activeOnly=false returns every category regardless
// of status (the admin listing, so staff can find and re-enable a
// disabled one).
func (r *CategoryRepository) FindAllWithProductCount(activeOnly bool) ([]models.Category, error) {
	var categories []models.Category
	q := r.DB.Order("sort_order ASC, created_at DESC")
	if activeOnly {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&categories).Error; err != nil {
		return nil, err
	}

	type countRow struct {
		CategoryID string
		Count      int64
	}
	var rows []countRow
	if err := r.DB.Table("products").
		Select("category_id, COUNT(*) as count").
		Group("category_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.CategoryID] = row.Count
	}
	for i := range categories {
		categories[i].ProductCount = counts[categories[i].Slug]
	}
	return categories, nil
}
