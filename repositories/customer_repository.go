package repositories

import (
	"bubblewhite-backend/models"

	"gorm.io/gorm"
)

type CustomerRepository struct {
	*BaseRepository[models.Customer]
}

func NewCustomerRepository(db *gorm.DB) *CustomerRepository {
	return &CustomerRepository{BaseRepository: NewBaseRepository[models.Customer](db)}
}

func (r *CustomerRepository) FindByEmail(email string) (*models.Customer, error) {
	var customer models.Customer
	if err := r.DB.Where("email = ?", email).First(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}

func (r *CustomerRepository) FindByPhone(phone string) (*models.Customer, error) {
	var customer models.Customer
	if err := r.DB.Where("phone = ?", phone).First(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}

func (r *CustomerRepository) FindByGoogleID(googleID string) (*models.Customer, error) {
	var customer models.Customer
	if err := r.DB.Where("google_id = ?", googleID).First(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}

func (r *CustomerRepository) FindByFacebookID(facebookID string) (*models.Customer, error) {
	var customer models.Customer
	if err := r.DB.Where("facebook_id = ?", facebookID).First(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}

func (r *CustomerRepository) FindByTelegramID(telegramID string) (*models.Customer, error) {
	var customer models.Customer
	if err := r.DB.Where("telegram_id = ?", telegramID).First(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}
