ALTER TABLE notes
DROP COLUMN IF EXISTS note_vector;

DROP INDEX IF EXISTS idx_user_notes_search;
