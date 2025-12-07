package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"slices"
	"strings"
	"time"
)

// Interface Segragation Principle (ISP)
type ImageDeleter interface {
	DeleteImage(ctx context.Context, s3Key string) error
}

type PresignedURLGenerator interface {
	GeneratePresignedURL(ctx context.Context, s3Key string, duration time.Duration) (string, error)
}

type ImageStorageNote interface {
	ImageDeleter
	PresignedURLGenerator
}

// NoteService handles notes business logic
type NoteService struct {
	noteModel  data.NoteModel
	imageModel data.ImageModel
	imageStore ImageStorageNote
	validator  *NoteValidator
	logger     *slog.Logger
}

func NewNoteService(
	noteModel data.NoteModel,
	imageModel data.ImageModel,
	imageStoreNote ImageStorageNote,
	validator *NoteValidator,
	logger *slog.Logger,
) *NoteService {
	return &NoteService{
		noteModel:  noteModel,
		imageModel: imageModel,
		imageStore: imageStoreNote,
		validator:  validator,
		logger:     logger,
	}
}

// CreateNoteInput represents the input for creating a note
type CreateNoteInput struct {
	Title       string
	Content     string
	NoteType    string
	Book        string
	Chapter     int
	StartVerse  int
	EndVerse    int
	StartOffset int
	EndOffset   int
}

// CreateNote handles validation and insertion based on note type.
// Returns validation errors, duplicate errors, or database errors.
func (s *NoteService) CreateNote(userID int64, input CreateNoteInput) (*data.NoteResponse, *validator.Validator, error) {
	// 1. Convert input to domain models
	content := &data.NoteContent{
		UserID:   userID,
		Title:    strings.TrimSpace(input.Title),
		Content:  input.Content,
		NoteType: input.NoteType,
	}
	location := &data.NoteLocation{
		Book:        input.Book,
		Chapter:     input.Chapter,
		StartVerse:  input.StartVerse,
		EndVerse:    input.EndVerse,
		StartOffset: input.StartOffset,
		EndOffset:   input.EndOffset,
	}

	// 2. Validate based on note type
	v := s.validator.ValidateNoteCreation(content, location)
	if !v.Valid() {
		return nil, v, nil
	}

	// 3. Create note based on type
	var note *data.NoteResponse
	var err error

	switch content.NoteType {
	case "GENERAL":
		note, err = s.noteModel.InsertGeneral(content)
	case "BIBLE", "CROSS_REFERENCE":
		note, err = s.noteModel.InsertLocated(content, location)
	default:
		return nil, nil, ErrInvalidNoteType
	}

	if err != nil {
		s.logger.Error("failed to create note",
			"user_id", userID,
			"note_type", content.NoteType,
			"error", err)
		return nil, nil, err
	}

	return note, nil, nil
}

// DeleteNote handles note deletion with S3 cleanup
// Business Rule: Delete S3 images first, then database
// Rationale: Prevent orphaned S3 objects that cose money
func (s *NoteService) DeleteNote(userID, noteID int64) error {
	// 1. Check ownership (authorization)
	exists, err := s.noteModel.ExistsForUser(noteID, userID)
	if err != nil {
		return fmt.Errorf("check note exists: %w", err)
	}

	if !exists {
		return ErrNoteNotFound
	}

	// 2. Get all images associated with this note
	images, err := s.imageModel.GetForNote(noteID)
	if err != nil && !errors.Is(err, data.ErrRecordNotFound) {
		s.logger.Error("failed to retrieve images for deletion",
			"note_id", noteID,
			"error", err)
		return fmt.Errorf("get note images: %w", err)
	}

	// 3. Delete from s3 first (most likely to fail)
	if len(images) > 0 {
		if err := s.deleteImagesFromS3(images, noteID); err != nil {
			// If S3 deletion fails, abort - don't touch database
			return err
		}
	}

	// 4. All S3 deletion successful - safe to delete from database
	if err := s.noteModel.Delete(noteID, userID); err != nil {
		// S3 is deleted but DB failed - log for manual cleanup
		s.logger.Error("DB deletion failed after S3 cleanup",
			"note_id", noteID,
			"user_id", userID,
			"image_count", len(images),
			"error", err)
		return fmt.Errorf("delete note from database: %w", err)
	}

	s.logger.Info("note deleted successfully",
		"note_id", noteID,
		"user_id", userID,
		"images_deleted", len(images),
	)

	return nil
}

