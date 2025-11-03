package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"slices"
	"strconv"
	"strings"
	"time"
)

var validationError = errors.New("validation error")

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

func (i *CreateNoteInput) ToNoteContent(userID int64) *data.NoteContent {
	return &data.NoteContent{
		UserID:   userID,
		Title:    strings.TrimSpace(i.Title),
		Content:  i.Content,
		NoteType: i.NoteType,
	}
}

func (i *CreateNoteInput) ToNoteLocation() *data.NoteLocation {
	return &data.NoteLocation{
		Book:        i.Book,
		Chapter:     i.Chapter,
		StartVerse:  i.StartVerse,
		EndVerse:    i.EndVerse,
		StartOffset: i.StartOffset,
		EndOffset:   i.EndOffset,
	}
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
func (app *application) createNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	ip := getIP(r)
	// Rate limit note creation by IP address
	if !app.noteRateLimiter.Allow(ip) {
		app.rateLimitExceededResponse(w, r)
		return
	}

	var input CreateNoteInput

	err := app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Convert input to domain models
	content := input.ToNoteContent(user.ID)
	location := input.ToNoteLocation()

	// validate and create note based on type
	v := validator.New()
	note, err := app.createNote(v, content, location)
	if err != nil {
		switch {
		case errors.Is(err, validationError):
			app.failedValidationResponse(w, r, v.Errors)
		case errors.Is(err, data.ErrDuplicateTitleGeneral):
			// GENERAL notes must have unique titles per user
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrLocationAlreadyLinked):
			// BIBLE/CROSS_REFERENCE notes cannot share the same location
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrDuplicateContent):
			// Content must be unique within the same note type
			app.editConflictResponse(w, r, err)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"note": note}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

