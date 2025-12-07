package service

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"time"
)

// Compression interface
type ImageProcessor interface {
	Process(buffer []byte, outputFormat string) ([]byte, error)
}

type ImageStorage interface {
	UploadImage(ctx context.Context, imageData []byte, fileName string, contentType string, userID int64) (string, error)
	GeneratePresignedURL(ctx context.Context, s3Key string, expiration time.Duration) (string, error)
	DeleteImage(ctx context.Context, s3Key string) error
}

type ImageService struct {
	noteModel      data.NoteModel
	imageModel     data.ImageModel
	imageProcessor ImageProcessor
	imageStorage   ImageStorage
}

func NewImageService(
	imageProcessor ImageProcessor,
	imageStorage ImageStorage,
	imageModel data.ImageModel,
	noteModel data.NoteModel,
) *ImageService {
	return &ImageService{
		imageProcessor: imageProcessor,
		imageStorage:   imageStorage,
		imageModel:     imageModel,
		noteModel:      noteModel,
	}
}

type UploadImageInput struct {
	UserID           int64
	NoteID           int64
	ImageBuffer      []byte
	OriginalFileName string
}

// UploadImage handles teh complete image upload workflow
// Returns: imageData, validator (if validations fails), error
func (s *ImageService) UploadImage(input UploadImageInput) (*data.ImageData, *validator.Validator, error) {
	s.noteModel.ExistsForUser(input.NoteID, input.UserID)
	// 1. Detect the actual content type based on file contents (magic bytes)
	// WHY: Security - don't trust client-provided Content-Type header
	// HOW: http.DetectContentType reads first 512 bytes
	mimeType := http.DetectContentType(input.ImageBuffer)

	// 2. Vaidate the image format type and input
	v := validateImageUpload(mimeType, input.NoteID)
	if !v.Valid() {
		return nil, v, nil
	}

	// 3. Process image (resize, compress, convert to WebP)
	processedImage, err := s.imageProcessor.Process(input.ImageBuffer, "webp")
	if err != nil {
		return nil, nil, err
	}

	// 4. Get dimensions
	width, height, err := GetImageDimensions(processedImage)
	if err != nil {
		return nil, nil, err
	}

	// 5. Upload to S3
	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer uploadCancel()

	s3Key, err := s.imageStorage.UploadImage(uploadCtx, processedImage, input.OriginalFileName, mimeType, input.UserID)
	if err != nil {
		return nil, nil, err
	}

	// 6. Save metadata to database
	imageInput := &data.ImageData{
		NoteID:           input.NoteID,
		S3Key:            s3Key,
		OriginalFileName: input.OriginalFileName,
		Width:            width,
		Height:           height,
		FileSize:         len(processedImage),
		MimeType:         "image/webp", // after processing, its always WebP
	}

	imageResponse, err := s.imageModel.Insert(input.UserID, imageInput)
	if err != nil {
		deleteCtx, deleteCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer deleteCancel()
		s.imageStorage.DeleteImage(deleteCtx, s3Key)
		return nil, nil, err
	}

	// 7. Generate presigned URL for immediate use
	urlCtx, urlCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer urlCancel()

	presignedURL, err := s.imageStorage.GeneratePresignedURL(urlCtx, s3Key, 3*time.Hour)
	if err != nil {
		return nil, nil, err
	}

	imageResponse.PresignedURL = presignedURL

	return imageResponse, nil, nil
}

func (s *ImageService) DeleteImage(s3Key string, userID, noteID int64) error {
	// 1. Delete from S3 first
	// WHY: If S3 delete fails, we don't want orphaned DB records
	// pointing to non-existent S3 objects
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.imageStorage.DeleteImage(ctx, s3Key)
	if err != nil {
		return err
	}

	// Delete image metadata from database
	// This also verifies the user owns the note and the image exists
	err = s.imageModel.Delete(userID, noteID, s3Key)
	if err != nil {
		return err
	}

	return nil
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
