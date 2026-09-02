package services

import (
	"errors"
	"mime/multipart"
	"strings"

	"bubblewhite-backend/utils"
)

type UploadService struct{}

func NewUploadService() *UploadService {
	return &UploadService{}
}

// UploadProductImage uploads to the "products" folder in the R2 bucket.
func (s *UploadService) UploadProductImage(fh *multipart.FileHeader) (*utils.UploadedFile, error) {
	return utils.UploadImage(fh, "products")
}

// UploadCategoryImage uploads to the "categories" folder in the R2 bucket.
func (s *UploadService) UploadCategoryImage(fh *multipart.FileHeader) (*utils.UploadedFile, error) {
	return utils.UploadImage(fh, "categories")
}

// UploadSiteImage uploads to the "site" folder — logo, banners, etc.
func (s *UploadService) UploadSiteImage(fh *multipart.FileHeader) (*utils.UploadedFile, error) {
	return utils.UploadImage(fh, "site")
}

// UploadBannerImage uploads to the "banners" folder in the R2 bucket.
func (s *UploadService) UploadBannerImage(fh *multipart.FileHeader) (*utils.UploadedFile, error) {
	return utils.UploadImage(fh, "banners")
}

// deleteInFolder removes an object, but only if its key actually lives under
// the expected folder — stops a caller with e.g. product.update from being
// able to delete arbitrary keys elsewhere in the bucket by passing any key.
func deleteInFolder(key, folder string) error {
	if !strings.HasPrefix(key, folder+"/") {
		return errors.New("key does not belong to this upload type")
	}
	return utils.DeleteImage(key)
}

// DeleteProductImage removes a "products/..." object — used when an admin
// removes an image they'd already uploaded but not yet saved onto a product
// (or replaces one), so it doesn't sit orphaned in the bucket.
func (s *UploadService) DeleteProductImage(key string) error {
	return deleteInFolder(key, "products")
}

func (s *UploadService) DeleteCategoryImage(key string) error {
	return deleteInFolder(key, "categories")
}

func (s *UploadService) DeleteSiteImage(key string) error {
	return deleteInFolder(key, "site")
}

func (s *UploadService) DeleteBannerImage(key string) error {
	return deleteInFolder(key, "banners")
}

// UploadPaymentMethodImage uploads to the "payment-methods" folder — logos
// for the checkout payment method selector.
func (s *UploadService) UploadPaymentMethodImage(fh *multipart.FileHeader) (*utils.UploadedFile, error) {
	return utils.UploadImage(fh, "payment-methods")
}

func (s *UploadService) DeletePaymentMethodImage(key string) error {
	return deleteInFolder(key, "payment-methods")
}
