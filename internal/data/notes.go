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

var ErrDuplicateTitleGeneral = errors.New("a general note with this title already exists for this user")
var ErrLocationAlreadyLinked = errors.New("this note already linked to this location")
var ErrDuplicateContent = errors.New("a note with this content already exists")

type NoteModel interface {
	GetAllLocatedForChapter(userID int64, filter *LocationFilters) ([]*NoteResponse, []*NoteResponse, error)
	Get(userID int64, id int64) (*NoteResponse, error)
	GetAllMetadata(userID int64, filter *NoteQueryParams) ([]*NoteMetadata, error)
	InsertLocated(content *NoteContent, location *NoteLocation) (*NoteResponse, error)
	InsertGeneral(note *NoteContent) (*NoteResponse, error)
	ExistsForUser(id int64, userID int64) (bool, error)
	Delete(id int64, userID int64) error
	Update(content *NoteContent) (*NoteResponse, error)

	DeleteLink(note_id, location_id, userID int64) error
	Link(input *NoteInputLocation) (*NoteResponse, error)
	SearchNotes(userID int64, word string, filter *Filters) ([]*NoteSearchResponse, Metadata, error)
}

type NoteContent struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"_"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	NoteType  string    `json:"note_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

type NoteMetadata struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Preview   string    `json:"Preview"`
	NoteType  string    `json:"note_type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NoteInputLocation struct {
	NoteID      int64
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

// GetAll retrieves all notes metadata for a specific user filtered by note type.
// Returns a slice of notes metadata matching the filter criteria with pagination and sorting applied.
// If no notes are found, returns an empty slice (not an error).
//
// Parameters:
//   - userID: The ID of teh user who owns the note
//   - *NoteQueryParams: The NoteQueryParams struct
//
// Returns:
//   - []*NoteMetadata: Slice of retreived note
//   - error: any database error that occured
func (m noteModel) GetAllMetadata(userID int64, filter *NoteQueryParams) ([]*NoteMetadata, error) {
	query := fmt.Sprintf(`
		SELECT 
			id, title, 
			SUBSTRING(content, 1, 200) as preview, 
			note_type, created_at, updated_at
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

	notesMetadata := []*NoteMetadata{}

	for rows.Next() {
		var metadata NoteMetadata
		err := rows.Scan(
			&metadata.ID,
			&metadata.Title,
			&metadata.Preview,
			&metadata.NoteType,
			&metadata.CreatedAt,
			&metadata.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		notesMetadata = append(notesMetadata, &metadata)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return notesMetadata, nil
}

func (m noteModel) Update(content *NoteContent) (*NoteResponse, error) {
	// Design: Content hashing strategy
	// 	GENERAL notes:
	//   - Same content allowed (different titles for context)
	//   - No content hashing needed
	//   - Only title must be unique per user
	//  BIBLE/CROSS_REFERENCE notes:
	//   - Must hash content to prevent duplicate annotations on same verse
	//   - Content must be unique (prevent spam/abuse)

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
			AND note_type = $7
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

	args := []any{content.ID, content.UserID, content.Title, content.Content, newHash, time.Now(), content.NoteType}

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
			// This happens when:
			// 1. Note doesn't exist
			// 2. Note doesn't belong to user
			// 3. Note type doesn't match (security check)
			return nil, ErrRecordNotFound
		}

		return nil, err
	}

	responseNote.Location = nil
	return &responseNote, nil
}

func (m noteModel) InsertLocated(content *NoteContent, location *NoteLocation) (*NoteResponse, error) {
	// BIBLE/CROSS_REFERENCE notes:
	//   - Must hash content to prevent duplicate annotations on same verse
	//   - Content must be unique (prevent spam/abuse)

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

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, insertQuery, insertArgs...).Scan(
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

	return &responseNote, nil
}

func (m noteModel) InsertGeneral(note *NoteContent) (*NoteResponse, error) {
	/*
		GENERAL notes:
		  - Same content allowed (different titles for context)
		  - No content hashing needed
		  - Only title must be unique per user
	*/
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

func (m noteModel) ExistsForUser(id int64, userID int64) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM 
				notes
			WHERE 
				id = $1
				AND user_id = $2	
			)
		`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var exists bool

	err := m.DB.QueryRowContext(ctx, query, id, userID).Scan(&exists)
	if err != nil {
		return false, nil
	}

	return exists, nil

}

func (m noteModel) Delete(id int64, userID int64) error {
	query := `
		DELETE FROM 
			notes
		WHERE 
			id = $1
			AND user_id = $2
		`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return err
	}

	return nil
}

