DROP INDEX IF EXISTS idx_notes_content_hash;
ALTER TABLE notes DROP CONSTRAINT IF EXISTS content_max_length;
ALTER TABLE notes DROP COLUMN IF EXISTS content_hash;