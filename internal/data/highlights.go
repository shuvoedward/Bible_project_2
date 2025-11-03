package data

import (
	"context"
	"database/sql"
	"time"
)

type HighlightModel interface {
	Insert(highlight *Highlight) error
	Get(userID int64, filter *LocationFilters) ([]*Highlight, error)
	Update(id, user_id int64, color string) error
	Delete(id, userId int64) error
}

type Highlight struct {
	ID          int64     `json:"id"`
	UserID      *int64    `json:"-" swaggerignore:"true"`
	Book        string    `json:"book"`
	Chapter     int       `json:"chapter"`
	StartVerse  int       `json:"start_verse"`
	EndVerse    int       `json:"end_verse"`
	StartOffset *int      `json:"start_offset"`
	EndOffset   *int      `json:"end_offset"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type highlightModel struct {
	DB *sql.DB
}

func NewHighlightModel(db *sql.DB) *highlightModel {
	return &highlightModel{
		DB: db,
	}
}

// Insert creates a new highlight record and populates the ID and CreatedAt fields
// of the provided highlight pointer.
func (m highlightModel) Insert(highlight *Highlight) error {
	query := `
		INSERT INTO highlights
			(user_id, book_id, chapter, start_verse, end_verse, start_offset, end_offset, color)
		SELECT
			$1, b.id, $3, $4, $5, $6, $7, $8
		FROM 
			books b
		WHERE 
			b.name = $2
		RETURNING id, created_at`

	args := []any{
		highlight.UserID,
		highlight.Book,
		highlight.Chapter,
		highlight.StartVerse,
		highlight.EndVerse,
		highlight.StartOffset,
		highlight.EndOffset,
		highlight.Color,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return m.DB.QueryRowContext(ctx, query, args...).Scan(&highlight.ID, &highlight.CreatedAt)

}

func (m highlightModel) Get(userID int64, filter *LocationFilters) ([]*Highlight, error) {
	if filter.StartVerse == 0 {
		filter.StartVerse = 1
		filter.EndVerse = 177
	}

	query := `
		SELECT 
		h.id, h.user_id, h.book_id, h.chapter, h.start_verse, h.end_verse, h.start_offset, h.end_offset, h.color, h.created_at, h.updated_at
		FROM highlights as h
		JOIN books as b	ON b.id = h.book_id
		WHERE h.user_id = $1
		AND b.name = $2
		AND h.chapter = $3
		AND (
			start_verse = 0
			OR
			(start_verse <= $5 AND end_verse >= $4)	
		)`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, filter.Book, filter.Chapter, filter.StartVerse, filter.EndVerse)
	if err != nil {
		return nil, err

	}
	defer rows.Close()

	var highlights []*Highlight

	for rows.Next() {
		var temp Highlight
		err := rows.Scan(
			&temp.ID,
			&temp.UserID,
			&temp.Book,
			&temp.Chapter,
			&temp.StartVerse,
			&temp.EndVerse,
			&temp.StartOffset,
			&temp.EndOffset,
			&temp.Color,
			&temp.CreatedAt,
			&temp.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		highlights = append(highlights, &temp)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return highlights, nil
}

// Update modifies the color of an existing highlight
// Returns ErrRecordNotFound if the highlight doesn't exist or doesn't belong to the user
func (m highlightModel) Update(id, userID int64, color string) error {
	query := `
		UPDATE 
			highlights
		SET 
			color = $1, 
			updated_at = now()
		WHERE id = $2 AND user_id = $3`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := m.DB.ExecContext(ctx, query, color, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected < 1 {
		return ErrRecordNotFound
	}
	return nil
}

// Delete removes a highlight from the database
// Returns ErrRecordNotFound if the highlight doesn't exist or doesn't belong to the user
func (m highlightModel) Delete(id, userID int64) error {
	query := `
		DELETE FROM highlights
		WHERE id = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	res, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected < 1 {
		return ErrRecordNotFound
	}

	return nil
}
