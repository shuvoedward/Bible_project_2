package service

import "errors"

// Service-level errors (no ValidationError needed!)
var (
	ErrInvalidNoteType = errors.New("invalid note type")
	ErrLinkNotFound    = errors.New("link not found")
	ErrNoteNotFound    = errors.New("note not found")
	ErrUnauthorized    = errors.New("unauthorized access")
)

var (
	ErrDuplicateEmail = errors.New("user with this email already exists")
	ErrTokenNotFound  = errors.New("invalid or expired token")
)
