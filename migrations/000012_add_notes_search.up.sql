-- Migration: Add full-text search capability to notes table.
-- Purpose: Enable fast, ranked searching across note titles and content.
-- Performance: btree_gin index allows efficient filtering by user_id + FTS in single index scan. 

-- Add tsvector column for full-text search
-- Automatically combines title and content into searchable lexemes
-- Handles NULL titles (cross-ref notes have no title)
-- Uses 'english' dictionary for stemming (e.g., "loving" matches "love")
ALTER TABLE notes 
ADD COLUMN note_vector tsvector;

CREATE OR REPLACE FUNCTION notes_vector_update() RETURNS trigger AS $$
BEGIN 
    NEW.note_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create a trigger to automatically update note_vector on INSERT/UPDATE
CREATE TRIGGER notes_vector_trigger
BEFORE INSERT OR UPDATE ON notes
FOR EACH ROW
EXECUTE FUNCTION notes_vector_update();

-- Populate existing rows
UPDATE notes 
SET note_vector = to_tsvector('english', COALESCE(title, '') || ' ' || COALESCE(content, ''));


CREATE EXTENSION IF NOT EXISTS btree_gin;

-- Create composite index for user-scoped full-text search
-- Optimized for query pattern: WHERE user_id = ? AND note_vector @@ websearch_to_tsquery(?)
-- Index covers both exact user filtering (B-tree) and text search (GIN)
CREATE INDEX idx_user_notes_search 
ON notes USING GIN(user_id, note_vector);