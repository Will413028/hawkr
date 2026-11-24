-- Create index "rfq_publication_invites_queued" to table: "rfq_publication_invites"
CREATE INDEX "rfq_publication_invites_queued" ON "procurement"."rfq_publication_invites" ("created_at", "id") WHERE (delivery_state = 'QUEUED'::text);
