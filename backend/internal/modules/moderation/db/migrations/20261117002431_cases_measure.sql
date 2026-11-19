-- Modify "cases" table
ALTER TABLE "moderation"."cases" ADD CONSTRAINT "cases_measure_check" CHECK ((NOT (decision_action IS DISTINCT FROM 'TAKEDOWN'::text)) = (measure_id IS NOT NULL)), ADD COLUMN "measure_id" uuid NULL;
