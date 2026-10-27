-- Modify "sellers" table
ALTER TABLE "sellers" ADD CONSTRAINT "chk_sellers_food_reg" CHECK (((is_processed_food = false) AND ((food_business_reg_no)::text = ''::text)) OR ((is_processed_food = true) AND ((food_business_reg_no)::text <> ''::text))), ADD COLUMN "is_processed_food" boolean NOT NULL DEFAULT false, ADD COLUMN "food_business_reg_no" character varying(50) NOT NULL DEFAULT '';

-- Backfill: carry each seller's existing KYC-time food declaration onto the seller
-- row, but only when the current approved application is internally consistent
-- (processed_food=true AND a non-empty reg no). Inconsistent/absent declarations
-- leave the seller at its safe default (false, ''), which the CHECK accepts.
UPDATE "sellers" s
SET "is_processed_food" = true,
    "food_business_reg_no" = ca."food_business_reg_no"
FROM "certification_applications" ca
WHERE s."current_certification_id" = ca."id"
  AND ca."status" = 'approved'
  AND ca."is_processed_food" = true
  AND ca."food_business_reg_no" <> '';
