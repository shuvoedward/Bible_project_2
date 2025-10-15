package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

// Get passage
func (app *application) getChapterOrVerses(w http.ResponseWriter, r *http.Request) {
	filter, err := app.getLocationFilters(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	app.validateLocationFilter(v, filter)
	if !v.Valid() {
		app.notFoundResponse(w, r)
		return
	}

	passage, err := app.models.Passages.Get(filter)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	// If user is not logged in, log the error but still return the passage
	// to ensure the user's reading experience is not blocked.
	highlights := []*data.Highlight{}
	var bibleNotes, crossRefNotes []*data.LocatedNoteResponse

	user := app.contextGetUser(r)
	if !user.IsAnonymous() {
		highlights, err = app.models.Highlights.Get(user.ID, filter)
		if err != nil {
			app.logger.Error(err.Error())
		}
		bibleNotes, crossRefNotes, err = app.models.Notes.GetAllLocatedForChapter(user.ID, filter)
		if err != nil {
			app.logger.Error(err.Error())
		}
	}

	err = app.writeJSON(w, http.StatusOK, envelope{
		"passage":         passage,
		"highlights":      highlights,
		"bible_notes":     bibleNotes,
		"cross-ref_notes": crossRefNotes,
	}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
