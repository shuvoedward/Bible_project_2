CREATE UNIQUE INDEX idx_note_locations_user_id_book_verse 
ON note_locations(user_id, note_id, book_id, chapter, start_verse, end_verse) 