// deleteImagesFromS3 deletes all images from S3 storage for specified note
// Returns error if any deletion fails
func (s *NoteService) deleteImagesFromS3(images []*data.ImageData, noteID int64) error {
	var failedKeys []string

	for _, image := range images {
		// Create timeout context for each S3 operation
		deleteCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := s.imageStore.DeleteImage(deleteCtx, image.S3Key)
		cancel()

		if err != nil {
			failedKeys = append(failedKeys, image.S3Key)
			s.logger.Error("failed to delete image",
				"error", err,
				"s3_key", image.S3Key,
				"note_id", noteID,
			)
		}
	}

	//If any S3 deletion falied, abort the entire operation
	if len(failedKeys) > 0 {
		s.logger.Error("aboring note deletion due to S3 failures",
			"note_id", noteID,
			"failed_count", len(failedKeys),
		)
		return fmt.Errorf("failed to delete %d image(s) from S3, total: %d", len(failedKeys), len(images))
	}

	return nil
}

// GetNote retrieves a single specified note
// Returns note, image data with presigned url with three hour, and error
func (s *NoteService) GetNote(userID, noteID int64) (*data.NoteResponse, []*data.ImageData, error) {
	// Retrieve note from database - automatically filters by user_id for security
	note, err := s.noteModel.Get(userID, noteID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Note doesn't exist or doesn't belong to this user
			return nil, nil, ErrNoteNotFound
		}
		return nil, nil, err
	}

	var imageData []*data.ImageData
	imageData, err = s.imageModel.GetForNote(noteID)
	if err != nil {
		if !errors.Is(err, data.ErrRecordNotFound) {
			// Log error but don't fail request if no images found
			// Notes without images are valid and should still be returned
			s.logger.Error("failed to retrieve images", "error", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, image := range imageData {
		presignedURL, err := s.imageStore.GeneratePresignedURL(ctx, image.S3Key, 3*time.Hour)
		if err != nil {
			// Log error but continue - frontend can handle missing URLs
			s.logger.Error("failed to generate presigned url", "error", err)
		}
		image.PresignedURL = presignedURL
	}

	return note, imageData, nil
}

// UpdateNote validates and updates a specified note
// Returns the updated note, validation and error
func (s *NoteService) UpdateNote(content *data.NoteContent) (*data.NoteResponse, *validator.Validator, error) {

	v := s.validator.ValidateUpdateNote(content)
	if !v.Valid() {
		return nil, v, nil
	}

	note, err := s.noteModel.Update(content)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Could be: note doesn't exist, wrong user, OR wrong note_type
			return nil, nil, ErrNoteNotFound
		}

		return nil, nil, err
	}

	return note, nil, nil
}

// LinkNote validates and links a note to a given location
// Returns linked note location, validation and error
func (s *NoteService) LinkNote(noteLinkLocation *data.NoteInputLocation) (*data.NoteResponse, *validator.Validator, error) {

	v := s.validator.ValidateNoteLink(noteLinkLocation)
	if !v.Valid() {
		return nil, v, nil
	}

	linkedNote, err := s.noteModel.Link(noteLinkLocation)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, nil, ErrNoteNotFound
		}
		return nil, nil, err
	}

	return linkedNote, nil, nil
}

// func (s *NoteService) DeleteLink(noteID, linID)

// ListNotesInput contains parameters for listing notes
type ListNotesInput struct {
	NoteType string
	Page     int
	PageSize int
	Sort     string
}

