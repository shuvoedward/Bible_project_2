package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

func (app *application) insertHighlightHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var highlight data.Highlight

	err := app.readJSON(r, &highlight)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validation error
	v := validator.New()

	highlight.UserID = &user.ID

	app.validateHighlight(v, &highlight)
	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Highlights.Insert(&highlight)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"highlight": highlight}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) updateHighlightHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Color string `json:"color"`
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validate
	v := validator.New()

	v.Check(input.Color != "", "color", "must be provided")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Highlights.Update(*id, user.ID, input.Color)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := struct {
		ID    int64
		Color string
	}{
		ID:    *id,
		Color: input.Color,
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"highlights": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) deleteHighlightHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	err = app.models.Highlights.Delete(*id, user.ID)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusNoContent, envelope{"delete": "deleted"}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
