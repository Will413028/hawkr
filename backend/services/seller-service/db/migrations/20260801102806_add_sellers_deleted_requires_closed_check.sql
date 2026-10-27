-- Modify "sellers" table
ALTER TABLE "sellers" ADD CONSTRAINT "chk_sellers_deleted_requires_closed" CHECK (((status)::text = 'closed'::text) OR (deleted_at IS NULL));
