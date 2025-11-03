package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/image_compress"
	"time"

	"github.com/julienschmidt/httprouter"
	_ "golang.org/x/image/webp"
)

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
func (app *application) imageUploadHandler(w http.ResponseWriter, r *http.Request) {
	// post v1/notes/:noteid/images
	user := app.contextGetUser(r)

	noteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	err = r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		// modify error
		app.badRequestResponse(w, r, err)
		return
	}

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		// modify erroor
		app.badRequestResponse(w, r, err)
		return
	}
	defer file.Close()

	// retrieve the MIME type
	// mimeType := fileHeader.Header.Get("Content-Type")

	buffer, err := io.ReadAll(file)
	if err != nil {
		return
	}

	// detect the actual content type based on file contents (magic bytes)
	mimeType := http.DetectContentType(buffer)

	// validate mime type
	// allow jpeg, png, webp, (heic, heif) -> separate algorithm
	// dont allow gif, bmp, tiff, svg, raw formats(cr2, nef)
	var supportedFormats = map[string]bool{
		"image/jpg":  true,
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/heic": true,
		"image/heif": true,
	}

	if !supportedFormats[mimeType] {
		return
	}

	imageProcessor := image_compress.ImageProcessor{
		MaxWidth:  1920,
		MaxHeight: 1920,
		Quality:   85,
	}

	// for older browser use jpeg
	processedImage, err := imageProcessor.ProcessImageBuffer(buffer, "webp")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	width, height, err := GetImageDimensions(processedImage)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	s3Key, err := app.s3ImageService.UploadImage(ctx, processedImage, fileHeader, user.ID)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	presignedURL, err := app.s3ImageService.GeneratePresignedURL(ctx, s3Key, 3*time.Hour)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	input := &data.ImageData{
		NoteID:           noteID,
		S3Key:            s3Key,
		OriginalFileName: fileHeader.Filename,
		Width:            width,
		Height:           height,
		FileSize:         len(processedImage),
		MimeType:         mimeType,
	}

	// insert into db
	imageResponse, err := app.models.NoteImages.Insert(user.ID, input)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			app.notFoundResponse(w, r)
		default:
			app.serverErrorResponse(w, r, err)
		}
		app.s3ImageService.DeleteImage(ctx, s3Key)
		return
	}

	imageResponse.PresignedURL = presignedURL

	err = app.writeJSON(w, http.StatusCreated, envelope{"imageData": imageResponse}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
func (app *application) imageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// route v1/notes/noteid/images/:s3key  - method delete
	user := app.contextGetUser(r)

	noteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	s3Key := params.ByName("s3_key")

	// Remove leading slash from S3 key (httprouter includes it)
	s3Key = s3Key[1:]
	// TODO: Add validation for s3Key format if needed
	// e.g., check for empty string, valid characters, expected patterny

	// delete from s3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = app.s3ImageService.DeleteImage(ctx, s3Key)
	if err != nil {
		// Note: Database record is already deleted at this point
		// Consider logging this error for manual S3 cleanup
		app.serverErrorResponse(w, r, err)
		return
	}

	// Delete image metadata from database
	// This verifies the user owns the note and the image exists
	err = app.models.NoteImages.Delete(user.ID, noteID, s3Key)
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

func GetImageDimensions(imageData []byte) (width int, height int, err error) {
	img, _, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return 0, 0, err
	}

	bounds := img.Bounds()
	width = bounds.Dx()
	height = bounds.Dy()

	return width, height, nil
}
