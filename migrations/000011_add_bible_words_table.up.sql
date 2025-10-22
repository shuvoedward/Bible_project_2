CREATE TABLE bible_words (
    word TEXT PRIMARY KEY,
    lexeme TEXT,
    frequency INT DEFAULT 1
);


CREATE INDEX idx_bible_words_lexeme_trgm ON bible_words
USING GIN (lexeme gin_trgm_ops);


-- Populate it
WITH word_list AS (
    SELECT lower(unnest(string_to_array(text, ' '))) as word
    FROM verses
)
INSERT INTO bible_words (word, lexeme, frequency)
SELECT word,
       regexp_replace(
           (to_tsvector('english', word))::text, 
           '''(\w+)''.*', 
           '\1'
       ) as lexeme,
       COUNT(*) as frequency
FROM word_list
WHERE length(word) > 2
  AND word ~ '^[a-z]+$'
GROUP BY word, lexeme
ON CONFLICT (word) DO NOTHING;