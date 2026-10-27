-- Modify "notifications" table
ALTER TABLE "notifications" ALTER COLUMN "event_type" TYPE character varying(100);
-- Modify "push_notifications" table
ALTER TABLE "push_notifications" ALTER COLUMN "event_type" TYPE character varying(100);
