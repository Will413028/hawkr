-- Modify "events" table
ALTER TABLE "events" ADD COLUMN "input_transcription" jsonb NULL, ADD COLUMN "output_transcription" jsonb NULL;
