package controllers

import (
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type UploadController struct {
	Service *services.UploadService
}

func NewUploadController(s *services.UploadService) *UploadController {
	return &UploadController{Service: s}
}

// POST /api/admin/uploads/products (multipart "file") (requires product.update)
func (ctrl *UploadController) UploadProductImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file provided (expected multipart field \"file\")")
		return
	}
	result, err := ctrl.Service.UploadProductImage(fh)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Created(c, result)
}

// POST /api/admin/uploads/categories (multipart "file") (requires category.update)
func (ctrl *UploadController) UploadCategoryImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file provided (expected multipart field \"file\")")
		return
	}
	result, err := ctrl.Service.UploadCategoryImage(fh)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Created(c, result)
}

// POST /api/admin/uploads/site (multipart "file") (requires settings.update)
// Used for the logo and any other site-wide image the admin panel manages.
func (ctrl *UploadController) UploadSiteImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file provided (expected multipart field \"file\")")
		return
	}
	result, err := ctrl.Service.UploadSiteImage(fh)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Created(c, result)
}

// POST /api/admin/uploads/banners (multipart "file") (requires banner.create)
func (ctrl *UploadController) UploadBannerImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file provided (expected multipart field \"file\")")
		return
	}
	result, err := ctrl.Service.UploadBannerImage(fh)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Created(c, result)
}

// POST /api/admin/uploads/payment-methods (multipart "file") (requires
// payment_method.update) — logos for the checkout payment method selector.
func (ctrl *UploadController) UploadPaymentMethodImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "no file provided (expected multipart field \"file\")")
		return
	}
	result, err := ctrl.Service.UploadPaymentMethodImage(fh)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Created(c, result)
}

// DELETE /api/admin/uploads/products?key=... (requires product.update)
// Deletes an image from R2 — used when an admin removes an image they'd
// already uploaded in the product form but hasn't (or won't) save onto the
// product, so nothing is left orphaned in the bucket.
func (ctrl *UploadController) DeleteProductImage(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "missing \"key\" query parameter")
		return
	}
	if err := ctrl.Service.DeleteProductImage(key); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}

// DELETE /api/admin/uploads/categories?key=... (requires category.update)
func (ctrl *UploadController) DeleteCategoryImage(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "missing \"key\" query parameter")
		return
	}
	if err := ctrl.Service.DeleteCategoryImage(key); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}

// DELETE /api/admin/uploads/site?key=... (requires settings.update)
func (ctrl *UploadController) DeleteSiteImage(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "missing \"key\" query parameter")
		return
	}
	if err := ctrl.Service.DeleteSiteImage(key); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}

// DELETE /api/admin/uploads/banners?key=... (requires banner.update)
func (ctrl *UploadController) DeleteBannerImage(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "missing \"key\" query parameter")
		return
	}
	if err := ctrl.Service.DeleteBannerImage(key); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}

// DELETE /api/admin/uploads/payment-methods?key=... (requires
// payment_method.update) — used both when an admin removes an image
// they'd already uploaded but not yet saved onto a payment method, and
// for the explicit "clear image" action once a logo IS saved (see
// admin/payment_method's removeImage — same pattern as
// categories.vue), so nothing is left orphaned in the bucket either way.
func (ctrl *UploadController) DeletePaymentMethodImage(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		utils.BadRequest(c, "missing \"key\" query parameter")
		return
	}
	if err := ctrl.Service.DeletePaymentMethodImage(key); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.NoContent(c)
}
