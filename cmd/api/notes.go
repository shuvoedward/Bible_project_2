package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"strings"
)

func (app *application) createNoteHandler(w http.ResponseWriter, r *http.Request) {
	var responseNote *data.LocatedNoteResponse
	var err error

	user := app.contextGetUser(r)

	if !app.noteRateLimiter.Allow(user.ID) {
		app.rateLimitExceededResponse(w, r)
		return
	}

	var input struct {
		Title       string `json:"title"`
		Content     string `json:"content"`
		NoteType    string `json:"note_type"`
		Book        string `json:"book"`
		Chapter     int    `json:"chapter"`
		StartVerse  int    `json:"start_verse"`
		EndVerse    int    `json:"end_verse"`
		StartOffset int    `json:"start_offset"`
		EndOffset   int    `json:"end_offset"`
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	input.Title = strings.TrimSpace(input.Title)

	content := &data.NoteContent{
		UserID:   user.ID,
		Title:    input.Title,
		Content:  input.Content,
		NoteType: input.NoteType,
	}

	location := &data.NoteLocation{
		Book:        input.Book,
		Chapter:     input.Chapter,
		StartVerse:  input.StartVerse,
		EndVerse:    input.EndVerse,
		StartOffset: input.StartOffset,
		EndOffset:   input.EndOffset,
	}

	// validation
	v := validator.New()

	switch content.NoteType {

	case "GENERAL":

		app.validateNoteContent(v, content.Content)
		app.validateGeneralNote(v, content)

		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		responseNote, err = app.models.Notes.InsertGeneral(content)

	case "BIBLE", "CROSS_REFERENCE":

		app.validateLocatedNote(v, content, location)
		if !v.Valid() {
			app.failedValidationResponse(w, r, v.Errors)
			return
		}

		responseNote, err = app.models.Notes.InsertLocated(content, location)

	default:
		v.AddError("note_type", "must provide a valid note type")
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateTitleGeneral):
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrLocationAlreadyLinked):
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrDuplicateContent):
			app.editConflictResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"note": responseNote}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	err = app.models.Notes.Delete(id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (app *application) updateNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		NoteType string `json:"note_type"`
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	input.Title = strings.TrimSpace(input.Title)
	// validation
	v := validator.New()

	content := &data.NoteContent{
		ID:       id,
		UserID:   user.ID,
		Title:    input.Title,
		Content:  input.Content,
		NoteType: input.NoteType,
	}

	v.Check(content.ID > 0, "note_id", "must be greater than zero")
	if content.NoteType == "CROSS_REFERENCE" {
		v.Check(content.Title == "", "title", "CROSS_REFERENCE note, title not allowed")
	}
	app.validateGeneralNote(v, content)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	responseNote, err := app.models.Notes.Update(content)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateTitleGeneral):
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrDuplicateContent):
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"note": responseNote}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) linkNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var input struct {
		ID int64 `json:"id"`
		//location
		Book        string `json:"book"`
		Chapter     int    `json:"chapter"`
		StartVerse  int    `json:"start_verse"`
		EndVerse    int    `json:"end_verse"`
		StartOffset int    `json:"start_offset"`
		EndOffset   int    `json:"end_offset"`
	}

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	input.ID = id

	v := validator.New()

	v.Check(input.ID > 0, "note_id", "must be a valid id")
	app.validateBook(v, input.Book, input.Chapter, input.StartVerse, input.EndVerse)
	app.validateLocation(v, &input.StartOffset, &input.EndOffset)
	app.validateUser(v, &user.ID)
	if input.StartVerse == input.EndVerse {
		if input.StartOffset != 0 && input.EndOffset != 0 {
			v.Check(input.StartOffset < input.EndOffset, "offset", "provide valid offset")
		}
	}
	v.Check(input.StartVerse <= input.EndVerse, "verse", "provide valid verse number")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	note := &data.NoteInputLocation{
		ID:          input.ID,
		UserID:      user.ID,
		Book:        input.Book,
		Chapter:     input.Chapter,
		StartVerse:  input.StartVerse,
		EndVerse:    input.EndVerse,
		StartOffset: input.StartOffset,
		EndOffset:   input.EndOffset,
	}

	responseNote, err := app.models.Notes.Link(note)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		case errors.Is(err, data.ErrLocationAlreadyLinked):
			app.editConflictResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"note": responseNote}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

func (app *application) DeleteLinkHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	NoteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	LocationID, err := app.readIDParam(r, "locationID")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	//validate
	v := validator.New()

	v.Check(NoteID > 0, "note_id", "must provide valid id")
	v.Check(LocationID > 0, "location_id", "must provide valid id")
	app.validateUser(v, &user.ID)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Notes.DeleteLink(NoteID, LocationID, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
