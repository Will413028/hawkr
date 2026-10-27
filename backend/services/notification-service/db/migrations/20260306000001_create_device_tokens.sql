-- Create device_tokens table
CREATE TABLE "device_tokens" (
  "id" bigserial NOT NULL,
  "user_id" bigint NOT NULL,
  "user_type" varchar(10) NOT NULL,
  "token" text NOT NULL,
  "platform" varchar(10) NOT NULL,
  "is_active" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_device_tokens_user_token" UNIQUE ("user_id", "user_type", "token")
);

CREATE INDEX "idx_device_tokens_user" ON "device_tokens" ("user_id", "user_type");
