CREATE TABLE IF NOT EXISTS notes(
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE, 
    title text,
    content text NOT NULL,
    note_type varchar(40) NOT NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT now(), 
    updated_at timestamp(0) with time zone NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS note_locations(
    id bigserial PRIMARY KEY, 
    note_id bigint NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    book_id integer NOT NULL REFERENCES books(id),
    chapter integer NOT NULL,
    start_verse integer NOT NULL,
    end_verse integer NOT NULL,
    start_offset integer, 
    end_offset integer
);

CREATE INDEX IF NOT EXISTS notes_user_notetype_idx ON notes
(user_id, note_type);

-- Index to quickly find all locations for a specific note
CREATE INDEX IF NOT EXISTS note_locations_note_id_idx ON note_locations
(note_id);

-- Index to quickly find all note locations within a specific chapter
CREATE INDEX IF NOT EXISTS note_locations_location_idx ON note_locations
(book_id, chapter);