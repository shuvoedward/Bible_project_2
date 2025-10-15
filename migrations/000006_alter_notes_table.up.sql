ALTER TABLE notes  ALTER COLUMN
title TYPE VARCHAR(255);

CREATE UNIQUE INDEX
idx_unique_general_note ON notes
(user_id, title) WHERE note_type = 'GENERAL';