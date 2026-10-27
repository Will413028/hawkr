-- Create "category_crop_mapping" table
CREATE TABLE "category_crop_mapping" (
  "category_id" text NOT NULL,
  "status" text NOT NULL,
  "selection_kind" text NULL,
  "crop_code" text NULL,
  "group_code" text NULL,
  "mapping_version" text NOT NULL,
  "source_note" text NULL,
  "updated_by" bigint NOT NULL,
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("category_id"),
  CONSTRAINT "category_crop_mapping_resolved_oneof_check" CHECK (((status = 'resolved'::text) AND (selection_kind = 'crop'::text) AND (crop_code IS NOT NULL) AND (group_code IS NULL)) OR ((status = 'resolved'::text) AND (selection_kind = 'group'::text) AND (group_code IS NOT NULL) AND (crop_code IS NULL)) OR ((status <> 'resolved'::text) AND (selection_kind IS NULL) AND (crop_code IS NULL) AND (group_code IS NULL))),
  CONSTRAINT "category_crop_mapping_selection_kind_check" CHECK ((selection_kind IS NULL) OR (selection_kind = ANY (ARRAY['crop'::text, 'group'::text]))),
  CONSTRAINT "category_crop_mapping_status_check" CHECK (status = ANY (ARRAY['resolved'::text, 'unmapped'::text, 'ambiguous'::text]))
);
