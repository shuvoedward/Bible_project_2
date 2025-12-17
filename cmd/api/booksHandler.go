package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"
	"shuvoedward/Bible_project/internal/validator"

	"github.com/julienschmidt/httprouter"
)

type BookServiceInterface interface {
	GetPassageWithUserData(userID int64, isAuthenticated bool, filter *data.LocationFilters) (*service.PassageResponse, *validator.Validator, error)
	SearchVersesByWord(searchQuery string, filters data.Filters) ([]*data.VerseMatch, data.Metadata, error)
}

type AutocompleteInterface interface {
	Autocomplete(query string) (*service.AutocompleteResult, error)
}

type BookHandler struct {
	app                 *application
	bookService         BookServiceInterface
	autocompleteService AutocompleteInterface
}

func NewBookHandler(
	app *application,
	bookService BookServiceInterface,
	autocompleteService AutocompleteInterface,
) *BookHandler {
	return &BookHandler{
		app:                 app,
		bookService:         bookService,
		autocompleteService: autocompleteService,
	}
}

func (h *BookHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodGet, "/v1/bible/:book/:chapter", h.app.generalRateLimit(h.GetPassageWithUserData))
	router.HandlerFunc(http.MethodGet, "/v1/autocomplete/bible", h.app.generalRateLimit(h.Autocomplete))
	router.HandlerFunc(http.MethodGet, "/v1/search/bible", h.app.generalRateLimit(h.Search))
}

func (h *BookHandler) handlerBooksError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrPassageNotFound):
		h.app.notFoundResponse(w, r)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

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
func (h *BookHandler) GetPassageWithUserData(w http.ResponseWriter, r *http.Request) {
	// if user not logged in always send passage
	// if logged in, send passage
	// along with user data related to the passage
	// user can have no notes, highlights, cross-ref notes
	// user has data but all of them or one of them failed to load
	// still send the partial data - user can reload again
	user := h.app.contextGetUser(r)

	filter, err := h.app.getLocationFilters(r)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}
	response, v, err := h.bookService.GetPassageWithUserData(user.ID, user.Activated, filter)
	if v != nil && !v.Valid() {
		h.app.notFoundResponse(w, r)
		return
	}

	if err != nil {
		h.handlerBooksError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"response": response}, nil)

	if err != nil {
		h.app.serverErrorResponse(w, r, err)
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
func (h *BookHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		h.app.badRequestResponse(w, r, errors.New("query can not be empty"))
		return
	}

	autocomplete, err := h.autocompleteService.Autocomplete(query)
	if err != nil {
		h.handlerBooksError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"autocomplete": autocomplete}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
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
func (h *BookHandler) Search(w http.ResponseWriter, r *http.Request) {
	searchQuery := r.URL.Query().Get("q")

	if searchQuery == "" {
		h.app.badRequestResponse(w, r, errors.New("query parameter 'q' can not be empty"))
		return
	}

	filters, err := h.app.readPaginationParams(r)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	verses, metadata, err := h.bookService.SearchVersesByWord(searchQuery, filters)

	if err != nil {
		h.handlerBooksError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"verses": verses, "metadata": metadata}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}