// Link creates a new location link for an existing note at the specified Bible verse location.
// It validates that the note belongs to the user before creating the link.
// Returns the note ID and the created location details.
func (m noteModel) Link(input *NoteInputLocation) (*NoteResponse, error) {
	// Use SELECT with JOIN to validate note ownership and book existence in single query.
	// If note doesn't belong to user or book doesn't exist, no rows returned and insert fails.
	query := `
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
			RETURNING 
				note_id, $3, chapter, start_verse, end_verse, start_offset, end_offset
	`

	args := []any{input.NoteID, input.UserID, input.Book, input.Chapter, input.StartVerse,
		input.EndVerse, input.StartOffset, input.EndOffset}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var responseNote NoteResponse
	responseNote.Location = &LocationResponse{}

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&responseNote.ID,
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
			// Either note doesn't exist, doesn't belong to user, or book name is invalid
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &responseNote, nil
}

// DeleteLink removes a location link from a note.
// Validates that the location belongs to the specified note and that the note belongs to the user.
// Returns ErrRecordNotFound if the location doesn't exist or doesn't belong to the user's note.
func (m noteModel) DeleteLink(note_id, location_id, userID int64) error {
	// Join with notes table to verify note ownership before deleting the location.
	// This prevents users from deleting locations of notes they don't own.
	query := `
		DELETE FROM 
			note_locations nl
		USING 
			notes n
		WHERE nl.id = $1 
			AND nl.note_id = $3 
			AND nl.note_id = n.id
			AND n.user_id = $2
		`

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
	// No rows affected means either location doesn't exist,
	// doesn't belong to this note, or note doesn't belong to user
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// SearchNotes performs full-text search on user's notes using PostgreSQL's ts_rank and ts_headline.
// Returns notes ordered by relevance with highlighted snippets showing matched terms.
// The snippet contains 10-20 words with matched terms wrapped in <mark> tags.
func (m noteModel) SearchNotes(userID int64, searchQuery string, filter *Filters) ([]*NoteSearchResponse, Metadata, error) {
	query := `
	WITH counted AS (
		SELECT 
			id, title, note_type, created_at, updated_at, 	
			ts_rank(note_vector, websearch_to_tsquery('english', $2)) AS rank,
			ts_headline('english', 
			COALESCE(title, '') || ' ' || COALESCE(content, ''), 
			websearch_to_tsquery('english', $2),
			'MaxWords = 20, MinWords=10, MaxFragments=1, StartSel=<mark>, StopSel=</mark>'
			) AS snippet
			COUNT(*) OVER() AS total_count
		FROM 
			notes
		WHERE
			user_id = $1
			AND note_vector @@ websearch_to_tsquery('english', $2)
		)
		SELECT * FROM counted
		ORDER BY
			rank DESC
		LIMIT $3
		OFFSET $4
	`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID, searchQuery, filter.limit(), filter.offset())
	if err != nil {
		return nil, Metadata{}, err
	}
	defer rows.Close()

	results := []*NoteSearchResponse{}
	var totalCount int

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
			&totalCount,
		)

		if err != nil {
			return nil, Metadata{}, err
		}

		result.Location = nil

		results = append(results, &result)
	}

	if err = rows.Err(); err != nil {
		return nil, Metadata{}, err
	}

	metadata := calculateMetadata(totalCount, filter.Page, filter.PageSize)

	return results, metadata, nil

}