// ListNotesMetadata retrieves paginated notes with validation
func (s *NoteService) ListNotesMetadata(userID int64, input ListNotesInput) ([]*data.NoteMetadata, *validator.Validator, error) {
	// 1. Validate usering existing validator
	v := validator.New()

	// Generic pagination validation (reusable)
	filters := data.Filters{
		Page:     input.Page,
		PageSize: input.PageSize,
		Sort:     input.Sort,
	}
	filters.Validate(v)

	// Domain-specific validation (note business rules)
	s.validateNoteListParams(v, input.NoteType, input.Sort)

	if !v.Valid() {
		return nil, v, nil
	}

	// 2. Build query params
	queryParams := &data.NoteQueryParams{
		Filters: data.Filters{
			Page:     input.Page,
			PageSize: input.PageSize,
			Sort:     input.Sort,
			// SortSafeList: ,
		},
		NoteType: input.NoteType,
	}

	// 3. Call repository
	notesMetadata, err := s.noteModel.GetAllMetadata(userID, queryParams)
	if err != nil {
		s.logger.Error("failed to list notes", "user_id", userID, "error", err)
		return nil, nil, fmt.Errorf("list notes: %w", err)
	}

	return notesMetadata, nil, nil
}

// validateNoteListParams validates note-specific list parameters
func (s *NoteService) validateNoteListParams(v *validator.Validator, noteType, sort string) {
	// Validate note type (domain rule)
	validNoteTypes := []string{data.NoteTypeGeneral, data.NoteTypeBible}
	v.Check(slices.Contains(validNoteTypes, noteType), "note_type", "must be GENERAL or BIBLE")

	// Validate sort fields (domain rule)
	validSortFields := []string{"created_at", "-created_at", "title", "-title"}
	if sort != "" {
		v.Check(slices.Contains(validSortFields, sort), "sort", "invalid sort field")
	}

	// Business rule: BIBLE notes can't be sorted by title
	if noteType == data.NoteTypeBible && (sort == "title" || sort == "-title") {
		v.AddError("sort", "BIBLE notes cannot be sorted by title")
	}
}

// getSortSafeListForNoteType returns allowed sort fields for note type
func (s *NoteService) getSortSafeListForNoteType(noteType string) []string {
	if noteType == data.NoteTypeBible {
		return []string{"created_at", "-created_at"}
	}
	return []string{"created_at", "-created_at", "title", "-title"}
}

type SearchInput struct {
	SearchQuery string
	Page        int
	PageSize    int
}

// SearchNotes searches notes and validates search input.
// Returns search notes results, metadata, validation and error
func (s *NoteService) SearchNotes(userID int64, input SearchInput) ([]*data.NoteSearchResponse, data.Metadata, *validator.Validator, error) {
	v := validator.New()

	filter := data.Filters{
		Page:     input.Page,
		PageSize: input.PageSize,
	}
	filter.Validate(v)

	if !v.Valid() {
		return nil, data.Metadata{}, v, nil
	}

	results, metadata, err := s.noteModel.SearchNotes(userID, input.SearchQuery, &filter)
	if err != nil {
		return nil, data.Metadata{}, nil, err
	}

	return results, metadata, nil, nil

}

func (s *NoteService) DeleteLink(userID, noteID, locationID int64) (*validator.Validator, error) {
	v := s.validator.validateDeleteLink(noteID, locationID, userID)
	if !v.Valid() {
		return v, nil
	}

	err := s.noteModel.DeleteLink(noteID, locationID, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}

	return nil, nil
}

func (nv *NoteValidator) validateDeleteLink(noteID, locationID, userID int64) *validator.Validator {
	v := validator.New()
	v.Check(noteID > 0, "note_id", "must provide valid id")
	v.Check(locationID > 0, "location_id", "must provide valid id")
	v.Check(userID > 0, "user_id", "must provide valid id")

	return v
}
