package services

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
)

type BannerService struct {
	Banners *repositories.BannerRepository
}

func NewBannerService(banners *repositories.BannerRepository) *BannerService {
	return &BannerService{Banners: banners}
}

func (s *BannerService) ListActive() ([]models.Banner, error) {
	return s.Banners.FindActiveOrdered()
}

func (s *BannerService) ListAll() ([]models.Banner, error) {
	return s.Banners.FindAllOrdered()
}

func (s *BannerService) GetByID(id uint) (*models.Banner, error) {
	return s.Banners.FindByID(id)
}

func (s *BannerService) Create(b *models.Banner) error {
	return s.Banners.Create(b)
}

func (s *BannerService) Update(b *models.Banner) error {
	return s.Banners.Update(b)
}

func (s *BannerService) Delete(id uint) error {
	return s.Banners.Delete(id)
}
