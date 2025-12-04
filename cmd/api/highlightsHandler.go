package main

import (
	"errors"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"

	"github.com/julienschmidt/httprouter"
)

type HighlightHandler struct {
	app     *application
	service *service.HighlightService
}

func NewHighlightHandler(app *application, service *service.HighlightService) *HighlightHandler {
	return &HighlightHandler{
		app:     app,
		service: service,
	}
}

func (h *HighlightHandler) RegisterRoutes(router httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/v1/highlights", h.app.generalRateLimit(h.app.requireActivatedUser(h.Insert)))
	router.HandlerFunc(http.MethodPatch, "/v1/highlights/:id", h.app.requireActivatedUser(h.app.generalRateLimit(h.Update)))
	router.HandlerFunc(http.MethodDelete, "/v1/highlights/:id", h.app.generalRateLimit(h.app.requireActivatedUser(h.Delete)))
}

func (h *HighlightHandler) handleHighlightError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrHighlightNotFound):
		h.app.notFoundResponse(w, r)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

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
func (h *HighlightHandler) Insert(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	var highlight data.Highlight

	err := h.app.readJSON(r, &highlight)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	v, err := h.service.InsertHighlight(&highlight, user.ID)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}

	if err != nil {
		h.handleHighlightError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"highlight": highlight}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
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
func (h *HighlightHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := h.app.contextGetUser(r)

	highlightID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	var input struct {
		Color string `json:"color"`
	}

	err = h.app.readJSON(r, &input)
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	v, err := h.service.UpdateHighlight(highlightID, user.ID, input.Color)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleHighlightError(w, r, err)
		return
	}

	response := struct {
		HighlightID int64  `json:"id"`
		Color       string `json:"color"`
	}{
		HighlightID: highlightID,
		Color:       input.Color,
	}

	err = h.app.writeJSON(w, http.StatusOK, envelope{"highlights": response}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
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
func (h *HighlightHandler) Delete(w http.ResponseWriter, r *http.Request) {
	highlightID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	user := h.app.contextGetUser(r)

	v, err := h.service.DeleteHighlight(highlightID, user.ID)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handleHighlightError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
