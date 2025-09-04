package data

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type VerseDetail struct {
	Number int    `json:"number"`
	Text   string `json:"text"`
}

// for whole chapter and few verses and for single verse
type Passage struct {
	Book    string        `json:"book"`
	Chapter int           `json:"chapter"`
	Verses  []VerseDetail `json:"verses"`
}

type PassageModel struct {
	DB *sql.DB
}

func NewPassageModel(db *sql.DB) PassageModel {
	return PassageModel{DB: db}
}

func (p *PassageModel) Get(filters PassageFilters) (*Passage, error) {

	if filters.Verse != 0 {
		return p.getSingleVerse(filters)
	} else if filters.StartVerse != 0 && filters.EndVerse != 0 {
		return p.getVerseRange(filters)
	} else {
		return p.getChapter(filters)
	}
}

func (p *PassageModel) getSingleVerse(filters PassageFilters) (*Passage, error) {
	query := `
		SELECT v.verse, v.text
		FROM verses as v
		JOIN books as b ON v.book_id = b.id
		WHERE b.name = $1 AND v.chapter = $2 AND v.verse = $3`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{filters.Book, filters.Chapter, filters.Verse}

	var verseDetail VerseDetail

	err := p.DB.QueryRowContext(ctx, query, args...).Scan(
		&verseDetail.Number,
		&verseDetail.Text,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &Passage{
		Book:    filters.Book,
		Chapter: filters.Chapter,
		Verses:  []VerseDetail{verseDetail},
	}, nil

}

func (p *PassageModel) getVerseRange(filters PassageFilters) (*Passage, error) {
	query := `
			SELECT  v.verse, v.text
			FROM verses as v
			JOIN books AS b ON b.id = v.book_id
			WHERE b.name = $1 AND v.chapter = $2 AND v.verse BETWEEN $3 AND $4`

	return p.queryVerses(query, filters.Book, filters.Chapter, filters.StartVerse, filters.EndVerse)
}

func (p *PassageModel) getChapter(filters PassageFilters) (*Passage, error) {
	query := `
			SELECT v.verse, v.text
			FROM verses as v
			JOIN books As b ON v.book_id = b.id
			WHERE b.name = $1 AND v.chapter = $2`
	return p.queryVerses(query, filters.Book, filters.Chapter)
}

func (p *PassageModel) queryVerses(query string, args ...any) (*Passage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	passage := &Passage{
		Book:    args[0].(string),
		Chapter: args[1].(int),
		Verses:  []VerseDetail{},
	}

	for rows.Next() {
		var verseDetail VerseDetail
		if err := rows.Scan(&verseDetail.Number, verseDetail.Text); err != nil {
			return nil, err
		}
		passage.Verses = append(passage.Verses, verseDetail)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(passage.Verses) == 0 {
		return nil, ErrRecordNotFound
	}

	return passage, nil
}
