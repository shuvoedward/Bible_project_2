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

const (
	NoteTypeGeneral  = "GENERAL"
	NoteTypeBible    = "BIBLE"
	NoteTypeCrossRef = "CROSS_REFERENCE"
)

const UniqueViolation = "23505"

var ErrDuplicateTitleGeneral = errors.New("a general note with this title already exists for this user")
var ErrLocationAlreadyLinked = errors.New("this note already linked to this location")
var ErrDuplicateContent = errors.New("a note with this content already exists")

type NoteModel interface {
	GetAllLocatedForChapter(userID int64, filter *LocationFilters) ([]*NoteResponse, []*NoteResponse, error)
	Get(userID int64, id int64) (*NoteResponse, error)
	GetAll(userID int64, filter *NoteQueryParams) ([]*NoteContent, error)
	InsertLocated(content *NoteContent, location *NoteLocation) (*NoteResponse, error)
	InsertGeneral(note *NoteContent) (*NoteResponse, error)
	Delete(id int64, userID int64) error
	Update(content *NoteContent) (*NoteResponse, error)
	Link(input *NoteInputLocation) (*NoteResponse, error)
	DeleteLink(note_id, location_id, userID int64) error

	SearchNotes(userID int64, word string, filter *Filters) ([]*NoteSearchResponse, error)
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

type NoteResponse struct {
	ID        int64     `json:"note_id"`
	UserID    int64     `json:"-"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	NoteType  string    `json:"note_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Location *LocationResponse `json:"location,omitempty"`
}

type NoteSearchResponse struct {
	NoteResponse
	Rank    float64 `json:"rank"`
	Snippet string  `json:"snippet"`
}

type NoteQueryParams struct {
	Filters
	NoteType string
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

// GetAllLocatedForChapter retrieves all notes (BIBLE and CROSS_REFERENCE)
// associated with a specific chapter and verse range for a given user.
// It ensures users have access to their own notes by filtering on both the note ID and userID.
// This prevents unauthorized access to notes belonging to other users.
//
// Parameters:
//
//   - userID: The ID of the user who owns the note
//   - *LocationFilters: The LocationFilters struct
//
// Returns:
//   - *NoteResponse: The retrieved Bible notes if found
//   - *NoteResponse: The retrieved Cross-reference note if found
//   - error: Any database error that occured.
func (m noteModel) GetAllLocatedForChapter(userID int64, filter *LocationFilters) ([]*NoteResponse, []*NoteResponse, error) {
	// SQL query to join notes, their locations, and the book name.
	// It selects notes that overlap with the requested verse range.
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

	BibleNotes := []*NoteResponse{}
	CrossRefNotes := []*NoteResponse{}

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

		locatedNote := &NoteResponse{
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

// Get retrieves a single note by its ID for a specific user.
// It ensures that users can only access their own notes by filtering on both
// the note ID and user ID. This prevents unauthorized access to notes belonging
// to other users.
//
// Parameters:
//   - userID: The ID of the user who owns the note
//   - id: The unique identifier of the note to retrieve
//
// Returns:
//   - *NoteResponse: The retrieved note if found
//   - error: ErrRecordNotFound if the note doesn't exist or doesn't belong to the user,
//     or any other database error that occurred
func (m noteModel) Get(userID int64, id int64) (*NoteResponse, error) {
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

	var note NoteResponse

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

func (m noteModel) GetAll(userID int64, filter *NoteQueryParams) ([]*NoteContent, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, user_id, title, content, note_type, created_at, updated_at
		FROM 
			notes
		WHERE
			user_id = $1
			AND note_type = $2
		ORDER BY
			%s %s	
		LIMIT $3
		OFFSET $4`, filter.sortColumn(), filter.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	args := []any{userID, filter.NoteType, filter.limit(), filter.offset()}

	rows, err := m.DB.QueryContext(ctx, query, args...)
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
			&note.Title,
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

func (m noteModel) Update(content *NoteContent) (*NoteResponse, error) {
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

	var responseNote NoteResponse

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

func (m noteModel) InsertLocated(content *NoteContent, location *NoteLocation) (*NoteResponse, error) {
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

	var responseNote NoteResponse
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

func (m noteModel) InsertGeneral(note *NoteContent) (*NoteResponse, error) {
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

	var responseNote NoteResponse

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

func (m noteModel) Link(input *NoteInputLocation) (*NoteResponse, error) {
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

	var responseNote NoteResponse
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

func (m noteModel) SearchNotes(userID int64, word string, filter *Filters) ([]*NoteSearchResponse, error) {
	query := `
		SELECT 
			id, title, note_type, created_at, updated_at, 	
			ts_rank(note_vector, websearch_to_tsquery('english', lower($2))) AS rank,
			ts_headline('english', 
			COALESCE(title, '') || ' ' || COALESCE(content, ''), 
			websearch_to_tsquery('english', $2),
			'MaxWords = 20, MinWords=10, MaxFragments=1, StartSel=<mark>, StopSel=</mark>'
			) AS snippet
		FROM 
			notes
		WHERE
			user_id = $1
			AND note_vector @@ websearch_to_tsquery('english', $2)
		ORDER BY
			rank DESC
		LIMIT $3
		OFFSET $4
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, word, filter.limit(), filter.offset())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	response := []*NoteSearchResponse{}

	for rows.Next() {
		var result NoteSearchResponse
		err := rows.Scan(
			&result.ID,
			&result.Title,
			&result.NoteType,
			&result.CreatedAt,
			&result.UpdatedAt,
			&result.Rank,
			&result.Snippet,
		)

		if err != nil {
			return nil, err
		}

		result.Location = nil

		response = append(response, &result)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return response, nil

}
