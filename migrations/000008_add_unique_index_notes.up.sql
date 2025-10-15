DROP INDEX IF EXISTS idx_unique_general_note;
DROP INDEX IF EXISTS idx_notes_content_hash;
-- GENERAL notes: unique title per user;
CREATE UNIQUE INDEX idx_general_notes_user_title
ON notes(user_id, LOWER(title))
WHERE note_type = 'GENERAL';



-- All notes: unique content_hash per user per type
CREATE UNIQUE INDEX idx_bible_notes_user_content_hash 
ON notes(user_id, content_hash)
WHERE note_type IN ('BIBLE', 'CROSS_REFERENCE');
