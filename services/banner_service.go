package services

import (
	"time"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"
)

const bannerActiveCacheKey = "banners:active"
const bannerCacheTTL = 10 * time.Minute

type BannerService struct {
	Banners *repositories.BannerRepository
	Cache   *utils.Cache
}

func NewBannerService(banners *repositories.BannerRepository) *BannerService {
	return &BannerService{Banners: banners, Cache: utils.NewCache()}
}

// ListActive is the storefront homepage hero banners — read on every
// visit to the homepage, changes only when an admin edits banners.
func (s *BannerService) ListActive() ([]models.Banner, error) {
	if cached, ok := s.Cache.Get(bannerActiveCacheKey); ok {
		return cached.([]models.Banner), nil
	}
	banners, err := s.Banners.FindActiveOrdered()
	if err != nil {
		return nil, err
	}
	s.Cache.Set(bannerActiveCacheKey, banners, bannerCacheTTL)
	return banners, nil
}

// ListAll is the admin's own banner management view — deliberately NOT
// cached, same reasoning as CategoryService.ListAdmin: an admin editing
// banners should always see current state immediately.
func (s *BannerService) ListAll() ([]models.Banner, error) {
	return s.Banners.FindAllOrdered()
}

func (s *BannerService) GetByID(id uint) (*models.Banner, error) {
	return s.Banners.FindByID(id)
}

func (s *BannerService) Create(b *models.Banner) error {
	if err := s.Banners.Create(b); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}

func (s *BannerService) Update(b *models.Banner) error {
	if err := s.Banners.Update(b); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}

func (s *BannerService) Delete(id uint) error {
	if err := s.Banners.Delete(id); err != nil {
		return err
	}
	s.Cache.Clear()
	return nil
}
