ALTER TABLE "identity"."admins" RENAME COLUMN "clerk_user_id" TO "access_subject";
ALTER TABLE "identity"."admins" RENAME CONSTRAINT "admins_clerk_user_id_not_null" TO "admins_access_subject_not_null";
ALTER INDEX "identity"."uidx_admins_clerk_user_id" RENAME TO "uidx_admins_access_subject";
ALTER TABLE "identity"."admins" DROP COLUMN "name", DROP COLUMN "picture_url";
