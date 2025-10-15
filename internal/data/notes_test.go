package data

import (
	"fmt"
	"testing"
)

func TestGetAllLocatedForChapter(t *testing.T) {
	// get note for cross-ref, Bible.
	// userID, locationFilter
	// first insert book, verses.
	fmt.Println("running note tests")
	testUser, err := createTestUser()
	if err != nil {
		t.Fatalf("failed to set password: %v", err)
	}
	testUser.ID = int64(123)

	err = insertTestUser(testUser)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	defer deleteTestUser(testUser.ID)

	// insert book, chapter, verses
	bookID, err := insertTestBook("Genesis")
	if err != nil {
		t.Fatalf("failed to insert test book: %v", err)
	}
	defer deleteTestBook(bookID)

	chapter := []string{
		"In the beginning God created the heavens and the earth.",
		"Now the earth was formless and void, and darkness was over the surface of the deep. And the Spirit of God was hovering over the surface of the waters.",
		"And God said, �Let there be light,� and there was light.",
		"And God saw that the light was good, and He separated the light from the darkness.",
		"God called the light �day,� and the darkness He called �night.� And there was evening, and there was morning�the first day.",
	}
	err = insertTestChapter(chapter, bookID)
	if err != nil {
		t.Fatalf("falied to insert test chapter %v", err)
	}
	defer deleteTestChapter(bookID, 1)

	content := &NoteContent{
		UserID:   testUser.ID,
		Title:    "test Bible note 1",
		Content:  "test Bible content 1",
		NoteType: "BIBLE",
	}
	location := &NoteLocation{
		Chapter:     1,
		StartVerse:  1,
		EndVerse:    1,
		StartOffset: 1,
		EndOffset:   5,
	}

	// insert Bible note
	noteID, locationID, err := insertTestLocatedNote(content, location, bookID)
	if err != nil {
		t.Fatalf("failed to insert test Bible note: %v", err)
	}
	defer deleteTestLocatedNote(*noteID, *locationID)

	// locationFilter
	filter := LocationFilters{
		Book:       "Genesis",
		Chapter:    1,
		StartVerse: 1,
		EndVerse:   4,
	}

	fmt.Println("here")
	m := NewModels(testDB)
	BibleNotes, crossRefNotes, err := m.Notes.GetAllLocatedForChapter(testUser.ID, &filter)
	if err != nil {
		t.Fatalf("GetAllLocatedForChapter() err: %v", err)
	}

	if len(BibleNotes) != 1 {
		t.Errorf("expected 1 Bible notes, got %d", len(BibleNotes))
	}

	if len(crossRefNotes) != 0 {
		t.Errorf("expected 0 cross-ref notes, got %d", len(crossRefNotes))
	}
}

func insertTestLocatedNote(content *NoteContent, location *NoteLocation, bookID int) (*int64, *int64, error) {
	noteID, err := insertTestNoteContent(content)
	if err != nil {
		return nil, nil, err
	}

	location.NoteID = *noteID

	locationID, err := insertTestNoteLocation(bookID, location)
	if err != nil {
		return nil, nil, err
	}

	return noteID, locationID, err
}

func deleteTestLocatedNote(noteId, locationID int64) {
	deleteTestNoteContent(noteId)
	deleteTestNoteLocation(locationID)
}

func insertTestNoteLocation(bookID int, location *NoteLocation) (*int64, error) {

	query := `
		INSERT INTO note_locations
			(note_id, book_id, chapter,start_verse, end_verse, start_offset, end_offset)
		VALUES
			($1, $2, $3, $4, $5, $6, $7)
		RETURNING 
			id
	`

	args := []any{location.NoteID, bookID, location.Chapter, location.StartVerse, location.EndVerse, location.StartOffset, location.EndOffset}

	var id int64
	err := testDB.QueryRow(query, args...).Scan(&id)
	if err != nil {
		return nil, err
	}

	return &id, nil

}

func insertTestNoteContent(content *NoteContent) (*int64, error) {
	query := `
		INSERT INTO notes
			(user_id, title, content, content_hash, note_type)	
		VALUES
			($1, $2, $3, $4, $5)
		RETURNING
			id 
	`
	contentHash := hashContent(content.Content)

	args := []any{content.UserID, content.Title, content.Content, contentHash, content.NoteType}

	var id int64

	err := testDB.QueryRow(query, args...).Scan(&id)
	if err != nil {
		return nil, err
	}

	return &id, nil

}

func deleteTestNoteContent(id int64) {
	query := `
		DELETE FROM notes
		WHERE id = $1	
	`
	testDB.Exec(query, id)
}

func deleteTestNoteLocation(id int64) {
	query := `
		DELETE FROM note_locations
		WHERE id = $1	
	`
	testDB.Exec(query, id)
}

func insertTestUser(user *User) error {
	query := `
		INSERT INTO users
			(id, name, email, password_hash, activated)	
		VALUES
			($1, $2, $3, $4, $5)
	`

	args := []any{user.ID, user.Name, user.Email, user.Password.hash, user.Activated}

	_, err := testDB.Exec(query, args...)
	if err != nil {
		return err
	}

	return nil
}

func deleteTestUser(id int64) error {
	query := `
		DELETE FROM users 
		WHERE id = $1	
	`

	_, err := testDB.Exec(query, id)
	if err != nil {
		return err
	}

	return nil
}
