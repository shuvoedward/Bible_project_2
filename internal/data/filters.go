package data

import (
	"shuvoedward/Bible_project/internal/validator"
	"slices"
	"strings"
)

type Filters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func (f Filters) limit() int {
	return f.PageSize
}

func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

func (f Filters) sortColumn() string {
	if slices.Contains(f.SortSafeList, f.Sort) {
		return strings.TrimPrefix(f.Sort, "-")
	}

	panic("unsafe sort parameter: " + f.Sort)
}

func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// Validate performs generic pagination validation
// Returns validator with errors if invalid
func (f *Filters) Validate(v *validator.Validator) {
	// Generic pagination rules (same for all endpoint)
	v.Check(f.Page > 0, "page", "must be atleast 1")
	v.Check(f.Page <= 10000, "page", "must be at most 10000")
	v.Check(f.PageSize > 0, "page_size", "must be at least 1")
	v.Check(f.PageSize <= 100, "page_size", "must be at most 100")
}

// ValidateSort checks if sort parameter is in safelist
func (f *Filters) ValidateSort(v *validator.Validator) {
	if len(f.SortSafeList) > 0 {
		if !slices.Contains(f.SortSafeList, f.Sort) {
			v.AddError("sort", "invalid sort value")
		}
	}
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitzero"`
	PageSize     int `json:"page_size,omitzero"`
	FirstPage    int `json:"first_page,omitzero"`
	LastPage     int `json:"last_page,omitzero"`
	TotalRecords int `json:"total_records,omitzero"`
}

func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		// Note that we return an empty Metadata struct if there are no records.
		return Metadata{}
	}

	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     (totalRecords + pageSize - 1) / pageSize,
		TotalRecords: totalRecords,
	}
}
