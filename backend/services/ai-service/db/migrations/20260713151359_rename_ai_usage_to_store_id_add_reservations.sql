-- Modify "ai_usage" table
-- account_id → store_id 是純改名（② re-key：用量計數改綁目前 store，見 transform_usecase.go）。
-- 手動改成 RENAME COLUMN，而非 atlas migrate diff 預設產出的 DROP+ADD ——後者對已有資料
-- 的表會直接失敗（新 store_id 是 NOT NULL 又沒有 default，既有 row 過不了）。RENAME COLUMN
-- 會自動帶著既有的 ai_usage_pkey 走，不需另外重建。
ALTER TABLE "ai_usage" RENAME COLUMN "account_id" TO "store_id";
-- Create "ai_usage_reservations" table
CREATE TABLE "ai_usage_reservations" (
  "id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "week_start" date NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "idx_ai_usage_reservations_store_week" to table: "ai_usage_reservations"
CREATE INDEX "idx_ai_usage_reservations_store_week" ON "ai_usage_reservations" ("store_id", "week_start", "expires_at");
