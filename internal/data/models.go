package data

import (
	"database/sql"
	"errors"
)

const UniqueViolation = "23505"

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type LocationFilters struct {
	Book       string
	Chapter    int
	StartVerse int
	EndVerse   int
}

type Models struct {
	Passages   PassageModel
	Highlights HighlightModel
	Users      UserModel
	Tokens     TokenModel
	Notes      NoteModel
	NoteImages ImageModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Passages:   NewPassageModel(db),
		Highlights: NewHighlightModel(db),
		Users:      NewUserModel(db),
		Tokens:     NewTokenModel(db),
		Notes:      NewNoteModel(db),
		NoteImages: NewImageModel(db),
	}
}
