-- Drop index "idx_procurement_conversations_buyer_active" from table: "procurement_conversations"
DROP INDEX "idx_procurement_conversations_buyer_active";
-- Create index "idx_procurement_conversations_buyer_active" to table: "procurement_conversations"
CREATE INDEX "idx_procurement_conversations_buyer_active" ON "procurement_conversations" ("buyer_id", "updated_at" DESC, "id" DESC) WHERE (deleted_at IS NULL);
