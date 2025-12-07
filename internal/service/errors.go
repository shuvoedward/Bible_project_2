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
	ErrDuplicateEmail   = errors.New("user with this email already exists")
	ErrTokenNotFound    = errors.New("invalid or expired token")
	ErrUserNotActivated = errors.New("user account not is not active")
	ErrEmailNotFound    = errors.New("email invalid")
	ErrPasswordNotMatch = errors.New("password did not match")
	ErrUserActivated    = errors.New("user has already been activated")
)

var (
	ErrHighlightNotFound = errors.New("highlight not found")
	ErrPassageNotFound   = errors.New("passage not found")
)

// AutocompleteService errors
var (
	ErrEmptyQuery = errors.New("query empty")
)

// ImageService errors
var (
	ErrUnsupportedImageFormat = errors.New("unsupported image format")
)
