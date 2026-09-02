package controllers

import (
	"strconv"

	"bubblewhite-backend/models"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type BannerController struct {
	Service *services.BannerService
}

func NewBannerController(s *services.BannerService) *BannerController {
	return &BannerController{Service: s}
}

// GET /api/banners — public, powers the storefront home page hero carousel.
func (ctrl *BannerController) ListActive(c *gin.Context) {
	banners, err := ctrl.Service.ListActive()
	if err != nil {
		utils.InternalError(c, "failed to fetch banners")
		return
	}
	utils.OK(c, banners)
}

// GET /api/admin/banners (requires banner.view) — includes inactive ones too.
func (ctrl *BannerController) List(c *gin.Context) {
	banners, err := ctrl.Service.ListAll()
	if err != nil {
		utils.InternalError(c, "failed to fetch banners")
		return
	}
	utils.OK(c, banners)
}

type bannerInput struct {
	ImageURL  string `json:"imageUrl" validate:"required"`
	Alt       string `json:"alt"`
	LinkURL   string `json:"linkUrl"`
	SortOrder int    `json:"sortOrder"`
	IsActive  *bool  `json:"isActive"`
}

// POST /api/admin/banners (requires banner.create)
func (ctrl *BannerController) Create(c *gin.Context) {
	var in bannerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	banner := models.Banner{
		ImageURL:  in.ImageURL,
		Alt:       in.Alt,
		LinkURL:   in.LinkURL,
		SortOrder: in.SortOrder,
		IsActive:  isActive,
	}
	if err := ctrl.Service.Create(&banner); err != nil {
		utils.InternalError(c, "failed to create banner")
		return
	}
	utils.Created(c, banner)
}

// PUT /api/admin/banners/:id (requires banner.update)
func (ctrl *BannerController) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid banner id")
		return
	}

	banner, err := ctrl.Service.GetByID(uint(id))
	if err != nil {
		utils.NotFound(c, "banner not found")
		return
	}

	var in bannerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	banner.ImageURL = in.ImageURL
	banner.Alt = in.Alt
	banner.LinkURL = in.LinkURL
	banner.SortOrder = in.SortOrder
	if in.IsActive != nil {
		banner.IsActive = *in.IsActive
	}

	if err := ctrl.Service.Update(banner); err != nil {
		utils.InternalError(c, "failed to update banner")
		return
	}
	utils.OK(c, banner)
}

// DELETE /api/admin/banners/:id (requires banner.delete)
func (ctrl *BannerController) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		utils.BadRequest(c, "invalid banner id")
		return
	}
	if err := ctrl.Service.Delete(uint(id)); err != nil {
		utils.InternalError(c, "failed to delete banner")
		return
	}
	utils.NoContent(c)
}
