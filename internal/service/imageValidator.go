package service

import "shuvoedward/Bible_project/internal/validator"

func validateImageUpload(mimetype string, noteID int64) *validator.Validator {
	v := validator.New()

	ValidateImageType(v, mimetype)
	v.Check(noteID > 0, "noteID", "must be valid")

	return v
}

func ValidateImageType(v *validator.Validator, mimeType string) error {
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

	v.Check(supportedFormats[mimeType], "image type", "not supported")

	return nil
}
