-- Modify "link_tokens" table
ALTER TABLE "identity"."link_tokens" ADD CONSTRAINT "link_tokens_apple_code_check" CHECK ((apple_client_id IS NULL) = (apple_code IS NULL)), ADD COLUMN "apple_client_id" text NULL, ADD COLUMN "apple_code" bytea NULL;
