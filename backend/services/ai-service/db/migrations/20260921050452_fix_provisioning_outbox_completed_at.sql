-- Modify "procurement_order_provisioning_outbox" table
ALTER TABLE "procurement_order_provisioning_outbox" DROP CONSTRAINT "chk_propo_completed_at", ADD CONSTRAINT "chk_propo_completed_at" CHECK ((status = 'pending'::text) = (completed_at IS NULL));
