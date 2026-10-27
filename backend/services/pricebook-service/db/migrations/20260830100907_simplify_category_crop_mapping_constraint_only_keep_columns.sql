-- Modify "category_crop_mapping" table
ALTER TABLE "category_crop_mapping" DROP CONSTRAINT "category_crop_mapping_selection_kind_check", DROP CONSTRAINT "category_crop_mapping_resolved_oneof_check", ADD CONSTRAINT "category_crop_mapping_resolved_oneof_check" CHECK (((status = 'resolved'::text) AND (group_code IS NOT NULL)) OR ((status <> 'resolved'::text) AND (group_code IS NULL)));
