-- Modify "conversations" table
ALTER TABLE "conversations" ADD COLUMN "order_no" character varying(50) NULL, ADD COLUMN "order_binding_verified_at" timestamptz NULL;
