-- Modify "cases" table
ALTER TABLE "moderation"."cases" ADD CONSTRAINT "cases_measure_check" CHECK ((measure_id IS NULL) OR (decision_action = 'TAKEDOWN'::text)), ADD COLUMN "measure_id" uuid NULL;
