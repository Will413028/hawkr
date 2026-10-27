-- Modify "admins" table
ALTER TABLE "identity"."admins" ADD COLUMN "name" text NULL, ADD COLUMN "picture_url" text NULL;
