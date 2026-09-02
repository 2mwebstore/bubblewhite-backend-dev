package services

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
)

type CategoryService struct {
	Categories *repositories.CategoryRepository
}

func NewCategoryService(categories *repositories.CategoryRepository) *CategoryService {
	return &CategoryService{Categories: categories}
}

// ListPublic returns only active categories, in admin-controlled display
// order — what the storefront (footer, shop filters, home category strip)
// actually shows.
func (s *CategoryService) ListPublic() ([]models.Category, error) {
	return s.Categories.FindAllWithProductCount(true)
}

// ListAdmin returns every category regardless of active status — so staff
// managing categories can still find and re-enable a disabled one.
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
	return s.Categories.Create(cat)
}

func (s *CategoryService) Update(cat *models.Category) error {
	return s.Categories.Update(cat)
}

func (s *CategoryService) Delete(id uint) error {
	return s.Categories.Delete(id)
}
