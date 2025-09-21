package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type PassageFilters struct {
	Book       string
	Chapter    int
	StartVerse int
	EndVerse   int
}

type Models struct {
	Passages PassageModel
	Users    UserModel
	Tokens   TokenModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Passages: NewPassageModel(db),
		Users:    NewUserModel(db),
		Tokens:   NewTokenModel(db),
	}
}
