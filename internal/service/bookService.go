package service

import (
	"errors"
	"log/slog"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/validator"
	"sync"
)

type BookService struct {
	passageModel   data.PassageModel
	highlightModel data.HighlightModel
	noteModel      data.NoteModel
	validator      *BibleValidator
	logger         *slog.Logger
}

func NewBookService(
	passgeModel data.PassageModel,
	highlightModel data.HighlightModel,
	noteModel data.NoteModel,
	validator *BibleValidator,
	logger *slog.Logger,
) *BookService {
	return &BookService{
		passageModel:   passgeModel,
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
func (s *BookService) GetPassageWithUserData(userID int64, isAuthenticated bool, filter *data.LocationFilters) (*PassageResponse, *validator.Validator, error) {
	v := validator.New()
	s.validator.ValidateBook(v, filter.Book, filter.Chapter, filter.StartVerse, filter.EndVerse)
	if !v.Valid() {
		return nil, v, nil
	}

	passage, err := s.passageModel.Get(filter)
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
		highlights, err := s.highlightModel.Get(userID, filter)
		if err != nil {
			s.logger.Error("failed to get highlights", "error", err)
		} else {
			response.Highlights = highlights
		}
	}()

	go func() {
		defer wg.Done()
		bibleNotes, crossRefNotes, err := s.noteModel.GetAllLocatedForChapter(userID, filter)
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
	searchQuery string,
	filters data.Filters) ([]*data.VerseMatch, data.Metadata, error) {
	verses, metadata, err := s.passageModel.SearchVersesByWord(searchQuery, filters)
	if err != nil {
		return nil, data.Metadata{}, err
	}

	return verses, metadata, nil
}
