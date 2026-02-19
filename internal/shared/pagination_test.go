package shared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPagination(t *testing.T) {
	p := NewPagination(3, 20, 95)
	assert.Equal(t, 3, p.CurrentPage)
	assert.Equal(t, 20, p.PerPage)
	assert.Equal(t, int64(95), p.Total)
	assert.Equal(t, 5, p.TotalPages)
	assert.Equal(t, 40, p.Offset())
}

func TestPagination_FirstPage(t *testing.T) {
	p := NewPagination(1, 20, 50)
	assert.Equal(t, 0, p.Offset())
	assert.Equal(t, 3, p.TotalPages)
	assert.True(t, p.IsFirst())
	assert.False(t, p.IsLast())
}

func TestPagination_LastPage(t *testing.T) {
	p := NewPagination(3, 20, 50)
	assert.True(t, p.IsLast())
	assert.False(t, p.IsFirst())
}

func TestPagination_InvalidPage(t *testing.T) {
	p := NewPagination(0, 20, 50)
	assert.Equal(t, 1, p.CurrentPage)

	p2 := NewPagination(-5, 20, 50)
	assert.Equal(t, 1, p2.CurrentPage)
}

func TestPagination_Pages(t *testing.T) {
	p := NewPagination(1, 10, 30)
	assert.Equal(t, []int{1, 2, 3}, p.Pages())
}
