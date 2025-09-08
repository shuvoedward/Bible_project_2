package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
)

// Get passage
func (app *application) getChapterOrVerses(w http.ResponseWriter, r *http.Request) {
	filters, err := app.getPassageFilters(r)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.badRequestResponse(w, r, err)
		}
		return
	}

	passage, err := app.models.Passages.Get(filters)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"passage": passage}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
