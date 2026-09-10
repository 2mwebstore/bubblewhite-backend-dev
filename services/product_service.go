package services

import (
	"fmt"
	"time"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

// productCacheTTL is shorter than settings/categories — product listings
// are the page a customer is most likely to be actively browsing right
// as a restock or price change happens, so a shorter window bounds how
// long a customer could see stale stock/pricing even in the rare case a
// write's cache-clear and a read overlap in a way that isn't already
// handled by clearing on every write below.
const productCacheTTL = 2 * time.Minute

type ProductService struct {
	Products *repositories.ProductRepository
	Cache    *utils.Cache
}

func NewProductService(products *repositories.ProductRepository) *ProductService {
	return &ProductService{Products: products, Cache: utils.NewCache()}
}

type productListResult struct {
	Products []models.Product
	Total    int64
}

// listCacheKey encodes every field of both filter and page into one
// string — each distinct combination of category/search/sort/page is
// genuinely a different result set and needs its own cache entry. A
// customer filtering "shirts, sorted by price, page 2" must never be
// served whatever happened to be cached under "all products, page 1".
func listCacheKey(filter repositories.ProductFilter, page utils.PageParams) string {
	featured := "nil"
	if filter.Featured != nil {
		featured = fmt.Sprintf("%v", *filter.Featured)
	}
	return fmt.Sprintf("list:cat=%s|search=%s|featured=%s|sortBy=%s|sortDir=%s|page=%d|size=%d",
		filter.Category, filter.Search, featured, filter.SortBy, filter.SortDir, page.Page, page.PageSize)
}

// List returns a page of products matching the given filter — the
// storefront's main browsing/search endpoint, cached per unique
// filter+pagination combination since each one is a genuinely different
// result set.
func (s *ProductService) List(filter repositories.ProductFilter, page utils.PageParams) ([]models.Product, int64, error) {
	key := listCacheKey(filter, page)
	if cached, ok := s.Cache.Get(key); ok {
		result := cached.(productListResult)
		return result.Products, result.Total, nil
	}

	scope := filter.Scope()

	total, err := s.Products.Count(scope)
	if err != nil {
		return nil, 0, err
	}

	products, err := s.Products.FindAll(scope, page.Scope())
	if err != nil {
		return nil, 0, err
	}

	s.Cache.Set(key, productListResult{Products: products, Total: total}, productCacheTTL)
	return products, total, nil
}

// GetByID is hit on every single product detail page view — cached for
// the same reason as List above.
func (s *ProductService) GetByID(id string) (*models.Product, error) {
	key := "detail:" + id
	if cached, ok := s.Cache.Get(key); ok {
		return cached.(*models.Product), nil
	}
	product, err := s.Products.FindByID(id)
	if err != nil {
		return nil, err
	}
	s.Cache.Set(key, product, productCacheTTL)
	return product, nil
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
	if err := s.Products.Create(p); err != nil {
		return err
	}
	// Clear() rather than targeted invalidation: a new product could
	// belong to any category and match any existing search/sort
	// combination someone has cached a result for — there's no cheap
	// way to know which of the many possible List() cache keys are
	// affected, so clearing all of them and letting the next read
	// repopulate is simpler and safer than trying to guess.
	s.Cache.Clear()
	return nil
}

func (s *ProductService) Update(p *models.Product) error {
	if err := s.Products.Update(p); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}

func (s *ProductService) Delete(id string) error {
	if err := s.Products.Delete(id); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}
