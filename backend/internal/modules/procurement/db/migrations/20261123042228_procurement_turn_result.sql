-- Modify "turns" table
ALTER TABLE "procurement"."turns" ADD CONSTRAINT "turns_turn_result_check" CHECK (jsonb_typeof(turn_result) = 'object'::text), ADD COLUMN "turn_result" jsonb NOT NULL;
