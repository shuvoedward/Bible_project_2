ALTER TABLE notes ADD COLUMN content_hash CHAR(64);
CREATE INDEX idx_notes_content_hash ON notes(content_hash) WHERE note_type IN ('BIBLE', 'CROSS_REFERENCE');

ALTER TABLE notes ADD CONSTRAINT content_max_length
CHECK (LENGTH(content)<=50000);

