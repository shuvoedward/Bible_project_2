package service

import "errors"

// Service-level errors (no ValidationError needed!)
var (
	ErrInvalidNoteType = errors.New("invalid note type")
	ErrLinkNotFound    = errors.New("link not found")
	ErrNoteNotFound    = errors.New("note not found")
	ErrUnauthorized    = errors.New("unauthorized access")
)
