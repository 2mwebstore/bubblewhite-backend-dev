package common

// PaginationQuery is the common shape of list-endpoint query params.
// Individual controllers parse their own extra filters on top of this.
type PaginationQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Search   string `form:"search"`
	SortBy   string `form:"sortBy"`
	SortDir  string `form:"sortDir"`
}
