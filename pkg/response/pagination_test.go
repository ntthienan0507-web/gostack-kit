package response

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePaginationParams_Defaults(t *testing.T) {
	p, limit, offset := NormalizePaginationParams(PaginationParams{})

	assert.Equal(t, 1, p.Page)
	assert.Equal(t, 20, p.PageSize)
	assert.Equal(t, int32(20), limit)
	assert.Equal(t, int32(0), offset)
}

func TestNormalizePaginationParams_ValidValues(t *testing.T) {
	p, limit, offset := NormalizePaginationParams(PaginationParams{Page: 3, PageSize: 10})

	assert.Equal(t, 3, p.Page)
	assert.Equal(t, 10, p.PageSize)
	assert.Equal(t, int32(10), limit)
	assert.Equal(t, int32(20), offset)
}

func TestNormalizePaginationParams_NegativePage(t *testing.T) {
	p, _, offset := NormalizePaginationParams(PaginationParams{Page: -1, PageSize: 10})

	assert.Equal(t, 1, p.Page)
	assert.Equal(t, int32(0), offset)
}

func TestNormalizePaginationParams_ZeroPage(t *testing.T) {
	p, _, _ := NormalizePaginationParams(PaginationParams{Page: 0})

	assert.Equal(t, 1, p.Page)
}

func TestNormalizePaginationParams_NegativePageSize(t *testing.T) {
	p, limit, _ := NormalizePaginationParams(PaginationParams{Page: 1, PageSize: -5})

	assert.Equal(t, 20, p.PageSize)
	assert.Equal(t, int32(20), limit)
}

func TestNormalizePaginationParams_ExceedsMaxPageSize(t *testing.T) {
	p, limit, _ := NormalizePaginationParams(PaginationParams{Page: 1, PageSize: 200})

	assert.Equal(t, 20, p.PageSize)
	assert.Equal(t, int32(20), limit)
}

func TestNormalizePaginationParams_BoundaryPageSize100(t *testing.T) {
	p, limit, _ := NormalizePaginationParams(PaginationParams{Page: 1, PageSize: 100})

	assert.Equal(t, 100, p.PageSize)
	assert.Equal(t, int32(100), limit)
}

func TestNormalizePaginationParams_PageSize101ExceedsMax(t *testing.T) {
	p, limit, _ := NormalizePaginationParams(PaginationParams{Page: 1, PageSize: 101})

	assert.Equal(t, 20, p.PageSize)
	assert.Equal(t, int32(20), limit)
}
