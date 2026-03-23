package data

import (
	"context"
	"database/sql"
	"errors"
)

const UniqueViolation = "23505"

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

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
	db         *sql.DB
}

func NewModels(db *sql.DB) Models {
	return Models{
		Passages:   NewPassageModel(db),
		Highlights: NewHighlightModel(db),
		Users:      NewUserModel(db),
		Tokens:     NewTokenModel(db),
		Notes:      NewNoteModel(db),
		NoteImages: NewImageModel(db),
		db:         db,
	}
}

func (m Models) WithTx(ctx context.Context, fn func(Models) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txModels := Models{
		Users:  NewUserModel(tx),
		Tokens: NewTokenModel(tx),
	}

	err = fn(txModels)
	if err != nil {
		return err
	}

	return tx.Commit()
}
