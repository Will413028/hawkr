-- Modify "invoice_documents" table
ALTER TABLE "invoice_documents" ADD CONSTRAINT "chk_invdoc_resend_count_nonneg" CHECK (notification_resend_count >= 0), ADD CONSTRAINT "chk_invdoc_resend_pair" CHECK ((notification_resent_at IS NULL) = (notification_resend_count = 0)), ADD COLUMN "notification_resent_at" timestamptz NULL, ADD COLUMN "notification_resend_count" integer NOT NULL DEFAULT 0;
