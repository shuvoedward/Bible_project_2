package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"
	"strconv"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type NoteHandler struct {
	app     *application
	service *service.NoteService
}

func NewNoteHandler(app *application, noteService *service.NoteService) *NoteHandler {
	return &NoteHandler{
		app:     app,
		service: noteService,
	}
}

func (h *NoteHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/v1/notes",
		h.app.requireActivatedUser(h.Create))

	router.HandlerFunc(http.MethodGet, "/v1/notes/:id",
		h.app.generalRateLimit(h.app.requireActivatedUser(h.Get)))

	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id",
		h.app.generalRateLimit(h.app.requireActivatedUser(h.Delete)))

	router.HandlerFunc(http.MethodPut, "/v1/notes/:id",
		h.app.generalRateLimit(h.app.requireActivatedUser(h.Update)))

	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/locations",
		h.app.generalRateLimit(h.app.requireActivatedUser(h.Link)))

	router.HandlerFunc(http.MethodGet, "/v1/notes",
		h.app.requireActivatedUser(h.app.requireActivatedUser((h.ListNotesMetadata))))

	router.HandlerFunc(http.MethodGet, "/v1/search/notes",
		h.app.requireActivatedUser(h.app.generalRateLimit(h.SearchNote)))

	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/locations/:locationID",
		h.app.generalRateLimit(h.app.requireActivatedUser(h.DeleteLink)))
}

type CreateNoteInput struct {
	Title    string `json:"title"`
	Content  string `json:"content"`
	NoteType string `json:"note_type"` // "GENERAL", "BIBLE", or "CROSS_REFERENCE"

	// Bible verse location fields (only used for BIBLE and CROSS_REFERENCE types)
	Book        string `json:"book"`
	Chapter     int    `json:"chapter"`
	StartVerse  int    `json:"start_verse"`
	EndVerse    int    `json:"end_verse"`    // end verse must be provided, included. when just one verse, svs = 1 and evs = 1.
	StartOffset int    `json:"start_offset"` // Character offset within start verse
	EndOffset   int    `json:"end_offset"`   // Character offset within end verse
}

// createNoteHandler inserts a note
// @Summary Create a new note
// @Description Creates a new note of type GENERAL, BIBLE, or CROSS_REFERENCE. GENERAL notes only require title and content. BIBLE and CROSS_REFERENCE notes require additional location fields (book, chapter, verses, and offsets), title optional for BIBLE and no title necessary for CROSS_REFERENCE.
// @Tags notes
// @Accept json
// @Produce json
// @Param input body CreateNoteInput true "Note creation data"
// @Success 201 {object} map[string]data.NoteResponse "Successfully created note"
// @Failure 400 {object} map[string]interface{} "Invalid request body"
// @Failure 409 {object} map[string]interface{} "Duplicate title (GENERAL), location already linked (BIBLE/CROSS_REFERENCE), or duplicate content"
// @Failure 422 {object} map[string]map[string]string "Validation errors"
// @Failure 429 {object} map[string]string "Rate limit exceeded"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes [post]
func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	ip := getIP(r)
	// Rate limit note creation by IP address
	if !h.app.rateLimiter.IP.Allow(ip) {
		h.app.rateLimitExceededResponse(w, r)
		return
	}

	// 1. Parse HTTP request
	var input CreateNoteInput
	err := h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	// 2. Call service
	serviceInput := service.CreateNoteInput{
		Title:       input.Title,
		Content:     input.Content,
		NoteType:    input.NoteType,
		Book:        input.Book,
		Chapter:     input.Chapter,
		StartVerse:  input.StartVerse,
		EndVerse:    input.EndVerse,
		StartOffset: input.StartOffset,
		EndOffset:   input.EndOffset,
	}

	note, v, err := h.service.CreateNote(user.ID, serviceInput)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
	}
	if err != nil {
		h.handleNoteError(w, r, err)
	}

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"note": note}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

func (h *NoteHandler) handleNoteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrNoteNotFound):
		h.app.notFoundResponse(w, r)
	case errors.Is(err, service.ErrLinkNotFound):
		h.app.notFoundResponse(w, r)
	case errors.Is(err, data.ErrDuplicateTitleGeneral):
		// GENERAL notes must have unique titles per user
		h.app.editConflictResponse(w, r, err)
	case errors.Is(err, data.ErrLocationAlreadyLinked):
		// BIBLE/CROSS_REFERENCE notes cannot share the same location
		h.app.editConflictResponse(w, r, err)
	case errors.Is(err, data.ErrDuplicateContent):
		// Content must be unique within the same note type
		h.app.editConflictResponse(w, r, err)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

