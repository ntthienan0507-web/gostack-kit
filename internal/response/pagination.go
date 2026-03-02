package response

import "math"

// PaginationParams parsed from query string: ?page=1&page_size=20
type PaginationParams struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// Pagination is the metadata returned in paginated responses.
type Pagination struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
	PageNext   int   `json:"page_next"`
	PagePrev   int   `json:"page_prev"`
}

// PaginatedResult wraps items + pagination for Success() auto-detection.
type PaginatedResult struct {
	Items      interface{}
	Pagination Pagination
}

// NewPagination builds pagination metadata from total count and params.
func NewPagination(total int64, params PaginationParams) Pagination {
	totalPages := int(math.Ceil(float64(total) / float64(params.PageSize)))

	pageNext := -1
	if params.Page < totalPages {
		pageNext = params.Page + 1
	}

	pagePrev := -1
	if params.Page > 1 {
		pagePrev = params.Page - 1
	}

	return Pagination{
		Total:      total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
		PageNext:   pageNext,
		PagePrev:   pagePrev,
	}
}

// NormalizePaginationParams applies defaults and bounds to pagination params.
// Returns (params, limit, offset).
func NormalizePaginationParams(p PaginationParams) (PaginationParams, int32, int32) {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 || p.PageSize > 100 {
		p.PageSize = 20
	}
	limit := int32(p.PageSize)
	offset := int32((p.Page - 1) * p.PageSize)
	return p, limit, offset
}
