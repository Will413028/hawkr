-- Modify "procurement_rfq_publication_outbox" table
ALTER TABLE "procurement_rfq_publication_outbox" DROP CONSTRAINT "chk_prpo_event_type", ADD CONSTRAINT "chk_prpo_event_type" CHECK (event_type = ANY (ARRAY['RFQ_PUBLICATION_INVITE_CREATED'::text, 'RFQ_PUBLICATION_INVITE_EXTERNAL_CREATED'::text]));
