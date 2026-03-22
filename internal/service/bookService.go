package service

import (
	"context"
	"errors"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"sync"
	"time"
)

type passageReader interface {
	Get(ctx context.Context, filters *data.LocationFilters) (*data.Passage, error)
	SearchVersesByWord(ctx context.Context, searchQuery string, filters data.Filters) ([]*data.VerseMatch, data.Metadata, error)
}

type highlightReader interface {
	Get(ctx context.Context, userID int64, filter *data.LocationFilters) ([]*data.Highlight, error)
}

type noteReader interface {
	GetAllLocatedForChapter(ctx context.Context, userID int64, filter *data.LocationFilters) ([]*data.NoteResponse, []*data.NoteResponse, error)
}

type BookService struct {
	passageModel   passageReader
	highlightModel highlightReader
	noteModel      noteReader
	validator      *BibleValidator
	logger         *slog.Logger
}

func NewBookService(
	passageModel passageReader,
	highlightModel highlightReader,
	noteModel noteReader,
	validator *BibleValidator,
	logger *slog.Logger,
) *BookService {
	return &BookService{
		passageModel:   passageModel,
		highlightModel: highlightModel,
		noteModel:      noteModel,
		validator:      validator,
		logger:         logger,
	}
}

type PassageResponse struct {
	Passage       *data.Passage
	Highlights    []*data.Highlight
	BibleNotes    []*data.NoteResponse
	CrossRefNotes []*data.NoteResponse
}

// GetPassageWithUserData handles validation and retrieves passage, and highlights, Bible notes and cross-ref notes if user authenticated and active
// Returns PassageResponse, validation error and error
func (s *BookService) GetPassageWithUserData(ctx context.Context, userID int64, isAuthenticated bool, filter *data.LocationFilters) (*PassageResponse, *validator.Validator, error) {
	v := validator.New()
	s.validator.ValidateBook(v, filter.Book, filter.Chapter, filter.StartVerse, filter.EndVerse)
	if !v.Valid() {
		return nil, v, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	passage, err := s.passageModel.Get(ctx, filter)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			return nil, nil, ErrPassageNotFound
		}
		return nil, nil, err
	}

	response := &PassageResponse{
		Passage:       passage,
		Highlights:    []*data.Highlight{},
		BibleNotes:    []*data.NoteResponse{},
		CrossRefNotes: []*data.NoteResponse{},
	}

	if !isAuthenticated {
		return response, nil, nil
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		highlights, err := s.highlightModel.Get(ctx, userID, filter)
		if err != nil {
			s.logger.Error("failed to get highlights", "error", err)
		} else {
			response.Highlights = highlights
		}
	}()

	go func() {
		defer wg.Done()
		bibleNotes, crossRefNotes, err := s.noteModel.GetAllLocatedForChapter(ctx, userID, filter)
		if err != nil {
			s.logger.Error("failed to get notes", "error", err)
		} else {
			response.BibleNotes = bibleNotes
			response.CrossRefNotes = crossRefNotes
		}
	}()

	wg.Wait()

	return response, nil, nil
}

func (s *BookService) SearchVersesByWord(
	ctx context.Context,
	searchQuery string,
	filters data.Filters) ([]*data.VerseMatch, data.Metadata, error) {

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	verses, metadata, err := s.passageModel.SearchVersesByWord(ctx, searchQuery, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	return verses, metadata, nil
}
