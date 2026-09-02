package services

import (
	"errors"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/utils"

	"gorm.io/gorm"
)

var ErrPaymentMethodNotFound = errors.New("payment method not found")

// Shared GORM scopes — same pattern as OrderFilter.Scope() elsewhere in
// this codebase.
func orderBySortOrder(db *gorm.DB) *gorm.DB {
	return db.Order("sort_order ASC")
}

func enabledOnly(db *gorm.DB) *gorm.DB {
	return db.Where("enabled = ?", true)
}

func codeEquals(code string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("code = ?", code)
	}
}

type PaymentMethodService struct {
	PaymentMethods *repositories.PaymentMethodRepository
}

func NewPaymentMethodService(repo *repositories.PaymentMethodRepository) *PaymentMethodService {
	return &PaymentMethodService{PaymentMethods: repo}
}

// ListAdmin returns every payment method (enabled or not), ordered for
// display — sortOrder first, matching however the admin has arranged them.
// The same sort scope is applied to both Count and the actual page fetch
// for consistency, though sort order doesn't affect the total either way.
func (s *PaymentMethodService) ListAdmin(page utils.PageParams) ([]models.PaymentMethod, int64, error) {
	total, err := s.PaymentMethods.Count()
	if err != nil {
		return nil, 0, err
	}
	methods, err := s.PaymentMethods.FindAll(orderBySortOrder, page.Scope())
	if err != nil {
		return nil, 0, err
	}
	return methods, total, nil
}

// ListEnabled returns only enabled payment methods, ordered for display —
// what the storefront's checkout selector actually shows. No admin auth
// needed for this one; it's public, matching GET /api/settings.
func (s *PaymentMethodService) ListEnabled() ([]models.PaymentMethod, error) {
	return s.PaymentMethods.FindAll(enabledOnly, orderBySortOrder)
}

// IsEnabled checks whether a given payment method CODE (e.g. "bakong") is
// currently enabled — the real, server-side gate used by
// OrderService.Checkout / InitiatePPCBankCheckout. Replaces the old
// Settings.*PaymentEnabled booleans this table migrated from. A code with
// no matching row (shouldn't normally happen — seed creates all three) is
// treated as disabled rather than erroring, since "doesn't exist" and
// "not enabled" should have the same practical effect on checkout.
func (s *PaymentMethodService) IsEnabled(code string) (bool, error) {
	methods, err := s.PaymentMethods.FindAll(codeEquals(code))
	if err != nil {
		return false, err
	}
	if len(methods) == 0 {
		return false, nil
	}
	return methods[0].Enabled, nil
}

func (s *PaymentMethodService) GetByID(id uint) (*models.PaymentMethod, error) {
	pm, err := s.PaymentMethods.FindByID(id)
	if err != nil {
		return nil, ErrPaymentMethodNotFound
	}
	return pm, nil
}

type PaymentMethodPatch struct {
	Name      string
	ImageURL  string
	Enabled   bool
	IsPrimary bool
	SortOrder int
}

// Update applies an admin edit — Code is NEVER editable (it's the link to
// real backend behavior, not display data). If IsPrimary is being turned
// on, every other row's IsPrimary is cleared first in the same operation,
// so exactly one payment method is ever primary — the checkout page needs
// a single unambiguous default, not "whichever one happens to be first."
func (s *PaymentMethodService) Update(id uint, patch PaymentMethodPatch) (*models.PaymentMethod, error) {
	pm, err := s.PaymentMethods.FindByID(id)
	if err != nil {
		return nil, ErrPaymentMethodNotFound
	}

	pm.Name = patch.Name
	pm.ImageURL = patch.ImageURL
	pm.Enabled = patch.Enabled
	pm.IsPrimary = patch.IsPrimary
	pm.SortOrder = patch.SortOrder

	if patch.IsPrimary {
		if err := s.PaymentMethods.ClearPrimary(id); err != nil {
			return nil, err
		}
	}

	if err := s.PaymentMethods.DB.Save(pm).Error; err != nil {
		return nil, err
	}
	return pm, nil
}
