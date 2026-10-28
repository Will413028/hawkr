-- Modify "procurement_confirmed_rfq_snapshots" table
ALTER TABLE "procurement_confirmed_rfq_snapshots" DROP COLUMN "quality_knowledge_state";
-- Modify "procurement_price_previews" table
ALTER TABLE "procurement_price_previews" DROP COLUMN "quality_knowledge_state";
-- Modify "procurement_rfq_publication_approvals" table
ALTER TABLE "procurement_rfq_publication_approvals" DROP COLUMN "quality_knowledge_state";
