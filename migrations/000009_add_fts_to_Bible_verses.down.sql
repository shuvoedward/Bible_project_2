ALTER TABLE verses
DROP COLUMN search_vector;

DROP INDEX IF EXISTS idx_bible_verses_search;