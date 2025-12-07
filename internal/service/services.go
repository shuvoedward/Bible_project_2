package service

import (
	"log/slog"
	"shuvoedward/Bible_project/internal/cache"
	"shuvoedward/Bible_project/internal/data"
	"shuvoedward/Bible_project/internal/mailer"
)

// Services contains all business logic services
type Service struct {
	Note         *NoteService
	User         *UserService
	Token        *TokenService
	Highlight    *HighlightService
	Book         *BookService
	Autocomplete *AutocompleteService
}

// NewServices creates all services with their dependencies
// Centralize service creation
func NewServices(
	models data.Models,
	logger *slog.Logger,
	s3Service ImageStorage,
	redisClient *cache.RedisClient,
	mailer *mailer.Mailer,
	books map[string]struct{},
	booksSearchIndex map[string][]string,
) *Service {
	noteValidator := NewNoteValidator(books)

	return &Service{
		Note: NewNoteService(
			models.Notes,
			models.NoteImages,
			s3Service,
			noteValidator,
			logger,
		),
		User: NewUserService(
			models.Users,
			models.Tokens,
			logger,
		),
		Token: NewTokenService(
			models.Tokens,
			models.Users,
			logger,
		),
		Highlight: NewHighlightService(
			models.Highlights,
			NewBibleValidator(books),
			logger,
		),
		Book: NewBookService(
			models.Passages,
			models.Highlights,
			models.Notes,
			NewBibleValidator(books),
			logger,
		),
		Autocomplete: NewAutocompleteService(
			models.Passages,
			booksSearchIndex,
		),
	}
}
