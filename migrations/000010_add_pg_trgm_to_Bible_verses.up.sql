CREATE EXTENSION pg_trgm;

CREATE INDEX idx_verses_trgm ON verses 
USING GIN (text gin_trgm_ops); 