// deleteNoteHandler deletes a note
// @Summary Delete a note
// @Description Deletes a note and all associated images. The note must belong to the authenticated user. Images are removed from S3 storage first, then from the database. If S3 deletion fails, the operation aborts to prevent orphaned objects.
// @Tags notes
// @Produce json
// @Param id path int true "Note ID"
// @Success 204 "Note successfully deleted"
// @Failure 400 {object} map[string]string "Invalid note ID"
// @Failure 404 {object} map[string]string "Note not found or does not belong to user"
// @Failure 500 {object} map[string]string "Internal server error or S3 deletion failure"
// @Security ApiKeyAuth
// @Router /v1/notes/{id} [delete]
func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	/*
		Design Decision: Delete S3 first, then DB
		- No orphaned S3 objects costing you money
		- If anything fails, user can retry
		- DB remains authoritative source
		- In the rare case DB fails after S3 succeeds, user just retries and
		  note is gone but images already deleted (idempotent)
	*/
	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	err = h.service.DeleteNote(user.ID, noteID)
	if err != nil {
		h.handleNoteError(w, r, err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateNoteHandler updates a note
// @Summary Update a note
// @Description Updates a note's title and content. The note must belong to the authenticated user and the note_type must match. For BIBLE and CROSS_REFERENCE notes, content is hashed to prevent duplicate annotations.
// @Tags notes
// @Accept json
// @Produce json
// @Param id path int true "Note ID"
// @Success 200 {object} map[string]data.NoteResponse "note" "Successfully updated note"
// @Failure 400 {object} map[string]interface{} "Invalid request body or note ID"
// @Failure 404 {object} map[string]string "Note not found, doesn't belong to user, or note_type mismatch"
// @Failure 409 {object} map[string]interface{} "Duplicate title (GENERAL) or duplicate content (BIBLE/CROSS_REFERENCE)"
// @Failure 422 {object} map[string]map[string]string "Validation errors"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes/{id} [put]
func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	/*
		Design Decision: Single Query Update with Security Checks
		- Client provides note_type to determine validation rules
		- WHERE clause verifies: id exists, user owns it, AND note_type matches
		- If note_type doesn't match, update fails with ErrRecordNotFound (appropriate)
		- Content hashing only applied to BIBLE/CROSS_REFERENCE notes
		- Single DB query for performance (no separate type check needed)
	*/

	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		NoteType string `json:"note_type"` // Used for validation rules and WHERE clause verification
	}

	err = h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	content := &data.NoteContent{
		ID:       noteID,
		UserID:   user.ID,
		Title:    strings.TrimSpace(input.Title),
		Content:  input.Content,
		NoteType: input.NoteType,
	}

	note, v, err := h.service.UpdateNote(content)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
	}

	if err != nil {
		h.handleNoteError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"note": note}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// linkNoteHandler links an existing note to a specific Bible verse location.
// @Summary Link note to verse location
// @Description Creates a location link for an existing note at the specified Bible verse and word offsets
// @Tags notes
// @Accept json
// @Produce json
// @Param id path int true "Note ID"
// @Param input body object true "Location details" example({"book":"John","chapter":3,"start_verse":16,"end_verse":16,"start_offset":10,"end_offset":14})
// @Success 201 {object} map[string]interface{} "Successfully linked note to location"
// @Failure 400 {object} map[string]interface{} "Invalid input"
// @Failure 404 {object} map[string]interface{} "Note not found or doesn't belong to user"
// @Failure 422 {object} map[string]interface{} "Validation failed"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes/{id}/locations [post]
func (h *NoteHandler) Link(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Book        string `json:"book"`
		Chapter     int    `json:"chapter"`
		StartVerse  int    `json:"start_verse"`
		EndVerse    int    `json:"end_verse"`
		StartOffset int    `json:"start_offset"`
		EndOffset   int    `json:"end_offset"`
	}

	err = h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	// noteLinkLocation is better? also, probably better to use NoteLocation and UserID separately
	locationInput := &data.NoteInputLocation{
		NoteID:      noteID,
		UserID:      user.ID,
		Book:        input.Book,
		Chapter:     input.Chapter,
		StartVerse:  input.StartVerse,
		EndVerse:    input.EndVerse,
		StartOffset: input.StartOffset,
		EndOffset:   input.EndOffset,
	}

	linkedNote, v, err := h.service.LinkNote(locationInput)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
	}
	if err != nil {
		h.handleNoteError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"link": linkedNote}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}

}

