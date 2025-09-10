package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
)

type PassageFilters struct {
	Book       string
	Chapter    int
	Verse      int
	StartVerse int
	EndVerse   int
}

type Models struct {
	Passages PassageModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Passages: NewPassageModel(db),
	}
}
