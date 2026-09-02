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
