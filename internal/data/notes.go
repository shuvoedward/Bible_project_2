package data

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

var ErrDuplicateTitleGeneral = errors.New("a general note with this title already exists for this user")
var ErrLocationAlreadyLinked = errors.New("this note already linked to this location")
var ErrDuplicateContent = errors.New("a note with this content already exists")

const UniqueViolation = "23505"

type NoteModel interface {
	GetAllLocatedForChapter(userID int64, filter *LocationFilters) ([]*LocatedNoteResponse, []*LocatedNoteResponse, error)
	Get(userID int64, id int64) (*LocatedNoteResponse, error)
	GetAll(userID int64, noteType string, limit, offset int) ([]*NoteContent, error)
	InsertLocated(content *NoteContent, location *NoteLocation) (*LocatedNoteResponse, error)
	InsertGeneral(note *NoteContent) (*LocatedNoteResponse, error)
	Delete(id int64, userID int64) error
	Update(content *NoteContent) (*LocatedNoteResponse, error)
	Link(input *NoteInputLocation) (*LocatedNoteResponse, error)
	DeleteLink(note_id, location_id, userID int64) error
}

type NoteContent struct {
	ID        int64
	UserID    int64
	Title     string
	Content   string
	NoteType  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NoteLocation struct {
	ID          int64
	NoteID      int64
	Book        string
	Chapter     int
	StartVerse  int
	EndVerse    int
	StartOffset int
	EndOffset   int
}

type NoteInputLocation struct {
	ID          int64
	UserID      int64
	Book        string
	Chapter     int
	StartVerse  int
	EndVerse    int
	StartOffset int
	EndOffset   int
}

type LocationResponse struct {
	ID          int64  `json:"location_id"`
	Book        string `json:"book"`
	Chapter     int    `json:"chapter"`
	StartVerse  int    `json:"start_verse"`
	EndVerse    int    `json:"end_verse"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
}

type LocatedNoteResponse struct {
	ID        int64     `json:"note_id"`
	UserID    int64     `json:"-"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	NoteType  string    `json:"note_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Location *LocationResponse `json:"location,omitempty"`
}

type noteModel struct {
	DB *sql.DB
}

func NewNoteModel(db *sql.DB) *noteModel {
	return &noteModel{DB: db}
}

func hashContent(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// Get notes for a verse ranges, Bible notes type, cross ref type
func (m noteModel) GetAllLocatedForChapter(userID int64, filter *LocationFilters) ([]*LocatedNoteResponse, []*LocatedNoteResponse, error) {
	query := `
		SELECT
			n.id, 
			n.user_id, 
			n.title, 
			CASE
				WHEN n.note_type = 'CROSS_REFERENCE' THEN n.content
				ELSE ''	
			END AS content, 
			n.note_type,
			n.created_at, 
			b.name, 
			nl.chapter, 
			nl.start_verse, 
			nl.end_verse, 
			nl.start_offset, 
			nl.end_offset
		FROM 
			notes AS n
		JOIN 
			note_locations AS  nl ON n.id = nl.note_id
		JOIN 
			books AS b ON nl.book_id = b.id
		WHERE 
			n.user_id = $1
			AND b.name = $2
			AND nl.chapter = $3
			AND nl.start_verse <= $5
			AND nl.end_verse >= $4
			AND n.note_type IN ('CROSS_REFERENCE', 'BIBLE')`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []any{userID, filter.Book, filter.Chapter, filter.StartVerse, filter.EndVerse}

	rows, err := m.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	BibleNotes := []*LocatedNoteResponse{}
	CrossRefNotes := []*LocatedNoteResponse{}

	for rows.Next() {
		var content NoteContent
		var location NoteLocation
		err := rows.Scan(
			&content.ID,
			&content.UserID,
			&content.Title,
			&content.Content,
			&content.NoteType,
			&content.CreatedAt,
			&location.Book,
			&location.Chapter,
			&location.StartVerse,
			&location.EndVerse,
			&location.StartOffset,
			&location.EndOffset,
		)

		if err != nil {
			return nil, nil, err
		}

		locatedNote := &LocatedNoteResponse{
			ID:        content.ID,
			Title:     content.Title,
			Content:   content.Content,
			NoteType:  content.NoteType,
			CreatedAt: content.CreatedAt,
		}

		locatedNote.Location = &LocationResponse{
			Book:        location.Book,
			Chapter:     location.Chapter,
			StartVerse:  location.StartVerse,
			EndVerse:    location.EndVerse,
			StartOffset: location.StartOffset,
			EndOffset:   location.EndOffset,
		}

		if locatedNote.NoteType == "BIBLE" {
			BibleNotes = append(BibleNotes, locatedNote)
		} else {
			CrossRefNotes = append(CrossRefNotes, locatedNote)
		}
	}

	if err = rows.Err(); err != nil {
		return nil, nil, err
	}

	return BibleNotes, CrossRefNotes, nil
}

func (m noteModel) Get(userID int64, id int64) (*LocatedNoteResponse, error) {
	query := `
		SELECT 
			id, user_id, title, content, note_type, created_at, updated_at
		FROM 
			notes
		WHERE 
			notes.id = $1 
			AND notes.user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var note LocatedNoteResponse

	err := m.DB.QueryRowContext(ctx, query, id, userID).Scan(
		&note.ID,
		&note.UserID,
		&note.Title,
		&note.Content,
		&note.NoteType,
		&note.CreatedAt,
		&note.UpdatedAt,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &note, nil
}

func (m noteModel) GetAll(userID int64, noteType string, limit, offset int) ([]*NoteContent, error) {
	query := `
		SELECT 
			id, userID, title, content, note_type, created_at, updated_at
		FROM 
			notes
		WHERE
			user_id = $1
			AND note_type = $2
		ORDER BY
			id ASC
		LIMIT $3
		OFFSET $4`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, noteType, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notes := []*NoteContent{}

	for rows.Next() {
		var note NoteContent
		err := rows.Scan(
			&note.ID,
			&note.UserID,
			&note.Content,
			&note.NoteType,
			&note.CreatedAt,
			&note.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		notes = append(notes, &note)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notes, nil
}

func (m noteModel) Update(content *NoteContent) (*LocatedNoteResponse, error) {
	query := `
		UPDATE 
			notes
		SET 
			title = $3,
			content = $4,
			content_hash = $5,
			updated_at = $6
		WHERE 
			id = $1 
			AND	user_id = $2
		RETURNING
			id, title, content, note_type, created_at, updated_at`

	var newHash *string

	if content.NoteType != "GENERAL" {
		hash := hashContent(content.Content)
		newHash = &hash
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var responseNote LocatedNoteResponse

	args := []any{content.ID, content.UserID, content.Title, content.Content, newHash, time.Now()}

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&responseNote.ID,
		&responseNote.Title,
		&responseNote.Content,
		&responseNote.NoteType,
		&responseNote.CreatedAt,
		&responseNote.UpdatedAt,
	)

	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique violation
				switch pgErr.Constraint {
				case "idx_general_notes_user_title":
					return nil, ErrDuplicateTitleGeneral
				case "idx_bible_notes_user_content_hash":
					return nil, ErrDuplicateContent
				default:
					return nil, ErrDuplicateContent
				}
			default:
				return nil, err
			}

		}

		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	responseNote.Location = nil

	return &responseNote, nil
}

func (m noteModel) InsertLocated(content *NoteContent, location *NoteLocation) (*LocatedNoteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	contentHash := hashContent(content.Content)

	// no need to return content-hash. it is only needed for backend?
	insertQuery := `
		WITH note_inserted AS(
			INSERT INTO notes
				(user_id, title, content, content_hash, note_type)
			VALUES
				($1, $2, $3, $4, $5)
			RETURNING 
				id AS note_id, title, content, note_type, created_at, updated_at
		),
		book_lookup AS(
			SELECT id AS book_id FROM books WHERE name = $6
		),
		location_inserted AS(
			INSERT INTO note_locations
			(note_id, book_id, chapter, start_verse, end_verse, start_offset, end_offset)  
		SELECT
			n.note_id,
			b.book_id,
			$7, $8, $9, $10, $11
		FROM note_inserted n
		CROSS JOIN book_lookup b 
		RETURNING * 
		)
		SELECT 
			n.note_id,
			n.title,
			n.content,
			n.note_type,
			n.created_at, 
			n.updated_at,
			$8 AS book,
			l.chapter,
			l.start_verse,
			l.end_verse,
			l.start_offset,
			l.end_offset
		FROM 
			note_inserted n
		CROSS JOIN 
			location_inserted l`

	var responseNote LocatedNoteResponse
	responseNote.Location = &LocationResponse{}

	insertArgs := []any{content.UserID, content.Title, content.Content, contentHash, content.NoteType,
		location.Book, location.Chapter, location.StartVerse, location.EndVerse, location.StartOffset, location.EndOffset}

	err = tx.QueryRowContext(ctx, insertQuery, insertArgs...).Scan(
		&responseNote.ID, &responseNote.Title, &responseNote.Content, &responseNote.NoteType,
		&responseNote.CreatedAt, &responseNote.UpdatedAt, &responseNote.Location.Book,
		&responseNote.Location.Chapter, &responseNote.Location.StartVerse, &responseNote.Location.EndVerse,
		&responseNote.Location.StartOffset, &responseNote.Location.EndOffset,
	)

	if err != nil {
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrDuplicateContent
		}
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &responseNote, nil
}

func (m noteModel) InsertGeneral(note *NoteContent) (*LocatedNoteResponse, error) {
	query := `
		INSERT INTO notes
			(user_id, title, content, note_type)	
		VALUES
			($1, $2, $3, $4)
		RETURNING 
			id, title, content, note_type, created_at, updated_at
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{note.UserID, note.Title, note.Content, note.NoteType}

	var responseNote LocatedNoteResponse

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&responseNote.ID,
		&responseNote.Title,
		&responseNote.Content,
		&responseNote.NoteType,
		&responseNote.CreatedAt,
		&responseNote.UpdatedAt,
	)

	if err != nil {
		var pgErr *pq.Error
		switch {
		case errors.As(err, &pgErr) && pgErr.Code == UniqueViolation:
			return nil, ErrDuplicateTitleGeneral
		default:
			return nil, err
		}
	}

	responseNote.Location = nil

	return &responseNote, nil
}

func (m noteModel) Delete(id int64, userID int64) error {
	query := `
		DELETE FROM notes
		WHERE 
			id = $1
			AND user_id = $2
		RETURNING 
			id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

func (m noteModel) Link(input *NoteInputLocation) (*LocatedNoteResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = tx.Rollback()
	}()

	findExistingNoteQuery := `
		SELECT EXISTS (
			SELECT 1
			FROM 
				note_locations l
			JOIN 
				books b ON l.book_id = b.id
			WHERE 
				l.note_id = $1
				AND b.name = $2	
				AND l.chapter = $3
				AND (
					(l.start_verse * 10000 + l.start_offset) <= ($5 * 10000 + $7)				
					AND 
					(l.end_verse * 10000 + l.end_offset) >= ($4 * 10000 + $6)
				)
		)
	`

	findExistingNoteArgs := []any{input.ID, input.Book, input.Chapter, input.StartVerse, input.EndVerse, input.StartOffset, input.EndOffset}

	var overlaps bool
	err = tx.QueryRowContext(ctx, findExistingNoteQuery, findExistingNoteArgs...).Scan(&overlaps)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}

	if overlaps {
		return nil, ErrLocationAlreadyLinked
	}

	insertQuery := `
		WITH inserted AS (
			INSERT INTO note_locations (
				note_id, book_id, chapter, start_verse,
				end_verse, start_offset, end_offset	
			)
			SELECT 
				n.id, b.id, $4, $5, $6, $7, $8
			FROM
				notes n
			INNER JOIN
				books b
				ON b.name = $3
			WHERE
				n.id = $1 
				AND n.user_id = $2
			RETURNING *
		)
		SELECT 
			n.id AS note_id, n.title, n.content, n.note_type, n.created_at, n.updated_at,
			b.name AS book_name,
			i.id, i.chapter, i.start_verse, i.end_verse, i.start_offset, i.end_offset
		FROM inserted i
		JOIN notes n ON n.id = i.note_id
		JOIN books b ON b.id = i.book_id
	`

	args := []any{input.ID, input.UserID, input.Book, input.Chapter, input.StartVerse,
		input.EndVerse, input.StartOffset, input.EndOffset}

	var responseNote LocatedNoteResponse
	responseNote.Location = &LocationResponse{}

	err = tx.QueryRowContext(ctx, insertQuery, args...).Scan(
		&responseNote.ID,
		&responseNote.Title,
		&responseNote.Content,
		&responseNote.NoteType,
		&responseNote.CreatedAt,
		&responseNote.UpdatedAt,
		&responseNote.Location.Book,
		&responseNote.Location.ID,
		&responseNote.Location.Chapter,
		&responseNote.Location.StartVerse,
		&responseNote.Location.EndVerse,
		&responseNote.Location.StartOffset,
		&responseNote.Location.EndOffset,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return &responseNote, nil
}

func (m noteModel) DeleteLink(note_id, location_id, userID int64) error {
	query := `
		DELETE FROM 
			note_locations nl
		USING 
			notes n
		WHERE nl.id = $1 
			AND nl.note_id = $3 
			AND nl.note_id = n.id
			AND n.user_id = $2
		RETURNING
			nl.id`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, location_id, userID, note_id)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}
