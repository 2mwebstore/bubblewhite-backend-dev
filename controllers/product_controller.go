package controllers

import (
	"strconv"
	"strings"

	"bubblewhite-backend/models"
	"bubblewhite-backend/repositories"
	"bubblewhite-backend/services"
	"bubblewhite-backend/utils"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	Service *services.ProductService
}

func NewProductController(s *services.ProductService) *ProductController {
	return &ProductController{Service: s}
}

// GET /api/products
// Query: category, search, featured=true, sortBy=price|name|created_at, sortDir=asc|desc, page, pageSize
func (ctrl *ProductController) List(c *gin.Context) {
	page := utils.ParsePageParams(c)

	filter := repositories.ProductFilter{
		Category: c.Query("category"),
		Search:   c.Query("search"),
		SortBy:   c.Query("sortBy"),
		SortDir:  utils.SortDirection(c.Query("sortDir")),
	}
	if c.Query("featured") == "true" {
		t := true
		filter.Featured = &t
	}

	products, total, err := ctrl.Service.List(filter, page)
	if err != nil {
		utils.InternalError(c, "failed to fetch products")
		return
	}

	utils.OKWithMeta(c, products, page.BuildMeta(total))
}

// GET /api/products/:id
func (ctrl *ProductController) GetByID(c *gin.Context) {
	product, err := ctrl.Service.GetByID(c.Param("id"))
	if err != nil {
		utils.NotFound(c, "product not found")
		return
	}
	utils.OK(c, product)
}

// GET /api/products/:id/related?limit=4
func (ctrl *ProductController) Related(c *gin.Context) {
	product, err := ctrl.Service.GetByID(c.Param("id"))
	if err != nil {
		utils.NotFound(c, "product not found")
		return
	}

	limit := 4
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 20 {
		limit = l
	}
	related, err := ctrl.Service.Related(product, limit)
	if err != nil {
		utils.InternalError(c, "failed to fetch related products")
		return
	}
	utils.OK(c, related)
}

// productInput is the admin create/update payload.
type productInput struct {
	ID          string   `json:"id" validate:"required"`
	Name        string   `json:"name" validate:"required"`
	Price       float64  `json:"price" validate:"required,gte=0"`
	CompareAt   *float64 `json:"compareAt"`
	Category    string   `json:"category" validate:"required"`
	Badge       *string  `json:"badge"`
	Featured    bool     `json:"featured"`
	Sizes       []string `json:"sizes"`
	Images      []string `json:"images"`
	Description string   `json:"description"`
}

// POST /api/admin/products (requires product.create permission)
func (ctrl *ProductController) Create(c *gin.Context) {
	var in productInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}
	if errs, _ := utils.ValidateStruct(in); errs != nil {
		utils.FailWithErrors(c, errs)
		return
	}

	// The ID is an admin-chosen primary key (not auto-generated), so a
	// collision is a normal, expectable mistake — check for it up front and
	// return a friendly field-level error instead of letting it fall through
	// to a raw "duplicate primary key" database error.
	if _, err := ctrl.Service.GetByID(in.ID); err == nil {
		utils.FailWithErrors(c, map[string]string{"id": "ID នេះមានរួចហើយ សូមប្រើលេខសម្គាល់ផ្សេង"})
		return
	}

	product := models.Product{
		ID:          in.ID,
		Name:        in.Name,
		Price:       in.Price,
		CompareAt:   in.CompareAt,
		CategoryID:  in.Category,
		Badge:       in.Badge,
		Featured:    in.Featured,
		Sizes:       models.JSONColumn[[]string]{Data: in.Sizes},
		Images:      models.JSONColumn[[]string]{Data: in.Images},
		Description: in.Description,
	}

	if err := ctrl.Service.Create(&product); err != nil {
		// Fallback for the rare race where two requests pass the pre-check
		// at the same moment — the database's own primary key constraint
		// still catches it, so translate that into the same friendly
		// message instead of a generic 500.
		if isDuplicateKeyError(err) {
			utils.FailWithErrors(c, map[string]string{"id": "ID នេះមានរួចហើយ សូមប្រើលេខសម្គាល់ផ្សេង"})
			return
		}
		utils.InternalError(c, "failed to create product")
		return
	}
	utils.Created(c, product)
}

// isDuplicateKeyError reports whether err looks like a MySQL primary/unique
// key violation (error 1062), without importing the MySQL driver's error
// types directly.
func isDuplicateKeyError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "1062") || strings.Contains(msg, "duplicate entry")
}

// PUT /api/admin/products/:id (requires product.update permission)
func (ctrl *ProductController) Update(c *gin.Context) {
	id := c.Param("id")

	product, err := ctrl.Service.GetByID(id)
	if err != nil {
		utils.NotFound(c, "product not found")
		return
	}

	var in productInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "invalid request body")
		return
	}

	product.Name = in.Name
	product.Price = in.Price
	product.CompareAt = in.CompareAt
	product.CategoryID = in.Category
	product.Badge = in.Badge
	product.Featured = in.Featured
	product.Sizes = models.JSONColumn[[]string]{Data: in.Sizes}
	product.Images = models.JSONColumn[[]string]{Data: in.Images}
	product.Description = in.Description

	if err := ctrl.Service.Update(product); err != nil {
		utils.InternalError(c, "failed to update product")
		return
	}
	utils.OK(c, product)
}

// DELETE /api/admin/products/:id (requires product.delete permission)
func (ctrl *ProductController) Delete(c *gin.Context) {
	if err := ctrl.Service.Delete(c.Param("id")); err != nil {
		utils.InternalError(c, "failed to delete product")
		return
	}
	utils.NoContent(c)
}