// deleteLinkHandler removes a location link from a note.
// @Summary Delete note location link
// @Description Removes a specific location link from a note. Only the note owner can delete links.
// @Tags notes
// @Produce json
// @Param id path int true "Note ID"
// @Param locationID path int true "Location ID"
// @Success 204 "Location link successfully deleted"
// @Failure 400 {object} map[string]interface{} "Invalid input"
// @Failure 404 {object} map[string]interface{} "Note or location not found, or doesn't belong to user"
// @Failure 422 {object} map[string]interface{} "Validation failed"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes/{id}/locations/{locationID} [delete]
func (h *NoteHandler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	// fix: locationID and userID is enough? no need for noteID
	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	locationID, err := h.app.readIDParam(r, "locationID")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	v, err := h.service.DeleteLink(user.ID, noteID, locationID)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}

	if err != nil {
		h.handleNoteError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listNotesHandler retrieves a paginated list of notes filtered by note type.
// @Summary List user notes
// @Description Get a paginated list of notes filtered by type (GENERAL or BIBLE) with optional sorting
// @Tags notes
// @Accept json
// @Produce json
// @Param note_type query string true "Note type" Enums(GENERAL, BIBLE)
// @Param page query int false "Page number" default(1) minimum(1) maximum(10000)
// @Param page_size query int false "Number of items per page" default(10) minimum(1) maximum(100)
// @Param sort query string false "Sort field and direction" Enums(created_at, -created_at, title, -title) default(-created_at)
// @Success 200 {object} map[string][]data.NoteMetadata"Successfully retrieved notes (key is note_type)"
// @Failure 400 {object} map[string]interface{} "Invalid query parameters or validation errors"
// @Failure 422 {object} map[string]interface{} "Validation failed"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes [get]
func (h *NoteHandler) ListNotesMetadata(w http.ResponseWriter, r *http.Request) {
	// for now two note_type - GENERAL, BIBLE
	// query: ?note_type=GENERAL&page=1&page_size=10&sort=(-)created_at, (-)title
	user := h.app.contextGetUser(r)

	input, err := h.app.parseNoteQuery(r)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	notesMetadata, v, err := h.service.ListNotesMetadata(user.ID, input)
	if !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// what if user doesn't have any note?
	if err != nil {
		h.handleNoteError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"notes_metadata": notesMetadata}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// getNoteHandler retrieves a single note along with all its associated images.
// For each image, generates a presigned URL valid for 3 hours to allow secure access to S3 objects.
// @Summary Get a single note by ID
// @Description Retrieve a specific note with its content and all associated images. Each image includes a presigned URL for temporary S3 access.
// @Tags notes
// @Accept json
// @Produce json
// @Param id path int true "Note ID"
// @Success 200 {object} map[string]interface{} "note: NoteContent object, images: array of ImageData objects with presigned URLs"
// @Failure 400 {object} map[string]interface{} "Invalid note ID parameter"
// @Failure 404 {object} map[string]interface{} "Note not found or doesn't belong to user"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/notes/{id} [get]
func (h *NoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	notesID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	note, images, err := h.service.GetNote(user.ID, notesID)
	if err != nil {
		h.handleNoteError(w, r, err)
	}

	// Return note content and images array to client
	// Frontend will match image IDs in content with images array
	err = h.app.writeJSON(w, http.StatusOK, envelope{"note": note, "images": images}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}

}

// @Summary Search notes with full-text search
// @Description Search through user's notes using PostgreSQL full-text search with pagination
// @Tags notes
// @Accept json
// @Produce json
// @Param q query string true "Search query"
// @Param page query int true "Page number (min: 1, max: 10000)"
// @Param page_size query int true "Items per page (min: 1, max: 100)"
// @Success 200 {object} object{notes=[]data.NoteSearchResponse,metadata=data.Metadata} "Search results with pagination metadata"
// @Failure 400 {object} map[string]string "Invalid request parameters"
// @Failure 422 {object} map[string]map[string]string "Validation errors"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security ApiKeyAuth
// @Router /v1/search/notes [get]
func (h *NoteHandler) SearchNote(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	query := r.URL.Query()

	searchQuery := query.Get("q")
	if searchQuery == "" {
		h.app.badRequestResponse(w, r, errors.New("search query can't be empty"))
	}

	// check err here, it falls into bad requests?
	page, err := strconv.Atoi(query.Get("page"))
	pageSize, err := strconv.Atoi(query.Get("page_size"))

	searchInput := service.SearchInput{
		SearchQuery: searchQuery,
		Page:        page,
		PageSize:    pageSize,
	}

	results, metadata, v, err := h.service.SearchNotes(user.ID, searchInput)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
	}
	if err != nil {
		h.handleNoteError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"notes": results, "metadata": metadata}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}