// createNote handles validation and insertion based on note type.
// Returns validation errors, duplicate errors, or database errors.
func (app *application) createNote(v *validator.Validator, content *data.NoteContent, location *data.NoteLocation) (*data.NoteResponse, error) {
	switch content.NoteType {

	case "GENERAL":
		app.validateNoteContent(v, content.Content)
		app.validateGeneralNote(v, content)
		if !v.Valid() {
			return nil, validationError
		}
		return app.models.Notes.InsertGeneral(content)

	case "BIBLE", "CROSS_REFERENCE":
		app.validateLocatedNote(v, content, location)
		if !v.Valid() {
			return nil, validationError
		}
		return app.models.Notes.InsertLocated(content, location)

	default:
		v.AddError("note_type", "must provide a valid note type")
		return nil, validationError
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
func (app *application) deleteNoteHandler(w http.ResponseWriter, r *http.Request) {
	/*
		Design Decision: Delete S3 first, then DB
		- No orphaned S3 objects costing you money
		- If anything fails, user can retry
		- DB remains authoritative source
		- In the rare case DB fails after S3 succeeds, user just retries and
		  note is gone but images already deleted (idempotent)
	*/
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	exists, err := app.models.Notes.ExistsForUser(id, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	if !exists {
		app.notFoundResponse(w, r)
		return
	}

	// Get images before deletion
	images, err := app.models.NoteImages.GetForNote(id)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		app.serverErrorResponse(w, r, err)
		return
	}

	// Delete from s3 first - this operation is most likely to fail
	var failedKeys []string
	for _, image := range images {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := app.s3ImageService.DeleteImage(ctx, image.S3Key)
		cancel()

		if err != nil {
			failedKeys = append(failedKeys, image.S3Key)
			app.logger.Error("failed to delete image",
				"error", err,
				"s3_key", image.S3Key,
				"note_id", id,
			)
		}
	}

	//If any S3 deletion falied, abort - don't touch the database
	if len(failedKeys) > 0 {
		app.logger.Error("aboring note deletion due to S3 failures",
			"note_id", id,
			"failed_count", len(failedKeys),
		)
		app.serverErrorResponse(w, r,
			fmt.Errorf("failed to delete %d image(s) from storage", len(failedKeys)))
		return
	}

	// All S3 deletion successful - now safe to delete from DB
	err = app.models.Notes.Delete(id, user.ID)
	if err != nil {
		// S3 is deleted but DB failed
		app.logger.Error("DB deletion failed after S3 cleanup",
			"note_id", id,
			"error", err,
		)
		app.serverErrorResponse(w, r, err)
		return
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
func (app *application) updateNoteHandler(w http.ResponseWriter, r *http.Request) {
	/*
		Design Decision: Single Query Update with Security Checks
		- Client provides note_type to determine validation rules
		- WHERE clause verifies: id exists, user owns it, AND note_type matches
		- If note_type doesn't match, update fails with ErrRecordNotFound (appropriate)
		- Content hashing only applied to BIBLE/CROSS_REFERENCE notes
		- Single DB query for performance (no separate type check needed)
	*/

	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Title    string `json:"title"`
		Content  string `json:"content"`
		NoteType string `json:"note_type"` // Used for validation rules and WHERE clause verification
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	input.Title = strings.TrimSpace(input.Title)

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

	note, err := app.models.Notes.Update(content)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrDuplicateTitleGeneral):
			// GENERAL notes must have unique title per user
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrDuplicateContent):
			// Notes can not have same content
			app.editConflictResponse(w, r, err)
		case errors.Is(err, data.ErrRecordNotFound):
			// Could be: note doesn't exist, wrong user, OR wrong note_type
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}

		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"note": note}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
func (app *application) linkNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var input struct {
		Book        string `json:"book"`
		Chapter     int    `json:"chapter"`
		StartVerse  int    `json:"start_verse"`
		EndVerse    int    `json:"end_verse"`
		StartOffset int    `json:"start_offset"`
		EndOffset   int    `json:"end_offset"`
	}

	noteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	err = app.readJSON(r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()

	v.Check(noteID > 0, "note_id", "must be a valid id")
	app.validateBook(v, input.Book, input.Chapter, input.StartVerse, input.EndVerse)
	app.validateLocation(v, &input.StartOffset, &input.EndOffset)
	app.validateUser(v, &user.ID)

	// For single verse links, ensure start offset comes before end offset
	if input.StartVerse == input.EndVerse {
		if input.StartOffset != 0 && input.EndOffset != 0 {
			v.Check(input.StartOffset < input.EndOffset, "offset", "start offset must be less than end offset")
		}
	}
	v.Check(input.StartVerse <= input.EndVerse, "verse", "start verse must be less than or equal to end verse")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	// noteLinkLocation is better?
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

	linkedNote, err := app.models.Notes.Link(locationInput)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	err = app.writeJSON(w, http.StatusCreated, envelope{"link": linkedNote}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
func (app *application) deleteLinkHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	noteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	locationID, err := app.readIDParam(r, "locationID")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	v := validator.New()
	v.Check(noteID > 0, "note_id", "must provide valid id")
	v.Check(locationID > 0, "location_id", "must provide valid id")
	app.validateUser(v, &user.ID)

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = app.models.Notes.DeleteLink(noteID, locationID, user.ID)
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
func (app *application) listNotesMetadataHandler(w http.ResponseWriter, r *http.Request) {
	// for now two note_tyoe GENERAL, BIBLE
	// query: ?note_type=GENERAL&page=1&page_size=10&sort=(-)created_at, (-)title
	user := app.contextGetUser(r)

	queryParams, err := app.parseNoteQuery(r)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// validation checks
	v := validator.New()

	v.Check(queryParams.Page <= 10000, "page", "must be at most 10000")
	v.Check(queryParams.PageSize <= 100, "page_size", "must be at most 100")
	v.Check(queryParams.NoteType == data.NoteTypeGeneral || queryParams.NoteType == data.NoteTypeBible,
		"type", "must be BIBLE or GENERAL")

	//validating sort
	queryParams.SortSafeList = []string{"created_at", "-created_at", "title", "-title"}
	if !slices.Contains(queryParams.SortSafeList, queryParams.Sort) {
		v.AddError("sort", "invalid value")
	}

	if queryParams.NoteType == "BIBLE" && (queryParams.Sort == "title" || queryParams.Sort == "-title") {
		v.AddError("sort", "BIBLE notes can not be sorted by title")
	}

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	notesMetadata, err := app.models.Notes.GetAllMetadata(user.ID, queryParams)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"notes_metadata": notesMetadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
func (app *application) getNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	id, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	// Retrieve note from database - automatically filters by user_id for security
	noteResponse, err := app.models.Notes.Get(user.ID, id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			// Note doesn't exist or doesn't belong to this user
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		return
	}

	var imageData []*data.ImageData

	imageData, err = app.models.NoteImages.GetForNote(noteResponse.ID)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			// Log error but don't fail request if no images found
			// Notes without images are valid and should still be returned
			app.logger.Error("failed to retrieve images", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, image := range imageData {
		presignedURL, err := app.s3ImageService.GeneratePresignedURL(ctx, image.S3Key, 3*time.Hour)
		if err != nil {
			// Log error but continue - frontend can handle missing URLs
			app.logger.Error("failed to generate presigned url", "error", err)
		}
		image.PresignedURL = presignedURL
	}

	// Return note content and images array to client
	// Frontend will match image IDs in content with images array
	err = app.writeJSON(w, http.StatusOK, envelope{"note": noteResponse, "images": imageData}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}

}

// searchNoteHandler godoc
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
func (app *application) searchNoteHandler(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var requestErrors []string
	query := r.URL.Query()

	searchQuery := query.Get("q")
	if searchQuery == "" {
		requestErrors = append(requestErrors, "query can not be empty")
	}

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		requestErrors = append(requestErrors, "page must be at least 1")
	}

	pageSize, err := strconv.Atoi(query.Get("page_size"))
	if err != nil || pageSize < 1 {
		requestErrors = append(requestErrors, "page_size must be at least 1")
	}

	if len(requestErrors) > 0 {
		app.badRequestResponse(w, r, errors.New(strings.Join(requestErrors, "; ")))
		return
	}
	v := validator.New()

	v.Check(page <= 10000, "page", "must be at most 10000")
	v.Check(pageSize <= 100, "page_size", "must be at most 100")

	if !v.Valid() {
		app.failedValidationResponse(w, r, v.Errors)
		return
	}

	filter := data.Filters{
		Page:     page,
		PageSize: pageSize,
	}

	results, metadata, err := app.models.Notes.SearchNotes(user.ID, searchQuery, &filter)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	err = app.writeJSON(w, http.StatusOK, envelope{"notes": results, "metadata": metadata}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
