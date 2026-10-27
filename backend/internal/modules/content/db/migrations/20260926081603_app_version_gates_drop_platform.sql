-- Modify "app_version_gates" table
ALTER TABLE "content"."app_version_gates" DROP CONSTRAINT "app_version_gates_platform_check", DROP CONSTRAINT "app_version_gates_pkey", DROP COLUMN "platform", ADD PRIMARY KEY ("app_type");
