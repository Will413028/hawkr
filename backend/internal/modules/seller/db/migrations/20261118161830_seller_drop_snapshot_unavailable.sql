-- Modify "measure_contents" table
ALTER TABLE "seller"."measure_contents" DROP CONSTRAINT "measure_contents_snapshot_state_check", ADD CONSTRAINT "measure_contents_snapshot_state_check" CHECK (snapshot_state = ANY (ARRAY['CAPTURED'::text, 'CLEARED'::text]));
