DROP INDEX IF EXISTS idx_unique_general_note;

ALTER TABLE notes
ALTER COLUMN title TYPE TEXT;