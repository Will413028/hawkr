-- Modify "procurement_candidate_sets" table
ALTER TABLE "procurement_candidate_sets" ADD COLUMN "failure_reason" text NOT NULL DEFAULT '';
