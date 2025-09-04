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
	query := `
        SELECT v.verse, v.text
        FROM verses as v
        JOIN books As b ON v.book_id = b.id
        WHERE b.name = $1 AND v.chapter = $2`

	args := []any{filters.Book, filters.Chapter}

	if filters.Verse != 0 {
		query += " AND v.verse = $3"
		args = append(args, filters.Verse)
	} else if filters.StartVerse != 0 && filters.EndVerse != 0 {
		query += " AND v.verse BETWEEN $3 AND $4"
		args = append(args, filters.StartVerse, filters.EndVerse)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := p.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	passage := Passage{
		Book:    filters.Book,
		Chapter: filters.Chapter,
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
		return nil, ErrRecordNotFound
	}

	return &passage, nil
}
