CREATE TABLE IF NOT EXISTS highlights(
   id bigserial PRIMARY KEY ,
   user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
   book_id integer NOT NULL REFERENCES books(id),
   chapter integer NOT NULL,
   start_verse integer,
   end_verse integer,
   start_offset integer,
   end_offset integer,
   color VARCHAR(50),
   created_at timestamp(0) with time zone NOT NULL DEFAULT now(), 
   updated_at timestamp(0) with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS highlights_user_book_chapter_idx ON highlights
(user_id, book_id, chapter);