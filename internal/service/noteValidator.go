package service

import (
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

// NoteValidtor handles note validation logic
type NoteValidator struct {
	BibleValidator *BibleValidator
}

func NewNoteValidator(books map[string]struct{}) *NoteValidator {
	return &NoteValidator{
		BibleValidator: NewBibleValidator(books)}
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

	nv.BibleValidator.ValidateBibleLocation(
		v,
		location.Book,
		location.Chapter,
		location.StartVerse,
		location.EndVerse,
		location.StartOffset,
		location.EndOffset,
	)
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

	v.Check(location.NoteID > 0, "note_id", "must be a valid id")

	nv.BibleValidator.ValidateBibleLocation(
		v,
		location.Book,
		location.Chapter,
		location.StartVerse,
		location.EndVerse,
		location.StartOffset,
		location.EndOffset,
	)

	return v
}

func (nv *NoteValidator) ValidateDeleteLink(noteID, locationID, userID int64) *validator.Validator {
	v := validator.New()

	v.Check(noteID > 0, "note_id", "must provide valid id")
	v.Check(locationID > 0, "location_id", "must provide valid id")
	v.Check(userID > 0, "user_id", "must provide valid id")

	return v
}
