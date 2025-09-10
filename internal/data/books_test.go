package data

import (
	"log"
	"testing"

	"github.com/lib/pq"
)

func TestGetSingleVerse(t *testing.T) {
	bookID, err := insertTestBook("Genesis")
	if err != nil {
		t.Fatalf("failed to insert test book: %v", err)
	}
	defer deleteTestBook(bookID)

	// Insert a verse
	text := "In the beginning, God created the heavens and the earth."
	verseID, err := insertVerse(bookID, 1, 1, text)
	if err != nil {
		t.Fatalf("failed to insert test verse: %v", err)
	}
	defer deleteTestVerse(verseID)

	m := NewModels(testDB)
	filters := PassageFilters{
		Book:    "Genesis",
		Chapter: 1,
		Verse:   1,
	}

	passage, err := m.Passages.Get(&filters)
	if err != nil {
		t.Fatalf("Get() returned an error: %v", err)
	}

	if passage.Book != "Genesis" {
		t.Errorf("expected the book to be 'genesis', but got %s", passage.Book)
	}

	if passage.Chapter != 1 {
		t.Errorf("expected the chapter to be 1, but got %d", passage.Chapter)
	}

	if len(passage.Verses) == 0 {
		t.Errorf("expected 1 verse, but got %d", len(passage.Verses))
	}

	if passage.Verses[0].Number != 1 {
		t.Errorf("expected verse number to be 1, but got %d", passage.Verses[0].Number)
	}

	if passage.Verses[0].Text != text {
		t.Errorf("unexpected verse text %s", passage.Verses[0].Text)
	}

}

func TestGetChapter(t *testing.T) {
	chapter := []string{
		"In the beginning God created the heavens and the earth.",
		"Now the earth was formless and void, and darkness was over the surface of the deep. And the Spirit of God was hovering over the surface of the waters.",
		"And God said, �Let there be light,� and there was light.",
		"And God saw that the light was good, and He separated the light from the darkness.",
		"God called the light �day,� and the darkness He called �night.� And there was evening, and there was morning�the first day.",
	}

	// 1.  Insert book
	bookID, err := insertTestBook("Genesis")
	if err != nil {
		log.Fatalf("failed to insert test book: %v", err)
	}
	defer deleteTestBook(bookID)

	// 2. Insert Chapter
	err = insertTestChapter(chapter, bookID)
	if err != nil {
		log.Fatalf("failed to insert test chapter: %v", err)
	}
	defer deleteTestChapter(bookID, 1)

	// 3. Get chapter
	filters := PassageFilters{
		Book:    "Genesis",
		Chapter: 1,
	}

	m := NewModels(testDB)
	passage, err := m.Passages.Get(&filters)
	if err != nil {
		t.Fatalf("Get() returned an err %v", err)
	}

	// 4. Test
	if passage.Book != "Genesis" {
		t.Errorf("expected the book to be 'Genesis', but got %s", passage.Book)
	}

	if passage.Chapter != 1 {
		t.Errorf("expected the chapter to be 1, but got %d", passage.Chapter)
	}

	if len(passage.Verses) != len(chapter) {
		t.Errorf("expected %d verses, but recieved %d verses", len(chapter), len(passage.Verses))
	}

	verseNumber := 1
	for i, verse := range passage.Verses {
		if verse.Number != verseNumber {
			t.Errorf("expected verse number to be %d, but got %d", verseNumber, verse.Number)
		}
		if verse.Text != chapter[i] {
			t.Errorf("unexpected verse %s", verse.Text)
		}
		verseNumber++
	}

}

func TestGetVerseRange(t *testing.T) {
	bookID, err := insertTestBook("Genesis")
	if err != nil {
		log.Fatalf("failed to insert test book: %v", err)
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
		t.Fatalf("failed to insert test chapter %v", err)
	}
	defer deleteTestChapter(bookID, 1)

	filter := PassageFilters{
		Book:       "Genesis",
		Chapter:    1,
		StartVerse: 1,
		EndVerse:   3,
	}

	m := NewModels(testDB)
	passage, err := m.Passages.Get(&filter)
	if err != nil {
		t.Fatalf("Get() returned an error %v", err)
	}

	if passage.Book != "Genesis" {
		t.Errorf("expected the book to be 'Genesis', but got %s", passage.Book)
	}

	if passage.Chapter != 1 {
		t.Errorf("expected the chapter to be 1, but got %d", passage.Chapter)
	}

	if len(passage.Verses) != 3 {
		t.Errorf("expected %d verses, but recieved %d verses", len(chapter), len(passage.Verses))
	}

	for i := 0; i < 3; i++ {
		if passage.Verses[i].Number != i+1 {
			t.Errorf("expected verse number to be %d, but got %d", i+1, passage.Verses[i].Number)
		}
		if chapter[i] != passage.Verses[i].Text {
			t.Errorf("unexpected verse %s", passage.Verses[i].Text)
		}
	}

}

func insertTestChapter(chapter []string, bookID int) error {
	tx, err := testDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("verses", "book_id", "chapter", "verse", "text"))
	if err != nil {
		return err
	}

	for i, v := range chapter {
		log.Printf("Inserting verse %d of %d : %s", i+1, 5, v)
		_, err = stmt.Exec(bookID, 1, i+1, v)
		if err != nil {
			return err
		}
	}

	// _, err = stmt.Exec()
	// if err != nil {
	// 	return err
	// }

	log.Println("Finished loop. Closing statement.")
	err = stmt.Close()
	if err != nil {
		return err
	}

	return tx.Commit()

}

func deleteTestChapter(bookID, chapter int) {
	query := `DELETE FROM verses where book_id = $1 and chapter = $2`
	testDB.Exec(query, bookID, chapter)
}

func insertTestBook(book string) (int, error) {
	var id int
	query := `INSERT INTO books (name) 
		VALUES ($1) 
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name  RETURNING id`
	err := testDB.QueryRow(query, book).Scan(&id)
	return id, err
}

func deleteTestBook(id int) {
	query := `DELETE FROM books where id = $1`
	testDB.Exec(query, id)
}

func insertVerse(bookID, chapter, verse int, text string) (int, error) {
	var id int
	query := `INSERT INTO verses (book_id, chapter, verse, text)
	VALUES ($1, $2, $3, $4) RETURNING id`
	err := testDB.QueryRow(query, bookID, chapter, verse, text).Scan(&id)
	return id, err
}

func deleteTestVerse(id int) {
	query := `DELETE FROM verses where id = $1`
	testDB.Exec(query, id)
}
