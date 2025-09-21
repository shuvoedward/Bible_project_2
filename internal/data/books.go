package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PassageModel interface {
	Get(filters *PassageFilters) (*Passage, error)
}

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

type passageModel struct {
	DB *sql.DB
}

func NewPassageModel(db *sql.DB) *passageModel {
	return &passageModel{DB: db}
}

func (p *passageModel) Get(filters *PassageFilters) (*Passage, error) {
	switch {
	case filters.StartVerse != 0 && filters.EndVerse != 0:
		return p.getVerseRange(filters)
	default:
		return p.getChapter(filters)
	}
}

func (p *passageModel) getVerseRange(filters *PassageFilters) (*Passage, error) {
	query := `
			SELECT  v.verse, v.text
			FROM verses as v
			JOIN books AS b ON b.id = v.book_id
			WHERE b.name = $1 AND v.chapter = $2 AND v.verse BETWEEN $3 AND $4`

	return p.queryVerses(query, filters.Book, filters.Chapter, filters.StartVerse, filters.EndVerse)
}

func (p *passageModel) getChapter(filters *PassageFilters) (*Passage, error) {
	query := `
			SELECT v.verse, v.text
			FROM verses as v
			JOIN books As b ON v.book_id = b.id
			WHERE b.name = $1 AND v.chapter = $2`

	return p.queryVerses(query, filters.Book, filters.Chapter)
}

func (p *passageModel) queryVerses(query string, args ...any) (*Passage, error) {
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
		if err := rows.Scan(&verseDetail.Number, &verseDetail.Text); err != nil {
			return nil, err
		}
		passage.Verses = append(passage.Verses, verseDetail)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	if len(passage.Verses) == 0 {
		fmt.Println(passage)
		return nil, ErrRecordNotFound
	}

	return passage, nil
}
