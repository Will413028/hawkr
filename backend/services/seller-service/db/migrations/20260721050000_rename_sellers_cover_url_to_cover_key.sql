-- Rename "sellers"."cover_url" to "cover_key" (#1660: §25 誠實命名——欄存 objectKey 應叫 *_key)
ALTER TABLE "sellers" RENAME COLUMN "cover_url" TO "cover_key";
