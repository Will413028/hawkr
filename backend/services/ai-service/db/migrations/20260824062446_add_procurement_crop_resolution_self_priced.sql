-- Modify "procurement_requests" table
ALTER TABLE "procurement_requests" ADD COLUMN "crop_resolution_self_priced" boolean NOT NULL DEFAULT false;
