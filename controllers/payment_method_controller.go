package controllers

import (
	"strconv"

	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type PaymentMethodController struct {
	Service *services.PaymentMethodService
}

func NewPaymentMethodController(s *services.PaymentMethodService) *PaymentMethodController {
	return &PaymentMethodController{Service: s}
}

// GET /api/payment-methods — public. The storefront's checkout selector
// reads its options from here, same pattern as GET /api/settings.
func (ctrl *PaymentMethodController) ListPublic(c *gin.Context) {
	methods, err := ctrl.Service.ListEnabled()
	if err != nil {
		utils.InternalError(c, "failed to fetch payment methods")
		return
	}
	utils.OK(c, methods)
}

// GET /api/admin/payment-methods (requires settings.view)
func (ctrl *PaymentMethodController) List(c *gin.Context) {
	page := utils.ParsePageParams(c)
	methods, total, err := ctrl.Service.ListAdmin(page)
	if err != nil {
		utils.InternalError(c, "failed to fetch payment methods")
		return
	}
	utils.OKWithMeta(c, methods, page.BuildMeta(total))
}

// GET /api/admin/payment-methods/:id (requires settings.view)
func (ctrl *PaymentMethodController) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid payment method id")
		return
	}
	pm, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "payment method not found")
		return
	}
	utils.OK(c, pm)
}

type updatePaymentMethodInput struct {
	Name      string `json:"name" validate:"required"`
	ImageURL  string `json:"imageUrl"`
	Enabled   bool   `json:"enabled"`
	IsPrimary bool   `json:"isPrimary"`
	SortOrder int    `json:"sortOrder"`
}

// PUT /api/admin/payment-methods/:id (requires settings.update)
func (ctrl *PaymentMethodController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid payment method id")
		return
	}
	var in updatePaymentMethodInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	pm, err := ctrl.Service.Update(uint(id), services.PaymentMethodPatch{
		Name:      in.Name,
		ImageURL:  in.ImageURL,
		Enabled:   in.Enabled,
		IsPrimary: in.IsPrimary,
		SortOrder: in.SortOrder,
	})
	if err != nil {
		utils.NotFound(c, "payment method not found")
		return
	}
	utils.OK(c, pm)
}
