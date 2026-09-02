package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PageParams is the parsed ?page=&pageSize= query pair.
type PageParams struct {
	Page     int
	PageSize int
}

// PageMeta is what gets returned in the response envelope's "meta" field.
type PageMeta struct {
	Page      int   `json:"page"`
	PageSize  int   `json:"pageSize"`
	Total     int64 `json:"total"`
	TotalPage int   `json:"totalPage"`
}

// ParsePageParams reads ?page= (default 1) and ?pageSize= (default 10, max 100).
func ParsePageParams(c *gin.Context) PageParams {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	return PageParams{Page: page, PageSize: pageSize}
}

// Scope returns a GORM scope that applies this page's LIMIT/OFFSET.
func (p PageParams) Scope() func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset((p.Page - 1) * p.PageSize).Limit(p.PageSize)
	}
}

// BuildMeta turns a total row count into a PageMeta for the response envelope.
func (p PageParams) BuildMeta(total int64) PageMeta {
	totalPage := int(total) / p.PageSize
	if int(total)%p.PageSize != 0 {
		totalPage++
	}
	return PageMeta{Page: p.Page, PageSize: p.PageSize, Total: total, TotalPage: totalPage}
}
