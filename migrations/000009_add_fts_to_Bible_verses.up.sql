ALTER TABLE verses
ADD COLUMN search_vector TSVECTOR
GENERATED ALWAYS AS (to_tsvector('simple', text)) STORED;


CREATE INDEX idx_bible_verses_search
ON verses USING GIN(search_vector);