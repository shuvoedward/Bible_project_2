package main

import (
	"errors"
	"io"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/service"

	"github.com/julienschmidt/httprouter"
	_ "golang.org/x/image/webp"
)

type ImageHandler struct {
	app     *application
	service *service.ImageService
}

func NewImageHandler(app *application, service *service.ImageService) *ImageHandler {
	return &ImageHandler{
		app:     app,
		service: service,
	}
}

func (h *ImageHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc(http.MethodPost, "/v1/notes/:id/images", h.app.generalRateLimit(h.app.requireActivatedUser(h.Upload)))
	router.HandlerFunc(http.MethodDelete, "/v1/notes/:id/images/*s3_key", h.app.generalRateLimit(h.app.requireActivatedUser(h.Delete)))
}

func (h *ImageHandler) handlerImageError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, data.ErrRecordNotFound):
		h.app.notFoundResponse(w, r)
	default:
		h.app.serverErrorResponse(w, r, err)
	}
}

// imageUploadHandler handles image upload for a specific note
// @Summary Upload image to note
// @Description Upload and process an image file, then attach it to a note owned by the authenticated user
// @Tags images
// @Accept multipart/form-data
// @Produce json
// @Param id path int true "Note ID"
// @Param image formData file true "Image file (JPEG, PNG, WebP, HEIC, HEIF)"
// @Success 201 {object} object{imageData=data.ImageData} "Successfully uploaded and processed image"
// @Failure 400 {object} object{error=string} "Invalid request (bad note ID, file size exceeded, or invalid file format)"
// @Failure 404 {object} object{error=string} "Note not found or unauthorized"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Security ApiKeyAuth
// @Router /notes/{id}/images [post]
func (h *ImageHandler) Upload(w http.ResponseWriter, r *http.Request) {
	// post v1/notes/:noteid/images
	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	err = r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		// modify error
		h.app.badRequestResponse(w, r, err)
		return
	}

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		// modify erroor
		h.app.badRequestResponse(w, r, err)
		return
	}
	defer file.Close()

	// retrieve the MIME type
	// mimeType := fileHeader.Header.Get("Content-Type")

	buffer, err := io.ReadAll(file)
	if err != nil {
		return
	}

	input := service.UploadImageInput{
		UserID:           user.ID,
		NoteID:           noteID,
		ImageBuffer:      buffer,
		OriginalFileName: fileHeader.Filename,
	}

	image, v, err := h.service.UploadImage(input)
	if v != nil && !v.Valid() {
		h.app.failedValidationResponse(w, r, v.Errors)
		return
	}
	if err != nil {
		h.handlerImageError(w, r, err)
		return
	}

	err = h.app.writeJSON(w, http.StatusCreated, envelope{"imageData": image}, nil)
	if err != nil {
		h.app.serverErrorResponse(w, r, err)
	}
}

// imageDeleteHandler deletes an image from a note
// @Summary Delete image from note
// @Description Delete an image attachment from a note owned by the authenticated user. Removes both database record and S3 object.
// @Tags images
// @Param id path int true "Note ID"
// @Param s3_key path string true "S3 Key of the image (URL encoded)"
// @Success 204 "Successfully deleted image"
// @Failure 400 {object} object{error=string} "Invalid note ID"
// @Failure 404 {object} object{error=string} "Image not found or unauthorized"
// @Failure 500 {object} object{error=string} "Internal server error"
// @Security ApiAuthKey
// @Router /notes/{id}/images/{s3_key} [delete]
func (h *ImageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// route v1/notes/:noteid/images/:s3key  - method delete
	user := h.app.contextGetUser(r)

	noteID, err := h.app.readIDParam(r, "id")
	if err != nil {
		h.app.badRequestResponse(w, r, err)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	s3Key := params.ByName("s3_key")

	// Remove leading slash from S3 key (httprouter includes it)
	s3Key = s3Key[1:]
	// TODO: Add validation for s3Key format if needed
	// e.g., check for empty string, valid characters, expected patterny

	err = h.service.DeleteImage(s3Key, user.ID, noteID)
	if err != nil {
		h.handlerImageError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
