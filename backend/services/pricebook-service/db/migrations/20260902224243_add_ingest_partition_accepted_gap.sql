-- Modify "ingest_partition" table
ALTER TABLE "ingest_partition" ADD CONSTRAINT "ingest_partition_accepted_evidence" CHECK ((status <> 5) OR ((accepted_at IS NOT NULL) AND (accepted_by IS NOT NULL) AND (accepted_reason IS NOT NULL))), ADD COLUMN "accepted_at" timestamptz NULL, ADD COLUMN "accepted_by" text NULL, ADD COLUMN "accepted_reason" text NULL;
