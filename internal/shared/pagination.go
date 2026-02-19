package shared

import "math"

type Pagination struct {
	CurrentPage int
	PerPage     int
	Total       int64
	TotalPages  int
}

func NewPagination(page, perPage int, total int64) Pagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}
	return Pagination{
		CurrentPage: page,
		PerPage:     perPage,
		Total:       total,
		TotalPages:  totalPages,
	}
}

func (p Pagination) Offset() int {
	return (p.CurrentPage - 1) * p.PerPage
}

func (p Pagination) IsFirst() bool {
	return p.CurrentPage <= 1
}

func (p Pagination) IsLast() bool {
	return p.CurrentPage >= p.TotalPages
}

func (p Pagination) Pages() []int {
	pages := make([]int, 0, p.TotalPages)
	for i := 1; i <= p.TotalPages; i++ {
		pages = append(pages, i)
	}
	return pages
}
