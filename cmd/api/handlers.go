package main

import "shuvoedward/Bible_project/internal/service"

// Handlers contains all HTTP methods
// This is specific to the HTTP API entry point
type Handlers struct {
	Note      *NoteHandler
	User      *UserHandler
	Token     *TokenHandler
	Highlight *HighlightHandler
	Book      *BookHandler
}

// NewHandlers creates all HTTP handlers
// Handlers are tied to HTTP - not reusable like services
func NewHandlers(app *application, services *service.Service) *Handlers {
	return &Handlers{
		Note:      NewNoteHandler(app, services.Note),
		User:      NewUserHandler(app, services.User),
		Token:     NewTokenService(app, services.Token),
		Highlight: NewHighlightHandler(app, services.Highlight),
		Book:      NewBookHandler(app, services.Book, services.Autocomplete),
	}
}
