package services

import (
	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"

	"gorm.io/gorm"
)

type ContactService struct {
	Contacts *repositories.ContactRepository
}

func NewContactService(contacts *repositories.ContactRepository) *ContactService {
	return &ContactService{Contacts: contacts}
}

func (s *ContactService) Submit(msg *models.ContactMessage) error {
	return s.Contacts.Create(msg)
}

// List returns every contact message, newest first.
func (s *ContactService) List() ([]models.ContactMessage, error) {
	return s.Contacts.FindAll(func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at DESC")
	})
}

func (s *ContactService) MarkRead(id uint) error {
	msg, err := s.Contacts.FindByID(id)
	if err != nil {
		return err
	}
	msg.IsRead = true
	return s.Contacts.Update(msg)
}

func (s *ContactService) Delete(id uint) error {
	return s.Contacts.Delete(id)
}
