package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

// @Summary Get a Bible chapter or verse range
// @Description Retrieves the text for a specified Bible chapter or a range of verses, along with associated user-specific data (highlights and notes) if the user is logged in and activated.
// @Tags Bible, Passages
// @Accept json
// @Produce json
// @Param book path string true "The name of the Bible book (e.g., Genesis)"
// @Param chapter path int true "The chapter number (e.g., 1)"
// @Param svs query int false "Start verse number (must be used with evs)"
// @Param evs query int false "End verse number (must be used with svs)"
// @Security ApiKeyAuth
// @Success 200 {object} object{passage=data.Passage,highlights=[]data.Highlight,bible_notes=[]data.NoteResponse,cross_ref_notes=[]data.NoteResponse} "Successfully retrieved passage and user data"
// @Failure 400 {object} object{error=string} "Invalid request parameters (e.g., invalid chapter or verse numbers)"
// @Failure 404 {object} object{error=string} "Passage not found (e.g., invalid book/chapter combination)"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/bible/{book}/{chapter} [get]
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

	// Retrieve user-specific data (highlights and notes) if user is authenticated
	// Errors here don't block the response - we log them and return passage without user data
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

// @Summary Autocomplete Bible search queries
// @Description Provides intelligent autocomplete suggestions based on the search query type. Returns book name suggestions for partial book names, word matches for single words (3+ character), or verse reference for multi-word queries containing book names and references.
// @Tags Search, Autocomplete
// @Accept json
// @Produce json
// @Param q query string true "Search query (e.g., 'mat', 'john 3', 'love')"
// @Success 200 {object} object{autocomplete=object{type=string,books=[]string,words=[]data.WordMatch,verses=[]data.VerseMatch}} "Successfully returned autocomplete suggestions. Type indicates the search category: 'book' for book name matches, 'word' for word searches, or 'verse' for verse reference searches"
// @Failure 400 {object} object{error=string} "Query parameter is empty or missing"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/autocomplete [get]
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

// @Summary Search Bible verses by word
// @Description Performs a full-text search across Bible verses to find occurrences of the specified word or phrase. Returns paginated results with verse references and surrounding context.
// @Tags Search, Verses
// @Accept json
// @Produce json
// @Param q query string true "Search word or phrase (e.g., 'love', 'faith')"
// @Param page query int true "Page number (minimum: 1)" minimum(1)
// @Param page_size query int true "Number of results per page (minimum: 1)" minimum(1)
// @Success 200 {object} object{verses=[]data.VerseMatch, metadata=data.Metadata} "Successfully returned paginated verse search results"
// @Failure 400 {object} object{error=string} "Invalid query parameters (empty query, empty metadata)"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Router /v1/search/bible [get]
func (app *application) searchHandler(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("q")

	if searchQuery == "" {
		app.badRequestResponse(w, r, errors.New("query parameter 'q' can not be empty"))
		return
	}

	filters, err := app.readPaginationParams(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	verses, metadata, err := app.models.Passages.SearchVersesByWord(searchQuery, filters)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"verses": verses, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
