package service

import (
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

// NoteValidtor handles note validation logic
type NoteValidator struct {
	books map[string]struct{} // Bible books for validation
}

func NewNoteValidator(books map[string]struct{}) *NoteValidator {
	return &NoteValidator{books: books}
}

// ValidateNoteCreation based on note type
// Returns the validator - caller checks v.Valid()
func (nv *NoteValidator) ValidateNoteCreation(content *data.NoteContent, location *data.NoteLocation) *validator.Validator {
	v := validator.New() // pass validator through NoteValidator struct?

	switch content.NoteType {
	case "GENERAL":
		nv.validateGeneralNote(v, content)
	case "BIBLE", "CROSS_REFERENCE":
		nv.validateLocatedNote(v, content, location)
	default:
		v.AddError("note_type", "must be GENERAL, BIBLE, or CROSS_REFERENCE")
	}

	return v
}

// validateGeneralNote validates GENERAL notes
func (nv *NoteValidator) validateGeneralNote(v *validator.Validator, content *data.NoteContent) {
	v.Check(content.UserID > 0, "user_id", "must be valid")
	v.Check(content.Title != "", "title", "must be provided")
	v.Check(content.Content != "", "content", "must be provided")
}

// validateLocatedNote validate BIBLE and CROSS_REFERENCE notes
func (nv *NoteValidator) validateLocatedNote(
	v *validator.Validator,
	content *data.NoteContent,
	location *data.NoteLocation) {

	v.Check(content.Content != "", "content", "must be provided")

	// CROSS_REFERENCE notes should not have titles
	if content.NoteType == "CROSS_REFERENCE" {
		v.Check(content.Title == "", "title", "CROSS_REFERENCE notes cannot have title")
	}

	nv.ValidateLocation(v, location)
}

func (nv *NoteValidator) ValidateLocation(v *validator.Validator, location *data.NoteLocation) {
	if location == nil {
		v.AddError("location", "must be provided for BIBLE and CROSS_REFERENCE notes")
		return
	}

	// Validate book
	v.Check(location.Book != "", "book", "must be provided")
	if location.Book != "" {
		_, exists := nv.books[location.Book]
		v.Check(exists, "book", "must be a valid Bible book")
	}

	// Validate chapter and verses
	v.Check(location.Chapter > 0 && location.Chapter <= 150, "chapter", "must be between 1 and 150")
	v.Check(location.StartVerse > 0 && location.StartVerse <= 176, "start_verse", "must be between 1 and 176")
	v.Check(location.EndVerse > 0 && location.EndVerse <= 176, "end_verse", "must be between 1 and 176")
	v.Check(location.StartVerse <= location.EndVerse, "verse", "start verse must be less than or equal to end verse")

	// Validate offsets
	v.Check(location.StartOffset >= 0, "start_offset", "cannot be negative")
	v.Check(location.EndOffset >= 0, "end_offset", "cannot be negative")

	// For single verse, start offset must be before end offset
	if location.StartVerse == location.EndVerse && location.EndOffset != 0 {
		v.Check(location.StartOffset < location.EndOffset, "offset", "start offset must be less than end offset")
	}
}

// ValidateUpdateNote validates note updates
// Returns the validator, caller checks v.Valid()
func (nv *NoteValidator) ValidateUpdateNote(content *data.NoteContent) *validator.Validator {
	v := validator.New()

	nv.validateGeneralNote(v, content)

	v.Check(content.ID > 0, "note_id", "must be greater than zero")

	if content.NoteType == "CROSS_REFERENCE" {
		v.Check(content.Title == "", "title", "CROSS_REFERENCE note, title not allowed")
	}

	return v
}

func (nv *NoteValidator) ValidateNoteLink(location *data.NoteInputLocation) *validator.Validator {
	v := validator.New()

	//
	v.Check(location.NoteID > 0, "note_id", "must be a valid id")

	// Validate book
	v.Check(location.Book != "", "book", "must be provided")
	if location.Book != "" {
		_, exists := nv.books[location.Book]
		v.Check(exists, "book", "must be a valid Bible book")
	}

	// Validate chapter and verses
	v.Check(location.Chapter > 0 && location.Chapter <= 150, "chapter", "must be between 1 and 150")
	v.Check(location.StartVerse > 0 && location.StartVerse <= 176, "start_verse", "must be between 1 and 176")
	v.Check(location.EndVerse > 0 && location.EndVerse <= 176, "end_verse", "must be between 1 and 176")
	v.Check(location.StartVerse <= location.EndVerse, "verse", "start verse must be less than or equal to end verse")

	// Validate offsets
	v.Check(location.StartOffset >= 0, "start_offset", "cannot be negative")
	v.Check(location.EndOffset >= 0, "end_offset", "cannot be negative")

	// For single verse, start offset must be before end offset
	if location.StartVerse == location.EndVerse && location.EndOffset != 0 {
		v.Check(location.StartOffset < location.EndOffset, "offset", "start offset must be less than end offset")
	}

	return v

}

func (nv *NoteValidator) ValidateDeleteLink(noteID, locationID, userID int64) *validator.Validator {
	v := validator.New()

	v.Check(noteID > 0, "note_id", "must provide valid id")
	v.Check(locationID > 0, "location_id", "must provide valid id")
	v.Check(userID > 0, "user_id", "must provide valid id")

	return v
}
