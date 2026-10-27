-- Rename column "exposable_since" to "published_at" on table "seller_scores"（同一個時間，改成 seller 的公開時間；快照每 15 分鐘整批重建，這裡只改名）
ALTER TABLE "discovery"."seller_scores" RENAME COLUMN "exposable_since" TO "published_at";
ALTER TABLE "discovery"."seller_scores" RENAME CONSTRAINT "seller_scores_exposable_since_not_null" TO "seller_scores_published_at_not_null";
