-- Create index "idx_procurement_confirmed_rfq_snapshots_request" to table: "procurement_confirmed_rfq_snapshots"
CREATE INDEX "idx_procurement_confirmed_rfq_snapshots_request" ON "procurement_confirmed_rfq_snapshots" ("request_id", "buyer_id", "created_at" DESC);
