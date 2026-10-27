CREATE TABLE "notification_preferences" (
  "id" bigserial NOT NULL,
  "user_id" bigint NOT NULL,
  "user_type" varchar(10) NOT NULL, -- 'buyer' | 'merchant'
  "category" varchar(30) NOT NULL,  -- 'order_update' | 'refund_update' | 'system' | 'chat' | 'follower' | 'post'
  "enabled" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_notification_preferences_user_category" UNIQUE ("user_id", "user_type", "category")
);
CREATE INDEX "idx_notification_preferences_user" ON "notification_preferences" ("user_id", "user_type");
