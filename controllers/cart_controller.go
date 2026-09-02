package controllers

import (
	"errors"
	"strconv"

	"bubblewhite-backend/middlewares"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type CartController struct {
	Service *services.CartService
}

func NewCartController(s *services.CartService) *CartController {
	return &CartController{Service: s}
}

// GET /api/customer/cart — requires customer auth
func (ctrl *CartController) List(c *gin.Context) {
	items, err := ctrl.Service.List(middlewares.CurrentCustomerID(c))
	if err != nil {
		utils.InternalError(c, "failed to fetch cart")
		return
	}
	utils.OK(c, items)
}

type addCartItemInput struct {
	ProductID string `json:"productId" validate:"required"`
	Size      string `json:"size"`
	Image     string `json:"image"` // the specific product image the customer was previewing, if any
	Quantity  int    `json:"quantity"`
}

// POST /api/customer/cart — requires customer auth
func (ctrl *CartController) Add(c *gin.Context) {
	var in addCartItemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}
	if in.Quantity <= 0 {
		in.Quantity = 1
	}

	if err := ctrl.Service.AddItem(middlewares.CurrentCustomerID(c), in.ProductID, in.Size, in.Image, in.Quantity); err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			utils.NotFound(c, "product not found")
			return
		}
		utils.InternalError(c, "failed to add to cart")
		return
	}
	utils.Created(c, gin.H{"message": "added to cart"})
}

type updateCartItemInput struct {
	Quantity int `json:"quantity" validate:"required"`
}

// PUT /api/customer/cart/:id — requires customer auth
func (ctrl *CartController) UpdateQuantity(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid cart item id")
		return
	}

	var in updateCartItemInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	if err := ctrl.Service.UpdateQuantity(middlewares.CurrentCustomerID(c), uint(id), in.Quantity); err != nil {
		utils.NotFound(c, "cart item not found")
		return
	}
	utils.OK(c, gin.H{"message": "updated"})
}

// DELETE /api/customer/cart/:id — requires customer auth
func (ctrl *CartController) Remove(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid cart item id")
		return
	}
	if err := ctrl.Service.RemoveItem(middlewares.CurrentCustomerID(c), uint(id)); err != nil {
		utils.NotFound(c, "cart item not found")
		return
	}
	utils.NoContent(c)
}

// DELETE /api/customer/cart — requires customer auth
func (ctrl *CartController) Clear(c *gin.Context) {
	if err := ctrl.Service.Clear(middlewares.CurrentCustomerID(c)); err != nil {
		utils.InternalError(c, "failed to clear cart")
		return
	}
	utils.NoContent(c)
}
