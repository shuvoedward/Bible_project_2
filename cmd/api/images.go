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

// post v1/notes/:noteid/images
func (app *application) imageUploadHandler(w http.ResponseWriter, r *http.Request) {
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

func (app *application) imageDeleteHandler(w http.ResponseWriter, r *http.Request) {
	// route v1/notes/noteid/images/:s3key  - method delete
	// check the note belong to the user
	// delete from db
	// delete from s3
	user := app.contextGetUser(r)

	noteID, err := app.readIDParam(r, "id")
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	params := httprouter.ParamsFromContext(r.Context())
	s3Key := params.ByName("s3_key")
	s3Key = s3Key[1:]
	// validate s3Key

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

	// delete from s3
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = app.s3ImageService.DeleteImage(ctx, s3Key)
	if err != nil {
		app.serverErrorResponse(w, r, err)
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
