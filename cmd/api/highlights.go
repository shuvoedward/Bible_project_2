package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

// @Summary Create a new highlight
// @Description Create a new Bible verse highlight with color and position details
// @Tags highlights
// @Accept json
// @Produce json
// @Param highlight body data.Highlight true "Highlight data (user_id will be set automatically from auth)"
// @Success 201 {object} object{highlight=data.Highlight} "Successfully created highlight"
// @Failure 400 {object} map[string]string "Invalid JSON or request body"
// @Failure 422 {object} map[string]map[string]string "Validation errors"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/highlights [post]
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

// updateHighlightHandler updates the color of a specific highlight
// @Summary Update highlight color
// @Description Update the color of an existing highlight owned by the authenticated user
// @Tags highlights
// @Accept json
// @Produce json
// @Param id path int true "Highlight ID"
// @Param input body object true "Highlight color update" example({"color": "#FFFF00"})
// @Success 200 {object} object{highlights=object{id=int,color=string}} "Successfully updated highlight"
// @Failure 400 {object} object{error=string} "Invalid request (bad ID or JSON)"
// @Failure 404 {object} object{error=string} "Highlight not found or unauthorized"
// @Failure 422 {object} object{error=map[string]string} "Validation failed"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/highlights/{id} [patch]
func (app *application) updateHighlightHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
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

	// Optional: Add format validation for color (hex, rgb, etc.)
	// v.Check(isValidColor(input.Color), "color", "must be a valid color format")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Highlights.Update(id, user.ID, input.Color)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	response := struct {
		ID    int64  `json:"id"`
		Color string `json:"color"`
	}{
		ID:    id,
		Color: input.Color,
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"highlights": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// deleteHighlightHandler deletes a specific highlight
// @Summary Delete highlight
// @Description Delete an existing highlight owned by the authenticated user
// @Tags highlights
// @Param id path int true "Highlight ID"
// @Success 204 "Successfully deleted highlight"
// @Failure 400 {object} object{error=string} "Invalid highlight ID"
// @Failure 404 {object} object{error=string} "Highlight not found or unauthorized"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/highlights/{id} [delete]
func (app *application) deleteHighlightHandler(w http.ResponseWriter, r *http.Request) {
	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	err = app.models.Highlights.Delete(id, user.ID)
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
