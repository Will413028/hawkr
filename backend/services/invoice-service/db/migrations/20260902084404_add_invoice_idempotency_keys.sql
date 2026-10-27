-- Create "invoice_idempotency_keys" table
CREATE TABLE "invoice_idempotency_keys" (
  "scope" character varying(64) NOT NULL,
  "actor_id" bigint NOT NULL,
  "idem_key" character varying(128) NOT NULL,
  "request_hash" character(64) NOT NULL,
  "result_ref" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT "pk_invidem" PRIMARY KEY ("scope", "actor_id", "idem_key"),
  CONSTRAINT "chk_invidem_hash" CHECK (request_hash ~ '^[0-9a-f]{64}$'::text),
  CONSTRAINT "chk_invidem_key_nonblank" CHECK (length((idem_key)::text) >= 1)
);
-- Create index "idx_invidem_created" to table: "invoice_idempotency_keys"
CREATE INDEX "idx_invidem_created" ON "invoice_idempotency_keys" ("created_at");
-- Set comment to table: "invoice_idempotency_keys"
COMMENT ON TABLE "invoice_idempotency_keys" IS '冪等鍵（#3015）。與業務寫入同一個 transaction ⇒ 同一把鍵絕不產生第二筆。保留期由維運 SQL 依 created_at 清理，非永久帳。';
