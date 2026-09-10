package services

import (
	"time"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

const categoryPublicCacheKey = "categories:public"
const categoryCacheTTL = 10 * time.Minute

type CategoryService struct {
	Categories *repositories.CategoryRepository
	Cache      *utils.Cache
}

func NewCategoryService(categories *repositories.CategoryRepository) *CategoryService {
	return &CategoryService{Categories: categories, Cache: utils.NewCache()}
}

// ListPublic returns only active categories, in admin-controlled display
// order — what the storefront (footer, shop filters, home category strip)
// actually shows on every single storefront page. Cached: this rarely
// changes (only when an admin edits categories) but is read constantly.
func (s *CategoryService) ListPublic() ([]models.Category, error) {
	if cached, ok := s.Cache.Get(categoryPublicCacheKey); ok {
		return cached.([]models.Category), nil
	}
	categories, err := s.Categories.FindAllWithProductCount(true)
	if err != nil {
		return nil, err
	}
	s.Cache.Set(categoryPublicCacheKey, categories, categoryCacheTTL)
	return categories, nil
}

// ListAdmin returns every category regardless of active status — so staff
// managing categories can still find and re-enable a disabled one.
// Deliberately NOT cached: an admin who just changed something here
// should always see the current state immediately, not a cached view
// from moments ago on the same page they're actively editing.
func (s *CategoryService) ListAdmin() ([]models.Category, error) {
	return s.Categories.FindAllWithProductCount(false)
}

func (s *CategoryService) GetByID(id uint) (*models.Category, error) {
	return s.Categories.FindByID(id)
}

func (s *CategoryService) GetBySlug(slug string) (*models.Category, error) {
	return s.Categories.FindBySlug(slug)
}

func (s *CategoryService) Create(cat *models.Category) error {
	if err := s.Categories.Create(cat); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}

func (s *CategoryService) Update(cat *models.Category) error {
	if err := s.Categories.Update(cat); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}

func (s *CategoryService) Delete(id uint) error {
	if err := s.Categories.Delete(id); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}
