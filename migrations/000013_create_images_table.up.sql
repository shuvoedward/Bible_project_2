CREATE TABLE IF NOT EXISTS images (
    id bigserial PRIMARY KEY,
    note_id bigint NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    s3_key varchar(512) NOT NULL,
    width integer ,
    height integer,
    original_filename varchar(255) ,
    mime_type varchar(50) NOT NULL,
    file_size integer, 
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_images_note_id ON images(note_id);

