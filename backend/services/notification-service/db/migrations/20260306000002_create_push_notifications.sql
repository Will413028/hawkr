-- Create push_notifications table
CREATE TABLE "push_notifications" (
  "id" bigserial NOT NULL,
  "user_id" bigint NOT NULL,
  "user_type" varchar(10) NOT NULL,
  "event_type" varchar(50) NOT NULL,
  "reference_id" varchar(100),
  "title" text NOT NULL,
  "body" text NOT NULL,
  "data" jsonb,
  "status" varchar(20) NOT NULL DEFAULT 'pending',
  "sent_at" timestamptz,
  "error_log" text,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);

CREATE INDEX "idx_push_notifications_user" ON "push_notifications" ("user_id", "user_type");
CREATE INDEX "idx_push_notifications_dedup" ON "push_notifications" ("event_type", "reference_id");
