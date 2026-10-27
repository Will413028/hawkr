-- Enable pg_trgm extension for trigram-based fuzzy search on recipes.title
-- (declared in schema/schema.sql:1; was missing from migration history)
CREATE EXTENSION IF NOT EXISTS pg_trgm;
