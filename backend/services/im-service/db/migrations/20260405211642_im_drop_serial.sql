-- Modify "conversations" table
ALTER TABLE "conversations" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "conversations_id_seq";
-- Modify "messages" table
ALTER TABLE "messages" ALTER COLUMN "id" DROP DEFAULT;
-- Drop sequence used by serial column "id"
DROP SEQUENCE IF EXISTS "messages_id_seq";
