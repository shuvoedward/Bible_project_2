package main

import (
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

func (app *application) validateLocationFilter(v *validator.Validator, filter *data.LocationFilters) {
	v.Check(filter.Book != "", "book", "must be provided")
	if _, exists := app.books[filter.Book]; !exists {
		v.AddError("book", "must be a valid book")
	}

	v.Check(filter.Chapter > 0 || filter.Chapter < 151, "chapter", "must be between 1 and 150")
	v.Check(filter.StartVerse > 0 || filter.StartVerse < 177, "start verse", "must be between 1 and 176")
	v.Check(filter.EndVerse > 0 || filter.EndVerse < 177, "end verse", "must be between 1 and 176")
}

func (app *application) validateHighlight(v *validator.Validator, h *data.Highlight) {
	filters := data.LocationFilters{
		Book:       h.Book,
		Chapter:    h.Chapter,
		StartVerse: h.StartVerse,
		EndVerse:   h.EndVerse,
	}
	app.validateLocationFilter(v, &filters)

	v.Check(h.UserID != nil, "userID", "must be provided")
	v.Check(*h.UserID >= 0, "userID", "can not be negative")

	v.Check(h.StartOffset != nil, "startOffSet", "must be provided")
	v.Check(*h.StartOffset >= 0, "startOffSet", "can not be negative")

	v.Check(h.EndOffset != nil, "endOffSet", "must be provided")
	v.Check(*h.EndOffset >= 0, "endOffSet", "can not be negative")

	v.Check(h.Color != "", "color", "must be provided")
}
