package main

import "shuvoedward/Bible_project/internal/service"

// Handlers contains all HTTP methods
// This is specific to the HTTP API entry point
type Handlers struct {
	Note *NoteHandler
}

// NewHandlers creates all HTTP handlers
// Handlers are tied to HTTP - not reusable like services
func NewHandlers(app *application, services *service.Service) *Handlers {
	return &Handlers{
		Note: NewNoteHandler(app, services.Note),
	}
}
