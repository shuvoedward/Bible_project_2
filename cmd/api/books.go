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
	var bibleNotes, crossRefNotes []*data.NoteResponse

	user := app.contextGetUser(r)
	if !user.IsAnonymous() && user.Activated {
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

func (app *application) autoCompleteHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		app.badRequestResponse(w, r, errors.New("query can not be empty"))
		return
	}

	// identify what type of search the user is performing
	result := identifySearchType(query, app.booksSearchIndex)
	if result == nil {
		// Query doesn't match any pattern(too short, invalid format)
		return
	}

	// Build the API response based on search type
	var response struct {
		Type   string             `json:"type"` // 3 types : "book", "verse", "word"; "book": action is to autocomplete
		Books  []string           `json:"books,omitempty"`
		Words  []*data.WordMatch  `json:"words,omitempty"`
		Verses []*data.VerseMatch `json:"verses,omitempty"`
	}

	switch result.Type {
	case "book":
		response.Type = "book"
		response.Books = result.Suggestions

	case "word":
		words, err := app.models.Passages.SuggestWords(result.Query)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		response.Type = "word"
		response.Words = words // May be empty - that's ok

	case "verse":
		verses, err := app.models.Passages.SuggestVerses(result.Query)
		if err != nil {
			app.serverErrorResponse(w, r, err)
			return
		}

		response.Type = "verse"
		response.Verses = verses // May be empty - that's ok
	}

	err := app.writeJSON(w, http.StatusOK, envelope{"autocomplete": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) searchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		app.badRequestResponse(w, r, errors.New("query can not be empty")) // return error
		return
	}

	// get a book, chapter - /v1/bible/:book/:chapter
	// get verses containing words - do this
	// get page and page_size
	params := data.SearchQueryParams{
		Filters: data.Filters{
			Page:     1,
			PageSize: 10,
		},
		Word: query,
	}

	verseList, err := app.models.Passages.SearchVersesByWord(params)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"list": verseList}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
