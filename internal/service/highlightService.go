package service

import (
	"errors"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
)

type HighlightService struct {
	highlightModel data.HighlightModel
	bibleValidator *BibleValidator
	logger         *slog.Logger
}

func NewHighlightService(highlightModel data.HighlightModel, bibleValidator *BibleValidator, logger *slog.Logger) *HighlightService {
	return &HighlightService{
		highlightModel: highlightModel,
		bibleValidator: bibleValidator,
		logger:         logger,
	}
}

// InsertHighlight validates highlight inputs, creates a new highlight in the database
// Returns validation and error, highlight is populated in place
func (s *HighlightService) InsertHighlight(highlight *data.Highlight, userID int64) (*validator.Validator, error) {
	// validation error
	v := validator.New()

	s.bibleValidator.ValidateBibleLocation(
		v,
		highlight.Book,
		highlight.Chapter,
		highlight.StartVerse,
		highlight.EndVerse,
		*highlight.StartOffset, // Note: Highlight uses *int, convert as needed
		*highlight.EndOffset,
	)

	v.Check(userID > 0, "user_id", "must be valid")
	v.Check(highlight.Color != "", "color", "must be provided")

	if !v.Valid() {
		return v, nil
	}

	highlight.UserID = &userID

	err := s.highlightModel.Insert(highlight)
	if err != nil {
		s.logger.Error("failed to create highlight", "user_id", userID, "error", err)
		return nil, err
	}

	return nil, nil
}

func (s *HighlightService) UpdateHighlight(highlightID, userID int64, color string) (*validator.Validator, error) {
	v := validator.New()
	v.Check(highlightID > 0, "highlightID", "must be valid")
	v.Check(color != "", "color", "must be provided")

	// Optional: Add format validation for color (hex, rgb, etc.)
	// v.Check(isValidColor(input.Color), "color", "must be a valid color format")

	if !v.Valid() {
		return v, nil
	}

	err := s.highlightModel.Update(highlightID, userID, color)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrHighlightNotFound
		}
		return nil, err
	}

	return nil, nil
}

func (s *HighlightService) DeleteHighlight(highlightID, userID int64) (*validator.Validator, error) {
	v := validator.New()
	v.Check(highlightID > 0, "highlightID", "must be valid")
	if !v.Valid() {
		return v, nil
	}

	err := s.highlightModel.Delete(highlightID, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, ErrHighlightNotFound
		}
		return nil, err
	}

	return nil, nil
}
