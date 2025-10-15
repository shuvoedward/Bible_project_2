package main

import (
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

func (app *application) validateLocationFilter(v *validator.Validator, filter *data.LocationFilters) {
	app.validateBook(v, filter.Book, filter.Chapter, filter.StartVerse, filter.EndVerse)
}

func (app *application) validateHighlight(v *validator.Validator, h *data.Highlight) {
	filters := data.LocationFilters{
		Book:       h.Book,
		Chapter:    h.Chapter,
		StartVerse: h.StartVerse,
		EndVerse:   h.EndVerse,
	}

	app.validateLocationFilter(v, &filters)

	app.validateUser(v, h.UserID)

	app.validateLocation(v, h.StartOffset, h.EndOffset)

	v.Check(h.Color != "", "color", "must be provided")
}

func (app *application) validateGeneralNote(v *validator.Validator, content *data.NoteContent) {
	app.validateUser(v, &content.UserID)
	v.Check(content.Title != "", "title", "must be provided")
	app.validateNoteContent(v, content.Content)
}

func (app *application) validateLocatedNote(v *validator.Validator, content *data.NoteContent, location *data.NoteLocation) {
	app.validateBook(v, location.Book, location.Chapter, location.StartVerse, location.EndVerse)
	app.validateUser(v, &content.ID)
	app.validateLocation(v, &location.StartOffset, &location.EndOffset)
	app.validateNoteContent(v, content.Content)
	if location.StartVerse == location.EndVerse {
		v.Check(location.StartOffset < location.EndOffset, "offset", "provide valid offset")
	}
	v.Check(location.StartVerse <= location.EndVerse, "verse", "provide valid verse number")
	if content.NoteType == "CROSS_REFERENCE" {
		v.Check(content.Title == "", "title", "CROSS_REFERENCE note, title not allowed")
	}
}

func (app *application) validateUser(v *validator.Validator, id *int64) {
	v.Check(id != nil, "userID", "must be provided")
	v.Check(*id >= 0, "userID", "can not be negative")
}

func (app *application) validateBook(v *validator.Validator, book string, chapter, startVerse, endVerse int) {
	v.Check(book != "", "book", "must be provided")
	if _, exists := app.books[book]; !exists {
		v.AddError("book", "must be a valid book")
	}

	v.Check(chapter > 0 || chapter < 151, "chapter", "must be between 1 and 150")
	v.Check(startVerse > 0 || startVerse < 177, "start verse", "must be between 1 and 176")
	v.Check(endVerse > 0 || endVerse < 177, "end verse", "must be between 1 and 176")
}

func (app *application) validateLocation(v *validator.Validator, startOffset, endOffset *int) {
	v.Check(startOffset != nil, "startOffSet", "must be provided")
	v.Check(*startOffset >= 0, "startOffSet", "can not be negative")

	v.Check(endOffset != nil, "endOffSet", "must be provided")
	v.Check(*endOffset >= 0, "endOffSet", "can not be negative")
}

func (app *application) validateNoteContent(v *validator.Validator, content string) {
	v.Check(content != "", "content", "must be provided")
}
