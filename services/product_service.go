package services

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

type ProductService struct {
	Products *repositories.ProductRepository
}

func NewProductService(products *repositories.ProductRepository) *ProductService {
	return &ProductService{Products: products}
}

// List returns a page of products matching the given filter.
func (s *ProductService) List(filter repositories.ProductFilter, page utils.PageParams) ([]models.Product, int64, error) {
	scope := filter.Scope()

	total, err := s.Products.Count(scope)
	if err != nil {
		return nil, 0, err
	}

	products, err := s.Products.FindAll(scope, page.Scope())
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

func (s *ProductService) GetByID(id string) (*models.Product, error) {
	return s.Products.FindByID(id)
}

func (s *ProductService) Related(product *models.Product, limit int) ([]models.Product, error) {
	if limit <= 0 || limit > 20 {
		limit = 4
	}
	return s.Products.FindRelated(product.CategoryID, product.ID, limit)
}

func (s *ProductService) Create(p *models.Product) error {
	if p.ID == "" {
		p.ID = utils.NewID("bw")
	}
	return s.Products.Create(p)
}

func (s *ProductService) Update(p *models.Product) error {
	return s.Products.Update(p)
}

func (s *ProductService) Delete(id string) error {
	return s.Products.Delete(id)
}
