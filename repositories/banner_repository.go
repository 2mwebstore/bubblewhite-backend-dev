package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type BannerRepository struct {
	*BaseRepository[models.Banner]
}

func NewBannerRepository(db *gorm.DB) *BannerRepository {
	return &BannerRepository{BaseRepository: NewBaseRepository[models.Banner](db)}
}

// FindActiveOrdered returns active banners in display order — what the
// storefront's hero carousel actually renders.
func (r *BannerRepository) FindActiveOrdered() ([]models.Banner, error) {
	var banners []models.Banner
	err := r.DB.Where("is_active = ?", true).Order("sort_order ASC, id ASC").Find(&banners).Error
	return banners, err
}

// FindAllOrdered returns every banner (active or not) for the admin list.
func (r *BannerRepository) FindAllOrdered() ([]models.Banner, error) {
	var banners []models.Banner
	err := r.DB.Order("sort_order ASC, id ASC").Find(&banners).Error
	return banners, err
}
